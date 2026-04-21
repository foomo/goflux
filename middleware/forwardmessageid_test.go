package middleware_test

import (
	"context"
	"testing"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForwardMessageID(t *testing.T) {
	var capturedCtx context.Context

	inner := func(ctx context.Context, msg goflux.Message[string]) error {
		capturedCtx = ctx

		return nil
	}

	handler := middleware.ForwardMessageID[string]()(inner)

	// Set message ID in incoming context.
	ctx := goflux.WithMessageID(context.Background(), "msg-123")
	msg := goflux.NewMessage("test", "payload")

	err := handler(ctx, msg)
	require.NoError(t, err)

	// Message ID should be forwarded through context.
	assert.Equal(t, "msg-123", goflux.MessageID(capturedCtx))
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

	// No message ID set — should remain empty.
	assert.Empty(t, goflux.MessageID(capturedCtx))
}
