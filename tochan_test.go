package goflux_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/transport/channel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ExampleToChan() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := channel.NewBus[string]()
	pub := channel.NewPublisher(bus)

	sub, _ := channel.NewSubscriber(bus, 1)

	ch := goflux.ToChan[string](ctx, sub, "test", 4)

	time.Sleep(10 * time.Millisecond)

	_ = pub.Publish(ctx, "test", "alpha")
	_ = pub.Publish(ctx, "test", "bravo")

	fmt.Println((<-ch).Payload)
	fmt.Println((<-ch).Payload)
	// Output:
	// alpha
	// bravo
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
