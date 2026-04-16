package goflux_test

import (
	"testing"
	"time"

	"github.com/foomo/goflow"
	"github.com/foomo/goflux"
	"github.com/foomo/goflux/transport/channel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromStream(t *testing.T) {
	ctx := t.Context()

	// Destination bus.
	dstBus := channel.NewBus[Event]()
	dstPub := channel.NewPublisher(dstBus)
	dstSub, err := channel.NewSubscriber(dstBus, 1)
	require.NoError(t, err)

	// Consume destination via ToChan for verification.
	dstCh := goflux.ToChan[Event](ctx, dstSub, "events", 4)

	// Allow subscriber to register.
	time.Sleep(50 * time.Millisecond)

	// Create a stream of messages and publish them.
	msgs := []goflux.Message[Event]{
		goflux.NewMessage("events", Event{ID: "1", Name: "alpha"}),
		goflux.NewMessage("events", Event{ID: "2", Name: "bravo"}),
	}
	stream := goflow.Of(ctx, msgs...)

	err = goflux.FromStream(ctx, stream, dstPub)
	require.NoError(t, err)

	msg1 := <-dstCh
	msg2 := <-dstCh

	assert.Equal(t, "alpha", msg1.Payload.Name)
	assert.Equal(t, "bravo", msg2.Payload.Name)
}
