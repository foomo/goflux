package nats_test

import (
	"context"
	"fmt"

	"github.com/foomo/goencode/json/v1"
	"github.com/foomo/goflux"
	fluxnats "github.com/foomo/goflux/transport/nats"
)

func ExampleNewSubscriber() {
	srv, conn := newServer()
	defer srv.Shutdown()
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	sub := fluxnats.NewSubscriber(conn, json.NewCodec[Event]().Decode)

	go func() {
		defer close(done)

		_ = sub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Event]) error {
			fmt.Println(msg.Subject, msg.Payload)
			cancel()

			return nil
		})
	}()

	// Wait for subscription to be ready, then publish raw JSON.
	conn.Flush()

	if err := conn.Publish("events", []byte(`{"id":"1","name":"foo"}`)); err != nil {
		panic(err)
	}

	<-done

	// Output: events {1 foo}
}
