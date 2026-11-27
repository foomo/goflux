package jetstream_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/foomo/goencode/json/v1"
	"github.com/foomo/goflux"
	fluxjetstream "github.com/foomo/goflux/transport/jetstream"
	"github.com/foomo/gofuncy"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// startSubscriber creates a stream and durable consumer, runs
// SubscribeWithReady with handler until it is ready, and returns the
// connection, JetStream handle and the channel that receives Subscribe's
// result.
func startSubscriber(t *testing.T, ctx context.Context, handler goflux.Handler[Event], opts ...fluxjetstream.Option) (*nats.Conn, jetstream.JetStream, <-chan error) {
	t.Helper()

	srv := startServerOnPort(t, -1)
	conn, _, _ := connectWithReconnect(t, srv.ClientURL())

	js, err := fluxjetstream.NewStream(ctx, conn, jetstream.StreamConfig{Name: "CONSUME", Subjects: []string{"consume.>"}})
	if err != nil {
		t.Fatal(err)
	}

	cons, err := js.CreateConsumer(ctx, "CONSUME", jetstream.ConsumerConfig{
		Durable:   "consume",
		AckPolicy: jetstream.AckExplicitPolicy,
	})
	if err != nil {
		t.Fatal(err)
	}

	sub := fluxjetstream.NewSubscriber(cons, json.NewCodec[Event]().Decode, opts...)
	done := make(chan error, 1)

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		err := sub.SubscribeWithReady(ctx, "consume.run", handler, ready)
		done <- err

		return err
	}, gofuncy.WithName("test.subscriber"))

	return conn, js, done
}

func nopHandler(context.Context, goflux.Message[Event]) error { return nil }

func expectResult(t *testing.T, done <-chan error, timeout time.Duration) error {
	t.Helper()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		t.Fatalf("Subscribe still blocking after %s", timeout)

		return nil
	}
}

// expectBlocking fails if Subscribe returns within d.
func expectBlocking(t *testing.T, done <-chan error, d time.Duration) {
	t.Helper()

	select {
	case err := <-done:
		t.Fatalf("Subscribe returned early: %v", err)
	case <-time.After(d):
	}
}

// TestSubscriber_connectionClosedBlocks asserts that Subscribe keeps blocking
// until ctx is cancelled when the caller closes the connection it owns.
func TestSubscriber_connectionClosedBlocks(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	conn, _, done := startSubscriber(t, ctx, nopHandler)
	conn.Close()

	expectBlocking(t, done, 500*time.Millisecond)
	cancel()

	if err := expectResult(t, done, 5*time.Second); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// waitPendingPull polls until cons has a pending pull request, so deleting it
// makes nats.go report jetstream.ErrConsumerDeleted instead of retrying.
func waitPendingPull(t *testing.T, cons jetstream.Consumer) {
	t.Helper()

	for deadline := time.Now().Add(5 * time.Second); ; {
		info, err := cons.Info(t.Context())
		if err != nil {
			t.Fatal(err)
		}

		if info.NumWaiting > 0 {
			return
		}

		if time.Now().After(deadline) {
			t.Fatal("no pull request pending after 5s")
		}

		time.Sleep(10 * time.Millisecond)
	}
}

// TestSubscriber_consumerDeletedRecovers asserts that the original Subscribe
// call receives messages again once a deleted consumer is recreated (for
// example by NACK), also when the deletion hits a pending pull and nats.go
// stops consuming. The test waits for a pending pull before deleting. A
// handler set with WithConsumeErrHandler sees the deletion without disabling
// the restart.
func TestSubscriber_consumerDeletedRecovers(t *testing.T) {
	t.Parallel()

	received := make(chan Event, 8)
	deleted := make(chan struct{}, 1)
	consumerCfg := jetstream.ConsumerConfig{Durable: "consume", AckPolicy: jetstream.AckExplicitPolicy}

	_, js, done := startSubscriber(t, t.Context(), func(_ context.Context, msg goflux.Message[Event]) error {
		received <- msg.Payload

		return nil
	},
		fluxjetstream.WithConsumeOpts(jetstream.PullExpiry(time.Second)),
		fluxjetstream.WithConsumeErrHandler(func(_ jetstream.ConsumeContext, err error) {
			if errors.Is(err, jetstream.ErrConsumerDeleted) {
				select {
				case deleted <- struct{}{}:
				default:
				}
			}
		}),
	)

	cons, err := js.Consumer(t.Context(), "CONSUME", "consume")
	if err != nil {
		t.Fatal(err)
	}

	waitPendingPull(t, cons)

	if err := js.DeleteConsumer(t.Context(), "CONSUME", "consume"); err != nil {
		t.Fatal(err)
	}

	waitFor(t, deleted, 5*time.Second, "ErrConsumerDeleted in the custom error handler")
	expectBlocking(t, done, 500*time.Millisecond)

	if _, err := js.CreateConsumer(t.Context(), "CONSUME", consumerCfg); err != nil {
		t.Fatal(err)
	}

	publishStored(t, t.Context(), js, "consume.run", "after")
	expectEvent(t, received, "after", 10*time.Second)
}

// noCachedInfo wraps a consumer whose CachedInfo returns nil, like custom
// consumer wrappers, so the subscriber has to ask the server for it.
type noCachedInfo struct{ jetstream.Consumer }

func (noCachedInfo) CachedInfo() *jetstream.ConsumerInfo { return nil }

// TestSubscriber_ephemeralConsumerDeletedReturns asserts that Subscribe returns
// an error when an ephemeral consumer is deleted, because it cannot be
// recreated under the same name, also when the consumer caches no info.
func TestSubscriber_ephemeralConsumerDeletedReturns(t *testing.T) {
	t.Parallel()

	for name, wrap := range map[string]func(jetstream.Consumer) jetstream.Consumer{
		"cached info":    func(c jetstream.Consumer) jetstream.Consumer { return c },
		"no cached info": func(c jetstream.Consumer) jetstream.Consumer { return noCachedInfo{c} },
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			testEphemeralConsumerDeletedReturns(t, wrap)
		})
	}
}

func testEphemeralConsumerDeletedReturns(t *testing.T, wrap func(jetstream.Consumer) jetstream.Consumer) {
	t.Helper()

	srv := startServerOnPort(t, -1)
	conn, _, _ := connectWithReconnect(t, srv.ClientURL())

	js, err := fluxjetstream.NewStream(t.Context(), conn, jetstream.StreamConfig{Name: "EPHEMERAL", Subjects: []string{"ephemeral.>"}})
	if err != nil {
		t.Fatal(err)
	}

	cons, err := js.CreateConsumer(t.Context(), "EPHEMERAL", jetstream.ConsumerConfig{
		Name:      "ephemeral",
		AckPolicy: jetstream.AckExplicitPolicy,
	})
	if err != nil {
		t.Fatal(err)
	}

	sub := fluxjetstream.NewSubscriber(wrap(cons), json.NewCodec[Event]().Decode,
		fluxjetstream.WithConsumeOpts(jetstream.PullExpiry(time.Second)))
	done := make(chan error, 1)

	gofuncy.StartWithReady(t.Context(), func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		err := sub.SubscribeWithReady(ctx, "ephemeral.run", nopHandler, ready)
		done <- err

		return err
	}, gofuncy.WithName("test.subscriber"))

	waitPendingPull(t, cons)

	if err := js.DeleteConsumer(t.Context(), "EPHEMERAL", "ephemeral"); err != nil {
		t.Fatal(err)
	}

	err = expectResult(t, done, 5*time.Second)
	if !errors.Is(err, goflux.ErrSubscribe) || !errors.Is(err, jetstream.ErrConsumerDeleted) {
		t.Fatalf("expected ErrSubscribe wrapping ErrConsumerDeleted, got %v", err)
	}
}

// TestSubscriber_consumerInfoFails asserts that Subscribe returns an error
// before ready when the consumer caches no info and the server cannot
// provide it.
func TestSubscriber_consumerInfoFails(t *testing.T) {
	t.Parallel()

	srv := startServerOnPort(t, -1)
	conn, _, _ := connectWithReconnect(t, srv.ClientURL())

	js, err := fluxjetstream.NewStream(t.Context(), conn, jetstream.StreamConfig{Name: "INFO", Subjects: []string{"info.>"}})
	if err != nil {
		t.Fatal(err)
	}

	cons, err := js.CreateConsumer(t.Context(), "INFO", jetstream.ConsumerConfig{Name: "gone"})
	if err != nil {
		t.Fatal(err)
	}

	if err := js.DeleteConsumer(t.Context(), "INFO", "gone"); err != nil {
		t.Fatal(err)
	}

	sub := fluxjetstream.NewSubscriber(noCachedInfo{cons}, json.NewCodec[Event]().Decode)

	err = sub.SubscribeWithReady(t.Context(), "info.run", nopHandler, func() {
		t.Error("ready called although consumer info failed")
	})
	if !errors.Is(err, goflux.ErrSubscribe) || !errors.Is(err, jetstream.ErrConsumerNotFound) {
		t.Fatalf("expected ErrSubscribe wrapping ErrConsumerNotFound, got %v", err)
	}
}

// TestSubscriber_stopAfterBlocks asserts that an intentional stop through
// jetstream.StopAfter is not reported as an error and Subscribe keeps
// blocking until ctx is cancelled.
func TestSubscriber_stopAfterBlocks(t *testing.T) {
	t.Parallel()

	delivered := make(chan struct{})

	ctx, cancel := context.WithCancel(t.Context())
	_, js, done := startSubscriber(t, ctx, func(context.Context, goflux.Message[Event]) error {
		close(delivered)

		return nil
	}, fluxjetstream.WithConsumeOpts(jetstream.StopAfter(1)))

	publishStored(t, t.Context(), js, "consume.run", "1")
	waitFor(t, delivered, 5*time.Second, "delivery")

	expectBlocking(t, done, 500*time.Millisecond)
	cancel()

	if err := expectResult(t, done, 5*time.Second); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// TestSubscriber_heartbeatConflictsWithDefaultExpiry asserts that a caller
// heartbeat that is invalid for DefaultPullExpiry, but valid for nats.go's
// default expiry, does not fail Subscribe.
func TestSubscriber_heartbeatConflictsWithDefaultExpiry(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	_, _, done := startSubscriber(t, ctx, nopHandler,
		fluxjetstream.WithConsumeOpts(jetstream.PullHeartbeat(fluxjetstream.DefaultPullExpiry)))

	expectBlocking(t, done, 200*time.Millisecond)
	cancel()

	if err := expectResult(t, done, 5*time.Second); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// TestSubscriber_cancelWaitsForHandler asserts that Subscribe does not return
// on ctx cancellation until the in-flight handler has returned.
func TestSubscriber_cancelWaitsForHandler(t *testing.T) {
	t.Parallel()

	started := make(chan struct{})
	release := make(chan struct{})

	ctx, cancel := context.WithCancel(t.Context())
	_, js, done := startSubscriber(t, ctx, func(context.Context, goflux.Message[Event]) error {
		close(started)
		<-release

		return nil
	})

	publishStored(t, t.Context(), js, "consume.run", "1")
	waitFor(t, started, 5*time.Second, "handler start")
	cancel()

	select {
	case err := <-done:
		t.Fatalf("Subscribe returned while the handler was running: %v", err)
	case <-time.After(200 * time.Millisecond):
	}

	close(release)

	if err := expectResult(t, done, 5*time.Second); err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// TestSubscriber_withConsumeOpts asserts that caller options passed via
// WithConsumeOpts override the goflux defaults: a short pull expiry surfaces
// a missing consumer quickly, and the custom error handler receives it.
func TestSubscriber_withConsumeOpts(t *testing.T) {
	t.Parallel()

	errs := make(chan error, 8)
	_, js, _ := startSubscriber(t, t.Context(), nopHandler, fluxjetstream.WithConsumeOpts(
		jetstream.PullExpiry(time.Second),
		jetstream.ConsumeErrHandler(func(_ jetstream.ConsumeContext, err error) {
			select {
			case errs <- err:
			default:
			}
		}),
	))

	if err := js.DeleteConsumer(t.Context(), "CONSUME", "consume"); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-errs:
		// Depending on whether a pull was pending at deletion, nats.go reports
		// the terminal ErrConsumerDeleted or keeps retrying with heartbeat /
		// no-responders errors.
		if !errors.Is(err, jetstream.ErrConsumerDeleted) &&
			!errors.Is(err, jetstream.ErrNoHeartbeat) && !errors.Is(err, nats.ErrNoResponders) {
			t.Fatalf("expected missing-consumer error, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("custom ConsumeErrHandler was not called within 5s of deleting the consumer")
	}
}
