package goflux_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/foomo/goflux"
)

// ExampleProcess demonstrates limiting concurrent handler invocations.
func ExampleProcess() {
	var (
		maxConcurrent atomic.Int64
		current       atomic.Int64
		mu            sync.Mutex
		received      []string
		wg            sync.WaitGroup
	)

	handler := goflux.Chain[Event](
		goflux.Process[Event](2),
	)(func(_ context.Context, msg goflux.Message[Event]) error {
		cur := current.Add(1)
		defer current.Add(-1)

		// Track peak concurrency.
		for {
			old := maxConcurrent.Load()
			if cur <= old || maxConcurrent.CompareAndSwap(old, cur) {
				break
			}
		}

		mu.Lock()

		received = append(received, msg.Payload.Name)
		mu.Unlock()

		return nil
	})

	ctx := context.Background()

	for i := range 5 {
		wg.Go(func() {
			_ = handler(ctx, goflux.NewMessage("events", Event{
				ID:   fmt.Sprintf("%d", i),
				Name: fmt.Sprintf("msg-%d", i),
			}))
		})
	}

	wg.Wait()

	fmt.Println("count:", len(received))
	fmt.Println("max concurrent <= 2:", maxConcurrent.Load() <= 2)
	// Output:
	// count: 5
	// max concurrent <= 2: true
}
