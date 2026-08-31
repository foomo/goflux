package nats_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/foomo/goencode/json/v1"
	"github.com/foomo/goflux"
	fluxnats "github.com/foomo/goflux/transport/nats"
	"github.com/nats-io/nats.go"
)

// subscribeReadyRounds is the number of subscribe/publish cycles each race test
// performs. The window between "SUB frame buffered client-side" and "server has
// registered the subscription" is a single network round trip, so a lost
// message is not reliably reproducible in one attempt — it depends on goroutine
// scheduling. Repeating tightens the odds that an unflushed subscribe is caught.
const subscribeReadyRounds = 200

// newConn opens an additional client connection to an already-running server.
//
// The publish must not share a connection with the subscriber: writes on one
// nats.Conn are serialised, so publishing through the subscriber's connection
// would order the PUB behind the SUB for free and mask exactly the bug under
// test.
func newConn(t *testing.T, url string) *nats.Conn {
	t.Helper()

	conn, err := nats.Connect(url, nats.Timeout(10*time.Second))
	if err != nil {
		t.Fatalf("nats connect: %s", err)
	}

	t.Cleanup(conn.Close)

	return conn
}

// TestSubscribeWithReady_publishAfterReadyIsNotLost asserts the core guarantee:
// once ready has fired, a message published from an unrelated connection is
// always delivered.
//
// Before the FlushWithContext in SubscribeWithReady, ready fired as soon as
// conn.Subscribe returned — while the SUB frame was still sitting in the client
// write buffer. The publish then reached the server first and core NATS dropped
// it silently, with no error on either side. This test fails in that state.
func TestSubscribeWithReady_publishAfterReadyIsNotLost(t *testing.T) {
	srv, subConn := newServer()

	defer srv.Shutdown()
	defer subConn.Close()

	pubConn := newConn(t, srv.ClientURL())
	sub := fluxnats.NewSubscriber(subConn, json.NewCodec[Event]().Decode)

	for round := range subscribeReadyRounds {
		subject := "readiness.plain." + itoa(round)

		received := make(chan goflux.Message[Event], 1)
		ctx, cancel := context.WithCancel(t.Context())

		ready := make(chan struct{})

		go func() {
			_ = sub.SubscribeWithReady(ctx, subject, func(_ context.Context, msg goflux.Message[Event]) error {
				select {
				case received <- msg:
				default:
				}

				return nil
			}, func() { close(ready) })
		}()

		select {
		case <-ready:
		case <-time.After(10 * time.Second):
			cancel()
			t.Fatalf("round %d: subscription never became ready", round)
		}

		// The contract under test: after ready, this publish cannot be lost.
		if err := pubConn.Publish(subject, []byte(`{"id":"1","name":"foo"}`)); err != nil {
			cancel()
			t.Fatalf("round %d: publish: %s", round, err)
		}

		select {
		case msg := <-received:
			if msg.Payload.ID != "1" || msg.Payload.Name != "foo" {
				cancel()
				t.Fatalf("round %d: unexpected payload %+v", round, msg.Payload)
			}
		case <-time.After(5 * time.Second):
			cancel()
			t.Fatalf("round %d: message published after ready was lost", round)
		}

		cancel()
	}
}

// TestSubscribeWithReady_queueGroup covers the QueueSubscribe branch, which is a
// separate code path in SubscribeWithReady and buffers its SUB frame the same way.
func TestSubscribeWithReady_queueGroup(t *testing.T) {
	srv, subConn := newServer()

	defer srv.Shutdown()
	defer subConn.Close()

	pubConn := newConn(t, srv.ClientURL())
	sub := fluxnats.NewSubscriber(
		subConn,
		json.NewCodec[Event]().Decode,
		fluxnats.WithQueueGroup("workers"),
	)

	for round := range subscribeReadyRounds {
		subject := "readiness.queue." + itoa(round)

		received := make(chan struct{}, 1)
		ctx, cancel := context.WithCancel(t.Context())

		ready := make(chan struct{})

		go func() {
			_ = sub.SubscribeWithReady(ctx, subject, func(_ context.Context, _ goflux.Message[Event]) error {
				select {
				case received <- struct{}{}:
				default:
				}

				return nil
			}, func() { close(ready) })
		}()

		select {
		case <-ready:
		case <-time.After(10 * time.Second):
			cancel()
			t.Fatalf("round %d: queue subscription never became ready", round)
		}

		if err := pubConn.Publish(subject, []byte(`{"id":"1","name":"foo"}`)); err != nil {
			cancel()
			t.Fatalf("round %d: publish: %s", round, err)
		}

		select {
		case <-received:
		case <-time.After(5 * time.Second):
			cancel()
			t.Fatalf("round %d: queue-group message published after ready was lost", round)
		}

		cancel()
	}
}

// TestServeWithReady_requestAfterReadyIsAnswered is the responder-side analogue,
// and the shape that matters for request/reply callers: subscribe for requests,
// then immediately issue one. Without the flush the responder's SUB can lose the
// race with the request, which surfaces to the caller as a request timeout
// rather than an error naming the real cause.
func TestServeWithReady_requestAfterReadyIsAnswered(t *testing.T) {
	srv, respConn := newServer()

	defer srv.Shutdown()
	defer respConn.Close()

	reqConn := newConn(t, srv.ClientURL())

	codec := json.NewCodec[Event]()
	responder := fluxnats.NewResponder(respConn, codec, codec)
	requester := fluxnats.NewRequester[Event, Event](reqConn, codec, codec)

	for round := range subscribeReadyRounds {
		subject := "readiness.serve." + itoa(round)

		ctx, cancel := context.WithCancel(t.Context())
		ready := make(chan struct{})

		go func() {
			_ = responder.ServeWithReady(ctx, subject, func(_ context.Context, req Event) (Event, error) {
				return Event{ID: req.ID, Name: "pong"}, nil
			}, func() { close(ready) })
		}()

		select {
		case <-ready:
		case <-time.After(10 * time.Second):
			cancel()
			t.Fatalf("round %d: responder never became ready", round)
		}

		reqCtx, reqCancel := context.WithTimeout(ctx, 5*time.Second)

		resp, err := requester.Request(reqCtx, subject, Event{ID: "1", Name: "ping"})

		reqCancel()

		if err != nil {
			cancel()
			t.Fatalf("round %d: request issued after ready failed: %s", round, err)
		}

		if resp.Name != "pong" {
			cancel()
			t.Fatalf("round %d: unexpected response %+v", round, resp)
		}

		cancel()
	}
}

// TestSubscribe_readyIsNotCalledAfterReturn guards the documented contract that
// ready never fires once SubscribeWithReady has returned, so callers can rely on
// "ready fired" implying a live subscription.
func TestSubscribe_readyIsNotCalledAfterReturn(t *testing.T) {
	srv, conn := newServer()

	defer srv.Shutdown()
	defer conn.Close()

	sub := fluxnats.NewSubscriber(conn, json.NewCodec[Event]().Decode)

	var (
		readyCalls atomic.Int64
		returned   atomic.Bool
	)

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	go func() {
		defer close(done)

		_ = sub.SubscribeWithReady(ctx, "readiness.contract", func(_ context.Context, _ goflux.Message[Event]) error {
			return nil
		}, func() {
			if returned.Load() {
				t.Error("ready called after SubscribeWithReady returned")
			}

			readyCalls.Add(1)
		})

		returned.Store(true)
	}()

	// Give the subscription time to establish, then tear it down.
	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("SubscribeWithReady did not return after ctx cancel")
	}

	if got := readyCalls.Load(); got != 1 {
		t.Fatalf("ready called %d times, want exactly 1", got)
	}
}

// itoa avoids pulling strconv into the test's import set for a trivial use.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	var buf [8]byte

	pos := len(buf)

	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}

	return string(buf[pos:])
}
