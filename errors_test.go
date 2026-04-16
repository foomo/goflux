package goflux_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/foomo/goflux"
)

func TestNonRetryable_Wrapping(t *testing.T) {
	t.Parallel()

	orig := errors.New("bad input")
	wrapped := goflux.NonRetryable(orig)

	if !errors.Is(wrapped, goflux.ErrNonRetryable) {
		t.Fatal("wrapped error should match ErrNonRetryable")
	}
}

func TestNonRetryable_Unwrap(t *testing.T) {
	t.Parallel()

	orig := errors.New("bad input")
	wrapped := goflux.NonRetryable(orig)

	if !errors.Is(wrapped, orig) {
		t.Fatal("wrapped error should still match original error")
	}
}

func TestIsNonRetryable_PlainError(t *testing.T) {
	t.Parallel()

	if goflux.IsNonRetryable(errors.New("transient")) {
		t.Fatal("plain error should not be non-retryable")
	}
}

func TestIsNonRetryable_WrappedError(t *testing.T) {
	t.Parallel()

	err := goflux.NonRetryable(errors.New("bad input"))
	if !goflux.IsNonRetryable(err) {
		t.Fatal("NonRetryable-wrapped error should be non-retryable")
	}
}

// ExampleNonRetryable demonstrates wrapping and detecting non-retryable errors.
func ExampleNonRetryable() {
	orig := errors.New("invalid payload")
	wrapped := goflux.NonRetryable(orig)

	fmt.Println("is non-retryable:", goflux.IsNonRetryable(wrapped))
	fmt.Println("is original:", errors.Is(wrapped, orig))
	fmt.Println("plain error:", goflux.IsNonRetryable(errors.New("transient")))
	// Output:
	// is non-retryable: true
	// is original: true
	// plain error: false
}
