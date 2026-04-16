package middleware_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/middleware"
)

func TestNewRetryPolicy_NonRetryable(t *testing.T) {
	t.Parallel()

	policy := middleware.NewRetryPolicy(5 * time.Second)
	decision := policy(goflux.ErrNonRetryable)

	if decision.Action != middleware.RetryTerm {
		t.Fatalf("expected RetryTerm, got %v", decision.Action)
	}
}

func TestNewRetryPolicy_Retryable(t *testing.T) {
	t.Parallel()

	delay := 5 * time.Second
	policy := middleware.NewRetryPolicy(delay)
	decision := policy(errors.New("transient"))

	if decision.Action != middleware.RetryNakWithDelay {
		t.Fatalf("expected RetryNakWithDelay, got %v", decision.Action)
	}

	if decision.Delay != delay {
		t.Fatalf("expected delay %v, got %v", delay, decision.Delay)
	}
}

func TestNewRetryPolicy_WrappedNonRetryable(t *testing.T) {
	t.Parallel()

	policy := middleware.NewRetryPolicy(5 * time.Second)
	decision := policy(goflux.NonRetryable(errors.New("bad data")))

	if decision.Action != middleware.RetryTerm {
		t.Fatalf("expected RetryTerm for wrapped non-retryable, got %v", decision.Action)
	}
}

func TestRetryAck_Success(t *testing.T) {
	t.Parallel()

	policy := middleware.NewRetryPolicy(time.Second)
	acker := &recordingAcker{}
	msg := goflux.Message[string]{Subject: "test", Payload: "ok"}.WithAcker(acker)

	handler := middleware.RetryAck[string](policy)(
		func(_ context.Context, _ goflux.Message[string]) error {
			return nil
		},
	)

	if err := handler(context.Background(), msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !acker.acked {
		t.Fatal("expected Ack to be called")
	}
}

func TestRetryAck_RetryableError(t *testing.T) {
	t.Parallel()

	delay := 2 * time.Second
	policy := middleware.NewRetryPolicy(delay)
	acker := &recordingAcker{}
	msg := goflux.Message[string]{Subject: "test", Payload: "fail"}.WithAcker(acker)

	handler := middleware.RetryAck[string](policy)(
		func(_ context.Context, _ goflux.Message[string]) error {
			return errors.New("transient")
		},
	)

	if err := handler(context.Background(), msg); err == nil {
		t.Fatal("expected error")
	}

	if !acker.nakedWithDelay {
		t.Fatal("expected NakWithDelay to be called")
	}

	if acker.delay != delay {
		t.Fatalf("expected delay %v, got %v", delay, acker.delay)
	}
}

func TestRetryAck_NonRetryableError(t *testing.T) {
	t.Parallel()

	policy := middleware.NewRetryPolicy(time.Second)
	acker := &recordingAcker{}
	msg := goflux.Message[string]{Subject: "test", Payload: "bad"}.WithAcker(acker)

	handler := middleware.RetryAck[string](policy)(
		func(_ context.Context, _ goflux.Message[string]) error {
			return goflux.NonRetryable(errors.New("schema mismatch"))
		},
	)

	if err := handler(context.Background(), msg); err == nil {
		t.Fatal("expected error")
	}

	if !acker.termed {
		t.Fatal("expected Term to be called")
	}
}

func TestRetryAck_NoAcker(t *testing.T) {
	t.Parallel()

	policy := middleware.NewRetryPolicy(time.Second)
	msg := goflux.Message[string]{Subject: "test", Payload: "fire-and-forget"}

	handler := middleware.RetryAck[string](policy)(
		func(_ context.Context, _ goflux.Message[string]) error {
			return errors.New("ignored")
		},
	)

	err := handler(context.Background(), msg)
	if err == nil {
		t.Fatal("expected error passthrough")
	}
}

// ExampleNewRetryPolicy demonstrates creating and using a RetryPolicy that
// classifies errors into retry actions.
func ExampleNewRetryPolicy() {
	policy := middleware.NewRetryPolicy(2 * time.Second)

	// Retryable error.
	d := policy(errors.New("connection reset"))
	fmt.Printf("action=%d delay=%v\n", d.Action, d.Delay)

	// Non-retryable error.
	d = policy(goflux.NonRetryable(errors.New("schema mismatch")))
	fmt.Printf("action=%d\n", d.Action)

	// Output:
	// action=1 delay=2s
	// action=2
}

type recordingAcker struct {
	acked          bool
	naked          bool
	nakedWithDelay bool
	delay          time.Duration
	termed         bool
}

func (a *recordingAcker) Ack() error { a.acked = true; return nil }
func (a *recordingAcker) Nak() error { a.naked = true; return nil }

func (a *recordingAcker) NakWithDelay(d time.Duration) error {
	a.nakedWithDelay = true
	a.delay = d

	return nil
}

func (a *recordingAcker) Term() error { a.termed = true; return nil }
