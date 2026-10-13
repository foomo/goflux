package middleware_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/middleware"
)

func TestKeepAlive_SignalsInProgressWhileRunning(t *testing.T) {
	t.Parallel()

	acker := &inProgressAcker{}
	msg := goflux.NewMessage("events", "payload").WithAcker(acker)

	handler := middleware.KeepAlive[string](5 * time.Millisecond)(
		func(_ context.Context, _ goflux.Message[string]) error {
			time.Sleep(50 * time.Millisecond)

			return nil
		},
	)

	if err := handler(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	after := acker.inProgress.Load()
	if after == 0 {
		t.Fatal("expected InProgress to be called while handler runs")
	}

	time.Sleep(30 * time.Millisecond)

	if got := acker.inProgress.Load(); got != after {
		t.Fatalf("expected no InProgress after handler returned, got %d more", got-after)
	}
}

func TestKeepAlive_NoAcker(t *testing.T) {
	t.Parallel()

	called := false
	handler := middleware.KeepAlive[string](time.Millisecond)(
		func(_ context.Context, _ goflux.Message[string]) error {
			called = true

			return nil
		},
	)

	if err := handler(context.Background(), goflux.NewMessage("events", "payload")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !called {
		t.Fatal("expected handler to be called")
	}
}

func TestKeepAlive_CancelledContext(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	handler := middleware.KeepAlive[string](time.Millisecond)(
		func(_ context.Context, _ goflux.Message[string]) error { return nil },
	)

	if err := handler(ctx, goflux.NewMessage("events", "payload").WithAcker(&inProgressAcker{})); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

type inProgressAcker struct {
	inProgress atomic.Int64
}

func (a *inProgressAcker) Ack() error        { return nil }
func (a *inProgressAcker) Nak() error        { return nil }
func (a *inProgressAcker) InProgress() error { a.inProgress.Add(1); return nil }
