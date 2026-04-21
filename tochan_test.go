package goflux_test

import (
	"context"
	"testing"
	"time"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/transport/channel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToChan(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	bus := channel.NewBus[string]()
	pub := channel.NewPublisher(bus)

	sub, err := channel.NewSubscriber(bus, 1)
	require.NoError(t, err)

	ch := goflux.ToChan[string](ctx, sub, "test", 4)

	time.Sleep(10 * time.Millisecond)

	require.NoError(t, pub.Publish(ctx, "test", "alpha"))
	require.NoError(t, pub.Publish(ctx, "test", "bravo"))

	msg1 := <-ch
	msg2 := <-ch

	assert.Equal(t, "alpha", msg1.Payload)
	assert.Equal(t, "bravo", msg2.Payload)
}

func TestToChan_closesOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())

	bus := channel.NewBus[string]()
	pub := channel.NewPublisher(bus)

	sub, err := channel.NewSubscriber(bus, 1)
	require.NoError(t, err)

	ch := goflux.ToChan[string](ctx, sub, "test", 1)

	time.Sleep(10 * time.Millisecond)

	// Send a message before cancelling so we know the subscriber is active.
	require.NoError(t, pub.Publish(ctx, "test", "hello"))

	msg := <-ch
	assert.Equal(t, "hello", msg.Payload)

	cancel()

	// After cancellation, channel should eventually close.
	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel should be closed")
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for channel to close")
	}
}
