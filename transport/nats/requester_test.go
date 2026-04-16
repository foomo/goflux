package nats_test

import (
	"context"
	"fmt"

	"github.com/foomo/goencode/json/v1"
	fluxnats "github.com/foomo/goflux/transport/nats"
	"github.com/nats-io/nats.go"
)

func ExampleNewRequester() {
	srv, conn := newServer()
	defer srv.Shutdown()
	defer conn.Close()

	// Set up a raw NATS responder that echoes back with a modified name.
	sub, err := conn.Subscribe("greet", func(msg *nats.Msg) {
		_ = msg.Respond([]byte(`{"id":"1","name":"hello back"}`))
	})
	if err != nil {
		panic(err)
	}

	defer func() { _ = sub.Unsubscribe() }()

	req := fluxnats.NewRequester[Event, Event](
		conn,
		json.NewCodec[Event](),
		json.NewCodec[Event](),
	)

	resp, err := req.Request(context.Background(), "greet", Event{ID: "1", Name: "hello"})
	if err != nil {
		panic(err)
	}

	fmt.Println(resp.ID, resp.Name)
	// Output: 1 hello back
}
