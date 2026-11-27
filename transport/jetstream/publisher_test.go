package jetstream_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/foomo/goencode/json/v1"
	"github.com/foomo/goflux"
	fluxjetstream "github.com/foomo/goflux/transport/jetstream"
	"github.com/nats-io/nats.go/jetstream"
)

// Compile-time interface compliance check.
var _ goflux.Publisher[Event] = (*fluxjetstream.Publisher[Event])(nil)

func ExampleNewPublisher() {
	srv, conn := newServer()
	defer srv.Shutdown()
	defer conn.Close()

	js, err := fluxjetstream.NewStream(context.Background(), conn, jetstream.StreamConfig{
		Name:     "EVENTS",
		Subjects: []string{"events.>"},
	})
	if err != nil {
		panic(err)
	}

	// Create a pull consumer to observe the published message.
	cons, err := js.CreateConsumer(context.Background(), "EVENTS", jetstream.ConsumerConfig{
		Durable:   "observer",
		AckPolicy: jetstream.AckExplicitPolicy,
	})
	if err != nil {
		panic(err)
	}

	pub := fluxjetstream.NewPublisher(js, json.NewCodec[Event]().Encode)
	if err := pub.Publish(context.Background(), "events.created", Event{ID: "1", Name: "foo"}); err != nil {
		panic(err)
	}

	msgs, err := cons.Fetch(1)
	if err != nil {
		panic(err)
	}

	for msg := range msgs.Messages() {
		fmt.Println(string(msg.Data()))
		_ = msg.Ack()
	}

	// Output: {"id":"1","name":"foo"}
}

// TestPublisher_messageIDDeduplicates asserts that the goflux message ID is
// sent as Nats-Msg-Id, so the stream stores a repeated publish only once.
func TestPublisher_messageIDDeduplicates(t *testing.T) {
	t.Parallel()

	srv := startServerOnPort(t, -1)
	conn, _, _ := connectWithReconnect(t, srv.ClientURL())

	js, err := fluxjetstream.NewStream(t.Context(), conn, jetstream.StreamConfig{Name: "DEDUP", Subjects: []string{"dedup.>"}})
	if err != nil {
		t.Fatal(err)
	}

	pub := fluxjetstream.NewPublisher(js, json.NewCodec[Event]().Encode)
	ctx := goflux.WithMessageID(t.Context(), "id-1")

	for range 2 {
		if err := pub.Publish(ctx, "dedup.run", Event{ID: "1"}); err != nil {
			t.Fatal(err)
		}
	}

	if err := pub.Publish(goflux.WithMessageID(t.Context(), "id-2"), "dedup.run", Event{ID: "2"}); err != nil {
		t.Fatal(err)
	}

	stream, err := js.Stream(t.Context(), "DEDUP")
	if err != nil {
		t.Fatal(err)
	}

	info, err := stream.Info(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if info.State.Msgs != 2 {
		t.Fatalf("expected 2 stored messages, got %d", info.State.Msgs)
	}
}

// TestSubscriber_handlerPublishesDoNotInheritMessageID asserts that messages
// published from a handler do not inherit the received message ID, so the
// stream does not drop them as duplicates when they land in the same stream.
func TestSubscriber_handlerPublishesDoNotInheritMessageID(t *testing.T) {
	t.Parallel()

	type received struct{ msgID, ctxID string }

	got := make(chan received, 1)

	var pub atomic.Pointer[fluxjetstream.Publisher[Event]]

	handler := func(ctx context.Context, msg goflux.Message[Event]) error {
		if msg.Subject != "consume.run" {
			return msg.Ack()
		}

		got <- received{msgID: msg.ID, ctxID: goflux.MessageID(ctx)}

		for i := range 3 {
			if err := pub.Load().Publish(ctx, "consume.out", Event{ID: fmt.Sprint(i)}); err != nil {
				return err
			}
		}

		return msg.Ack()
	}

	_, js, _ := startSubscriber(t, t.Context(), handler)
	pub.Store(fluxjetstream.NewPublisher(js, json.NewCodec[Event]().Encode))

	if err := pub.Load().Publish(goflux.WithMessageID(t.Context(), "id-1"), "consume.run", Event{ID: "in"}); err != nil {
		t.Fatal(err)
	}

	if r := <-got; r.msgID != "id-1" || r.ctxID != "" {
		t.Fatalf("expected msg.ID id-1 and no ctx ID, got %+v", r)
	}

	stream, err := js.Stream(t.Context(), "CONSUME")
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)

	for {
		info, err := stream.Info(t.Context())
		if err != nil {
			t.Fatal(err)
		}

		if info.State.Msgs == 4 {
			return
		}

		if time.Now().After(deadline) {
			t.Fatalf("expected 4 stored messages, got %d", info.State.Msgs)
		}

		time.Sleep(50 * time.Millisecond)
	}
}
