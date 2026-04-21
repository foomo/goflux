package bridge_test

import (
	"testing"
	"time"

	"github.com/foomo/goflux/bridge"
	"github.com/foomo/goflux/transport/channel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Event struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func TestToStream(t *testing.T) {
	ctx := t.Context()

	bus := channel.NewBus[Event]()
	pub := channel.NewPublisher(bus)

	sub, err := channel.NewSubscriber(bus, 1)
	require.NoError(t, err)

	stream := bridge.ToStream[Event](ctx, sub, "events", 4)

	go func() {
		time.Sleep(50 * time.Millisecond)

		_ = pub.Publish(ctx, "events", Event{ID: "1", Name: "alpha"})
		_ = pub.Publish(ctx, "events", Event{ID: "2", Name: "bravo"})
	}()

	ch := stream.Chan()
	msg1 := <-ch
	msg2 := <-ch

	assert.Equal(t, "alpha", msg1.Payload.Name)
	assert.Equal(t, "bravo", msg2.Payload.Name)
}
