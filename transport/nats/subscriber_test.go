package nats_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/foomo/goencode/json/v1"
	"github.com/foomo/goflux"
	fluxnats "github.com/foomo/goflux/transport/nats"
	"github.com/foomo/gofuncy"
)

func ExampleNewSubscriber() {
	srv, conn := newServer()
	defer srv.Shutdown()
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	sub := fluxnats.NewSubscriber(conn, json.NewCodec[Event]().Decode)

	ready := make(chan struct{})

	go func() {
		defer close(done)

		_ = sub.SubscribeWithReady(ctx, "events", func(_ context.Context, msg goflux.Message[Event]) error {
			fmt.Println(msg.Subject, msg.Payload)
			cancel()

			return nil
		}, func() { close(ready) })
	}()

	// Wait until the server has registered the subscription, then publish raw
	// JSON. Without this the publish can outrun the subscription and be dropped.
	<-ready

	if err := conn.Publish("events", []byte(`{"id":"1","name":"foo"}`)); err != nil {
		panic(err)
	}

	<-done

	// Output: events {1 foo}
}

// Compile-time check: the subscriber supports readiness signalling.
var _ goflux.ReadySubscriber[Event] = (*fluxnats.Subscriber[Event])(nil)

// TestSubscriber_messageID asserts that the handler receives the publisher's
// message ID as msg.ID but not in its context, so its publishes do not
// inherit it.
func TestSubscriber_messageID(t *testing.T) {
	t.Parallel()

	srv, conn := newServer()
	t.Cleanup(srv.Shutdown)
	t.Cleanup(conn.Close)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ids := make(chan [2]string, 1)
	sub := fluxnats.NewSubscriber(conn, json.NewCodec[Event]().Decode)

	done := make(chan struct{})

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		defer close(done)

		return sub.SubscribeWithReady(ctx, "events", func(ctx context.Context, msg goflux.Message[Event]) error {
			ids <- [2]string{msg.ID, goflux.MessageID(ctx)}

			return nil
		}, ready)
	}, gofuncy.WithName("test.subscriber"))

	pub := fluxnats.NewPublisher(conn, json.NewCodec[Event]().Encode)
	if err := pub.Publish(goflux.WithMessageID(t.Context(), "id-1"), "events", Event{ID: "1"}); err != nil {
		t.Fatal(err)
	}

	if got := <-ids; got != [2]string{"id-1", ""} {
		t.Fatalf("expected msg.ID id-1 and no ctx ID, got %v", got)
	}

	cancel()
	<-done
}
