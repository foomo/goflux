package goflux_test

import (
	"testing"
	"time"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/transport/channel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToStream(t *testing.T) {
	ctx := t.Context()

	bus := channel.NewBus[Event]()
	pub := channel.NewPublisher(bus)

	sub, err := channel.NewSubscriber(bus, 1)
	require.NoError(t, err)

	stream := goflux.ToStream[Event](ctx, sub, "events", 4)

	// Publish in a goroutine after subscriber has time to register.
	go func() {
		time.Sleep(50 * time.Millisecond)

		_ = pub.Publish(ctx, "events", Event{ID: "1", Name: "alpha"})
		_ = pub.Publish(ctx, "events", Event{ID: "2", Name: "bravo"})
	}()

	// Read two messages from the stream's underlying channel.
	ch := stream.Chan()
	msg1 := <-ch
	msg2 := <-ch

	assert.Equal(t, "alpha", msg1.Payload.Name)
	assert.Equal(t, "bravo", msg2.Payload.Name)
}
