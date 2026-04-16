package middleware_test

import (
	"context"
	"fmt"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/middleware"
)

// ExampleAutoAck demonstrates the AutoAck middleware. On handler success,
// Ack is called; on handler error, Nak is called.
func ExampleAutoAck() {
	// Spy acker records which method was called.
	acker := &spyAcker{}

	msg := goflux.NewMessage("events", "payload").WithAcker(acker)

	handler := middleware.AutoAck[string]()(func(_ context.Context, _ goflux.Message[string]) error {
		return nil // success
	})

	_ = handler(context.Background(), msg)

	fmt.Println("acked:", acker.acked)
	fmt.Println("naked:", acker.naked)
	// Output:
	// acked: true
	// naked: false
}

// spyAcker records ack/nak calls for testing.
type spyAcker struct {
	acked bool
	naked bool
}

func (s *spyAcker) Ack() error { s.acked = true; return nil }
func (s *spyAcker) Nak() error { s.naked = true; return nil }
