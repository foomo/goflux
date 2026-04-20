package pipe_test

import (
	"testing"
	"time"

	"github.com/foomo/goflow"
	"github.com/foomo/goflux"
	"github.com/foomo/goflux/pipe"
	"github.com/foomo/goflux/transport/channel"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFromStream(t *testing.T) {
	ctx := t.Context()

	dstBus := channel.NewBus[Event]()
	dstPub := channel.NewPublisher(dstBus)
	dstSub, err := channel.NewSubscriber(dstBus, 1)
	require.NoError(t, err)

	dstCh := pipe.ToChan[Event](ctx, dstSub, "events", 4)

	time.Sleep(50 * time.Millisecond)

	msgs := []goflux.Message[Event]{
		goflux.NewMessage("events", Event{ID: "1", Name: "alpha"}),
		goflux.NewMessage("events", Event{ID: "2", Name: "bravo"}),
	}
	stream := goflow.Of(ctx, msgs...)

	err = pipe.FromStream(stream, dstPub, "")
	require.NoError(t, err)

	msg1 := <-dstCh
	msg2 := <-dstCh

	assert.Equal(t, "alpha", msg1.Payload.Name)
	assert.Equal(t, "bravo", msg2.Payload.Name)
}

func TestBoundFromStream(t *testing.T) {
	ctx := t.Context()

	dstBus := channel.NewBus[Event]()
	dstPub := channel.NewPublisher(dstBus)
	dstSub, err := channel.NewSubscriber(dstBus, 1)
	require.NoError(t, err)

	dstCh := pipe.ToChan[Event](ctx, dstSub, "events", 4)

	time.Sleep(50 * time.Millisecond)

	boundPub := goflux.BindPublisher[Event](dstPub, "events")

	msgs := []goflux.Message[Event]{
		goflux.NewMessage("events", Event{ID: "1", Name: "alpha"}),
		goflux.NewMessage("events", Event{ID: "2", Name: "bravo"}),
	}
	stream := goflow.Of(ctx, msgs...)

	err = pipe.BoundFromStream(stream, boundPub)
	require.NoError(t, err)

	msg1 := <-dstCh
	msg2 := <-dstCh

	assert.Equal(t, "alpha", msg1.Payload.Name)
	assert.Equal(t, "bravo", msg2.Payload.Name)
}
