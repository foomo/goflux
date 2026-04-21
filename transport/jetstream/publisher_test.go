package jetstream_test

import (
	"context"
	"fmt"

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
