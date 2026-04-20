package goflux_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/transport/channel"
	"github.com/foomo/gofuncy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ExampleTopic demonstrates bundling a Publisher and Subscriber into a single
// Topic value. This is useful when a service needs to both produce and consume
// the same message type.
func ExampleTopic() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	bus := channel.NewBus[Event]()

	sub, err := channel.NewSubscriber(bus, 1)
	if err != nil {
		panic(err)
	}

	topic := goflux.Topic[Event]{
		Publisher:  channel.NewPublisher(bus),
		Subscriber: sub,
	}

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return topic.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Event]) error {
			fmt.Println(msg.Payload.Name)
			cancel()

			return nil
		})
	}, gofuncy.WithName("subscriber"))

	time.Sleep(10 * time.Millisecond)

	if err := topic.Publish(ctx, "events", Event{ID: "1", Name: "bundled"}); err != nil {
		panic(err)
	}

	<-ctx.Done()
	// Output: bundled
}

func TestBoundTopic(t *testing.T) {
	ctx := t.Context()

	bus := channel.NewBus[string]()
	pub := channel.NewPublisher(bus)

	sub, err := channel.NewSubscriber(bus, 1)
	require.NoError(t, err)

	bt := goflux.BindTopic[string](pub, sub, "bound-subject")

	var got goflux.Message[string]
	done := make(chan struct{})

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return bt.Subscribe(ctx, func(_ context.Context, msg goflux.Message[string]) error {
			got = msg
			close(done)

			return nil
		})
	}, gofuncy.WithName("bound-topic-subscriber"))

	time.Sleep(10 * time.Millisecond)

	require.NoError(t, bt.Publish(ctx, "hello"))
	<-done

	assert.Equal(t, "bound-subject", got.Subject)
	assert.Equal(t, "hello", got.Payload)
}
