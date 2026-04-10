package goflux_test

import (
	"context"
	"fmt"

	"github.com/foomo/goflux"
)

// ExampleRoundRobin demonstrates distributing messages across publishers
// in round-robin order.
func ExampleRoundRobin() {
	ctx := context.Background()

	a := &collectPublisher[Event]{}
	b := &collectPublisher[Event]{}

	pub := goflux.RoundRobin[Event](a, b)

	_ = pub.Publish(ctx, "events", Event{ID: "1", Name: "first"})
	_ = pub.Publish(ctx, "events", Event{ID: "2", Name: "second"})
	_ = pub.Publish(ctx, "events", Event{ID: "3", Name: "third"})

	fmt.Println("a:", a.get())
	fmt.Println("b:", b.get())
	// Output:
	// a: [{1 first} {3 third}]
	// b: [{2 second}]
}
