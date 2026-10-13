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
	decision := policy(goflux.ErrNonRetryable, goflux.Delivery{})

	if decision.Action != middleware.RetryTerm {
		t.Fatalf("expected RetryTerm, got %v", decision.Action)
	}
}

func TestNewRetryPolicy_Retryable(t *testing.T) {
	t.Parallel()

	delay := 5 * time.Second
	policy := middleware.NewRetryPolicy(delay)
	decision := policy(errors.New("transient"), goflux.Delivery{})

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
	decision := policy(goflux.NonRetryable(errors.New("bad data")), goflux.Delivery{})

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
	d := policy(errors.New("connection reset"), goflux.Delivery{})
	fmt.Printf("action=%d delay=%v\n", d.Action, d.Delay)

	// Non-retryable error.
	d = policy(goflux.NonRetryable(errors.New("schema mismatch")), goflux.Delivery{})
	fmt.Printf("action=%d\n", d.Action)

	// Output:
	// action=1 delay=2s
	// action=2
}

func TestNewBackoffRetryPolicy_GrowsWithDeliveryCount(t *testing.T) {
	t.Parallel()

	policy := middleware.NewBackoffRetryPolicy(time.Second, 10*time.Second)

	tests := []struct {
		numDelivered uint64
		want         time.Duration
	}{
		{0, time.Second}, // unknown → first delivery
		{1, time.Second},
		{2, 2 * time.Second},
		{3, 4 * time.Second},
		{4, 8 * time.Second},
		{5, 10 * time.Second}, // capped
		{100, 10 * time.Second},
	}

	for _, tt := range tests {
		d := policy(errors.New("transient"), goflux.Delivery{NumDelivered: tt.numDelivered})
		if d.Action != middleware.RetryNakWithDelay {
			t.Fatalf("delivery %d: expected RetryNakWithDelay, got %v", tt.numDelivered, d.Action)
		}

		if d.Delay != tt.want {
			t.Fatalf("delivery %d: expected delay %v, got %v", tt.numDelivered, tt.want, d.Delay)
		}
	}
}

func TestNewBackoffRetryPolicy_Factor(t *testing.T) {
	t.Parallel()

	policy := middleware.NewBackoffRetryPolicy(time.Second, time.Minute, middleware.WithBackoffFactor(3))

	d := policy(errors.New("transient"), goflux.Delivery{NumDelivered: 3})
	if d.Delay != 9*time.Second {
		t.Fatalf("expected delay 9s, got %v", d.Delay)
	}
}

func TestNewBackoffRetryPolicy_MaxDeliveries(t *testing.T) {
	t.Parallel()

	policy := middleware.NewBackoffRetryPolicy(time.Second, time.Minute, middleware.WithMaxDeliveries(3))

	if d := policy(errors.New("transient"), goflux.Delivery{NumDelivered: 2}); d.Action != middleware.RetryNakWithDelay {
		t.Fatalf("expected RetryNakWithDelay before limit, got %v", d.Action)
	}

	if d := policy(errors.New("transient"), goflux.Delivery{NumDelivered: 3}); d.Action != middleware.RetryTerm {
		t.Fatalf("expected RetryTerm at limit, got %v", d.Action)
	}
}

func TestNewBackoffRetryPolicy_NonRetryable(t *testing.T) {
	t.Parallel()

	policy := middleware.NewBackoffRetryPolicy(time.Second, time.Minute)

	if d := policy(goflux.NonRetryable(errors.New("bad")), goflux.Delivery{NumDelivered: 1}); d.Action != middleware.RetryTerm {
		t.Fatalf("expected RetryTerm, got %v", d.Action)
	}
}

func TestRetryAck_PassesDeliveryToPolicy(t *testing.T) {
	t.Parallel()

	policy := middleware.NewBackoffRetryPolicy(time.Second, time.Minute)
	acker := &recordingAcker{numDelivered: 3}
	msg := goflux.Message[string]{Subject: "test", Payload: "fail"}.WithAcker(acker)

	handler := middleware.RetryAck[string](policy)(
		func(_ context.Context, _ goflux.Message[string]) error {
			return errors.New("transient")
		},
	)

	_ = handler(context.Background(), msg)

	if acker.delay != 4*time.Second {
		t.Fatalf("expected delay 4s for third delivery, got %v", acker.delay)
	}
}

// ExampleNewBackoffRetryPolicy demonstrates a delay that grows with the
// message's delivery count and terminates after a maximum.
func ExampleNewBackoffRetryPolicy() {
	policy := middleware.NewBackoffRetryPolicy(time.Second, 30*time.Second,
		middleware.WithMaxDeliveries(5),
	)

	for n := uint64(1); n <= 5; n++ {
		d := policy(errors.New("timeout"), goflux.Delivery{NumDelivered: n})
		fmt.Printf("delivery=%d action=%d delay=%v\n", n, d.Action, d.Delay)
	}

	// Output:
	// delivery=1 action=1 delay=1s
	// delivery=2 action=1 delay=2s
	// delivery=3 action=1 delay=4s
	// delivery=4 action=1 delay=8s
	// delivery=5 action=2 delay=0s
}

type recordingAcker struct {
	acked          bool
	naked          bool
	nakedWithDelay bool
	delay          time.Duration
	termed         bool
	numDelivered   uint64
}

func (a *recordingAcker) Delivery() goflux.Delivery {
	return goflux.Delivery{NumDelivered: a.numDelivered}
}

func (a *recordingAcker) Ack() error { a.acked = true; return nil }
func (a *recordingAcker) Nak() error { a.naked = true; return nil }

func (a *recordingAcker) NakWithDelay(d time.Duration) error {
	a.nakedWithDelay = true
	a.delay = d

	return nil
}

func (a *recordingAcker) Term() error { a.termed = true; return nil }
