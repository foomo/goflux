package nats_test

import (
	"context"
	"fmt"
	"time"

	"github.com/foomo/goencode/json/v1"
	"github.com/foomo/goflux"
	fluxnats "github.com/foomo/goflux/transport/nats"
)

func ExampleNewResponder() {
	srv, conn := newServer()
	defer srv.Shutdown()
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resp := fluxnats.NewResponder[Event, Event](
		conn,
		json.NewCodec[Event](),
		json.NewCodec[Event](),
	)

	// Start the responder in the background.
	go func() {
		_ = resp.Serve(ctx, "greet", func(_ context.Context, req Event) (Event, error) {
			return Event{ID: req.ID, Name: "hello " + req.Name}, nil
		})
	}()

	// Allow the subscription to register.
	conn.Flush()

	// Send a request via a separate Requester.
	requester := fluxnats.NewRequester[Event, Event](
		conn,
		json.NewCodec[Event](),
		json.NewCodec[Event](),
	)

	reqCtx, reqCancel := context.WithTimeout(ctx, 5*time.Second)
	defer reqCancel()

	reply, err := requester.Request(reqCtx, "greet", Event{ID: "1", Name: "world"})
	if err != nil {
		panic(err)
	}

	fmt.Println(reply.ID, reply.Name)
	// Output: 1 hello world
}

// Compile-time interface compliance check.
var _ goflux.Responder[Event, Event] = (*fluxnats.Responder[Event, Event])(nil)
