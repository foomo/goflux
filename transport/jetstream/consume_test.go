package jetstream_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/foomo/goencode/json/v1"
	"github.com/foomo/goflux"
	fluxjetstream "github.com/foomo/goflux/transport/jetstream"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// startSubscriber creates a stream and durable consumer, runs
// SubscribeWithReady until it is ready, and returns the connection, JetStream
// handle and the channel that receives Subscribe's result.
func startSubscriber(t *testing.T, opts ...fluxjetstream.Option) (*nats.Conn, jetstream.JetStream, <-chan error) {
	t.Helper()

	srv := startServerOnPort(t, -1)
	conn, _, _ := connectWithReconnect(t, srv.ClientURL())

	ctx := t.Context()

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
	ready := make(chan struct{})
	done := make(chan error, 1)

	go func() {
		done <- sub.SubscribeWithReady(ctx, "consume.run", func(context.Context, goflux.Message[Event]) error {
			return nil
		}, func() { close(ready) })
	}()

	waitFor(t, ready, 10*time.Second, "subscription ready")

	return conn, js, done
}

// TestSubscriber_connectionClosedReturnsError asserts that Subscribe returns
// an error instead of blocking silently when consumption stops on a terminal
// error.
func TestSubscriber_connectionClosedReturnsError(t *testing.T) {
	t.Parallel()

	conn, _, done := startSubscriber(t)
	conn.Close()

	select {
	case err := <-done:
		if !errors.Is(err, goflux.ErrSubscribe) {
			t.Fatalf("expected ErrSubscribe, got %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Subscribe kept blocking after the connection was closed")
	}
}

// TestSubscriber_withConsumeOpts asserts that caller options passed via
// WithConsumeOpts override the goflux defaults: a short pull expiry surfaces
// a missing consumer quickly, and the custom error handler receives it.
func TestSubscriber_withConsumeOpts(t *testing.T) {
	t.Parallel()

	errs := make(chan error, 8)
	_, js, _ := startSubscriber(t, fluxjetstream.WithConsumeOpts(
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
