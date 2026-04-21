package middleware_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ExampleForwardMessageID() {
	inner := func(ctx context.Context, msg goflux.Message[string]) error {
		fmt.Println(goflux.MessageID(ctx))

		return nil
	}

	handler := middleware.ForwardMessageID[string]()(inner)

	ctx := goflux.WithMessageID(context.Background(), "msg-123")

	_ = handler(ctx, goflux.NewMessage("test", "payload"))
	// Output: msg-123
}

func TestForwardMessageID_noID(t *testing.T) {
	var capturedCtx context.Context

	inner := func(ctx context.Context, msg goflux.Message[string]) error {
		capturedCtx = ctx

		return nil
	}

	handler := middleware.ForwardMessageID[string]()(inner)

	err := handler(context.Background(), goflux.NewMessage("test", "payload"))
	require.NoError(t, err)

	assert.Empty(t, goflux.MessageID(capturedCtx))
}
