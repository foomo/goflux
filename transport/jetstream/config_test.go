package jetstream_test

import (
	"fmt"

	fluxjetstream "github.com/foomo/goflux/transport/jetstream"
)

func ExampleConsumerConfig() {
	cfg := fluxjetstream.ConsumerConfig("orders", "billing")
	fmt.Printf("name=%s durable=%q\n", cfg.Name, cfg.Durable)

	// Opt in to a durable consumer.
	cfg.Durable = cfg.Name
	fmt.Printf("name=%s durable=%q\n", cfg.Name, cfg.Durable)

	// Output:
	// name=goflux_billing_orders durable=""
	// name=goflux_billing_orders durable="goflux_billing_orders"
}
