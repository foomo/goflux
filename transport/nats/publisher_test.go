package nats_test

import (
	"context"
	"fmt"

	"github.com/foomo/goencode/json/v1"
	"github.com/foomo/goflux"
	fluxnats "github.com/foomo/goflux/transport/nats"
	"github.com/nats-io/nats.go"
)

// Compile-time interface compliance checks.
var (
	_ goflux.Publisher[Event]        = (*fluxnats.Publisher[Event])(nil)
	_ goflux.Subscriber[Event]       = (*fluxnats.Subscriber[Event])(nil)
	_ goflux.Requester[Event, Event] = (*fluxnats.Requester[Event, Event])(nil)
	_ goflux.Responder[Event, Event] = (*fluxnats.Responder[Event, Event])(nil)
)

func ExampleNewPublisher() {
	srv, conn := newServer()
	defer srv.Shutdown()
	defer conn.Close()

	// Subscribe via raw NATS to observe the published message.
	ch := make(chan []byte, 1)

	sub, err := conn.Subscribe("events", func(msg *nats.Msg) {
		ch <- msg.Data
	})
	if err != nil {
		panic(err)
	}

	defer func() { _ = sub.Unsubscribe() }()

	pub := fluxnats.NewPublisher(conn, json.NewCodec[Event]())
	if err := pub.Publish(context.Background(), "events", Event{ID: "1", Name: "foo"}); err != nil {
		panic(err)
	}

	fmt.Println(string(<-ch))

	// Output: {"id":"1","name":"foo"}
}
