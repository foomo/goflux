package middleware_test

import (
	"context"
	"fmt"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/middleware"
)

// ExampleInjectMessageID demonstrates extracting a message ID from the
// message header and injecting it into the context. This is useful when
// composing handlers with Subscriber.Subscribe directly, where the transport
// may not inject the message ID automatically.
func ExampleInjectMessageID() {
	header := goflux.Header{}
	header.Set(goflux.MessageIDHeader, "order-123")

	msg := goflux.NewMessageWithHeader("events", "payload", header)

	handler := middleware.InjectMessageID[string]()(
		func(ctx context.Context, msg goflux.Message[string]) error {
			fmt.Println("id:", goflux.MessageID(ctx))

			return nil
		},
	)

	_ = handler(context.Background(), msg)
	// Output: id: order-123
}

// ExampleInjectHeader demonstrates injecting the full message header into
// the context so downstream code can access it via HeaderFromContext.
func ExampleInjectHeader() {
	header := goflux.Header{}
	header.Set("X-Tenant", "acme")
	header.Set("X-Region", "eu-west-1")

	msg := goflux.NewMessageWithHeader("events", "payload", header)

	handler := middleware.InjectHeader[string]()(
		func(ctx context.Context, msg goflux.Message[string]) error {
			h := goflux.HeaderFromContext(ctx)
			fmt.Println("tenant:", h.Get("X-Tenant"))
			fmt.Println("region:", h.Get("X-Region"))

			return nil
		},
	)

	_ = handler(context.Background(), msg)
	// Output:
	// tenant: acme
	// region: eu-west-1
}
