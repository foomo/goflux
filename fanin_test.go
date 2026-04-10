package goflux_test

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/pkg/channel"
	"github.com/foomo/gofuncy"
)

// ExampleFanIn demonstrates merging two subscribers into one handler.
// Messages from both source buses are dispatched to the same handler.
func ExampleFanIn() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	busA := channel.NewBus[Event]()
	pubA := channel.NewPublisher(busA)

	subA, err := channel.NewSubscriber(busA, 1)
	if err != nil {
		panic(err)
	}

	busB := channel.NewBus[Event]()
	pubB := channel.NewPublisher(busB)

	subB, err := channel.NewSubscriber(busB, 1)
	if err != nil {
		panic(err)
	}

	var (
		mu       sync.Mutex
		received []string
	)

	merged := goflux.FanIn[Event](subA, subB)

	done := make(chan struct{})

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return merged.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Event]) error {
			mu.Lock()

			received = append(received, msg.Payload.Name)
			if len(received) == 2 {
				close(done)
			}
			mu.Unlock()

			return nil
		})
	}, gofuncy.WithName("fanin-subscriber"))

	time.Sleep(10 * time.Millisecond)

	_ = pubA.Publish(ctx, "events", Event{ID: "1", Name: "from-a"})
	_ = pubB.Publish(ctx, "events", Event{ID: "2", Name: "from-b"})

	<-done

	mu.Lock()
	sort.Strings(received)

	for _, r := range received {
		fmt.Println(r)
	}
	mu.Unlock()
	// Output:
	// from-a
	// from-b
}
