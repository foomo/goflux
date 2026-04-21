package pipe_test

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/pipe"
	"github.com/foomo/goflux/transport/channel"
	"github.com/foomo/gofuncy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Event struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Summary struct {
	Label string `json:"label"`
}

type LineItem struct {
	OrderID string `json:"order_id"`
	Item    string `json:"item"`
}

type Order struct {
	ID    string   `json:"id"`
	Items []string `json:"items"`
}

// failPublisher is a Publisher that always returns the given error.
type failPublisher[T any] struct{ err error }

func (p *failPublisher[T]) Publish(context.Context, string, T) error { return p.err }
func (p *failPublisher[T]) Close() error                             { return nil }

// countingFailPublisher publishes successfully until failAfter count, then returns err.
type countingFailPublisher[T any] struct {
	count     atomic.Int32
	failAfter int32
	err       error
}

func (p *countingFailPublisher[T]) Publish(context.Context, string, T) error {
	n := p.count.Add(1)
	if n > p.failAfter {
		return p.err
	}

	return nil
}

func (p *countingFailPublisher[T]) Close() error { return nil }

// ---------------------------------------------------------------------------
// New
// ---------------------------------------------------------------------------

func TestNew(t *testing.T) {
	ctx := t.Context()

	srcBus := channel.NewBus[Event]()
	srcPub := channel.NewPublisher(srcBus)
	srcSub, err := channel.NewSubscriber(srcBus, 1)
	require.NoError(t, err)

	dstBus := channel.NewBus[Event]()
	dstPub := channel.NewPublisher(dstBus)
	dstSub, err := channel.NewSubscriber(dstBus, 1)
	require.NoError(t, err)

	dstCh := make(chan goflux.Message[Event], 1)

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return dstSub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Event]) error {
			dstCh <- msg

			return nil
		})
	}, gofuncy.WithName("dst-subscriber"))

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return srcSub.Subscribe(ctx, "events", pipe.New[Event](dstPub))
	}, gofuncy.WithName("pipe"))

	require.NoError(t, srcPub.Publish(ctx, "events", Event{ID: "1", Name: "hello"}))

	select {
	case msg := <-dstCh:
		assert.Equal(t, "events", msg.Subject)
		assert.Equal(t, Event{ID: "1", Name: "hello"}, msg.Payload)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestNew_filterRejectsReturnsNil(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	srcBus := channel.NewBus[Event]()
	srcPub := channel.NewPublisher(srcBus)
	srcSub, err := channel.NewSubscriber(srcBus, 1)
	require.NoError(t, err)

	dstBus := channel.NewBus[Event]()
	dstPub := channel.NewPublisher(dstBus)
	dstSub, err := channel.NewSubscriber(dstBus, 1)
	require.NoError(t, err)

	dstCh := make(chan goflux.Message[Event], 1)

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return dstSub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Event]) error {
			dstCh <- msg

			return nil
		})
	}, gofuncy.WithName("dst-subscriber"))

	filter := func(_ context.Context, msg goflux.Message[Event]) bool {
		return msg.Payload.ID != "skip"
	}

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return srcSub.Subscribe(ctx, "events", pipe.New[Event](dstPub, pipe.WithFilter(filter)))
	}, gofuncy.WithName("pipe"))

	// Publish filtered message — should be silently skipped (nil return).
	require.NoError(t, srcPub.Publish(ctx, "events", Event{ID: "skip", Name: "ignored"}))

	// Publish accepted message.
	require.NoError(t, srcPub.Publish(ctx, "events", Event{ID: "keep", Name: "accepted"}))

	select {
	case msg := <-dstCh:
		assert.Equal(t, "keep", msg.Payload.ID)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestNew_publishErrorReturnsAndCallsDeadLetter(t *testing.T) {
	pubErr := errors.New("publish failed")
	badPub := &failPublisher[Event]{err: pubErr}

	var (
		dlCalled atomic.Bool
		dlMsg    goflux.Message[Event]
		dlErr    error
	)

	deadLetter := func(_ context.Context, msg goflux.Message[Event], err error) {
		dlMsg = msg
		dlErr = err

		dlCalled.Store(true)
	}

	handler := pipe.New[Event](badPub, pipe.WithDeadLetter(deadLetter))

	err := handler(context.Background(), goflux.NewMessage("events", Event{ID: "1", Name: "hello"}))

	require.Error(t, err)
	assert.Equal(t, pubErr, err)
	assert.True(t, dlCalled.Load())
	assert.Equal(t, "1", dlMsg.Payload.ID)
	assert.Equal(t, pubErr, dlErr)
}

func TestNew_withMiddleware(t *testing.T) {
	ctx := t.Context()

	srcBus := channel.NewBus[Event]()
	srcPub := channel.NewPublisher(srcBus)
	srcSub, err := channel.NewSubscriber(srcBus, 1)
	require.NoError(t, err)

	dstBus := channel.NewBus[Event]()
	dstPub := channel.NewPublisher(dstBus)
	dstSub, err := channel.NewSubscriber(dstBus, 1)
	require.NoError(t, err)

	dstCh := make(chan goflux.Message[Event], 1)

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return dstSub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Event]) error {
			dstCh <- msg

			return nil
		})
	}, gofuncy.WithName("dst-subscriber"))

	// Middleware that injects a message ID into context.
	var middlewareCalled atomic.Bool

	mw := func(next goflux.Handler[Event]) goflux.Handler[Event] {
		return func(ctx context.Context, msg goflux.Message[Event]) error {
			middlewareCalled.Store(true)

			ctx = goflux.WithMessageID(ctx, "mw-injected-id")

			return next(ctx, msg)
		}
	}

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return srcSub.Subscribe(ctx, "events", pipe.New[Event](dstPub, pipe.WithMiddleware(mw)))
	}, gofuncy.WithName("pipe"))

	require.NoError(t, srcPub.Publish(ctx, "events", Event{ID: "1", Name: "hello"}))

	select {
	case msg := <-dstCh:
		assert.Equal(t, "hello", msg.Payload.Name)
		assert.True(t, middlewareCalled.Load(), "middleware should have been called")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

// ---------------------------------------------------------------------------
// NewMap
// ---------------------------------------------------------------------------

func TestNewMap(t *testing.T) {
	ctx := t.Context()

	srcBus := channel.NewBus[Event]()
	srcPub := channel.NewPublisher(srcBus)
	srcSub, err := channel.NewSubscriber(srcBus, 1)
	require.NoError(t, err)

	dstBus := channel.NewBus[Summary]()
	dstPub := channel.NewPublisher(dstBus)
	dstSub, err := channel.NewSubscriber(dstBus, 1)
	require.NoError(t, err)

	dstCh := make(chan goflux.Message[Summary], 1)

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return dstSub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Summary]) error {
			dstCh <- msg

			return nil
		})
	}, gofuncy.WithName("dst-subscriber"))

	mapFn := func(_ context.Context, msg goflux.Message[Event]) (Summary, error) {
		return Summary{Label: msg.Payload.Name}, nil
	}

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return srcSub.Subscribe(ctx, "events", pipe.NewMap[Event, Summary](dstPub, mapFn))
	}, gofuncy.WithName("pipe-map"))

	require.NoError(t, srcPub.Publish(ctx, "events", Event{ID: "1", Name: "hello"}))

	select {
	case msg := <-dstCh:
		assert.Equal(t, Summary{Label: "hello"}, msg.Payload)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestNewMap_mapErrorReturnsToTransport(t *testing.T) {
	mapErr := errors.New("map failed")

	var dlCalled atomic.Bool

	deadLetter := func(_ context.Context, _ goflux.Message[Event], _ error) {
		dlCalled.Store(true)
	}

	mapFn := func(_ context.Context, _ goflux.Message[Event]) (Summary, error) {
		return Summary{}, mapErr
	}

	dstPub := &failPublisher[Summary]{} // won't be reached

	handler := pipe.NewMap[Event, Summary](dstPub, mapFn,
		pipe.WithMapDeadLetter[Event, Summary](deadLetter),
	)

	err := handler(context.Background(), goflux.NewMessage("events", Event{ID: "1", Name: "hello"}))

	require.Error(t, err)
	require.ErrorIs(t, err, mapErr)
	assert.True(t, dlCalled.Load())
}

// ---------------------------------------------------------------------------
// NewFlatMap
// ---------------------------------------------------------------------------

func TestNewFlatMap(t *testing.T) {
	ctx := t.Context()

	srcBus := channel.NewBus[Order]()
	srcPub := channel.NewPublisher(srcBus)
	srcSub, err := channel.NewSubscriber(srcBus, 1)
	require.NoError(t, err)

	dstBus := channel.NewBus[LineItem]()
	dstPub := channel.NewPublisher(dstBus)
	dstSub, err := channel.NewSubscriber(dstBus, 1)
	require.NoError(t, err)

	dstCh := make(chan goflux.Message[LineItem], 4)

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return dstSub.Subscribe(ctx, "orders", func(_ context.Context, msg goflux.Message[LineItem]) error {
			dstCh <- msg

			return nil
		})
	}, gofuncy.WithName("dst-subscriber"))

	flatMapFn := func(_ context.Context, msg goflux.Message[Order]) ([]LineItem, error) {
		items := make([]LineItem, len(msg.Payload.Items))
		for i, item := range msg.Payload.Items {
			items[i] = LineItem{OrderID: msg.Payload.ID, Item: item}
		}

		return items, nil
	}

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return srcSub.Subscribe(ctx, "orders", pipe.NewFlatMap[Order, LineItem](dstPub, flatMapFn))
	}, gofuncy.WithName("pipe-flatmap"))

	require.NoError(t, srcPub.Publish(ctx, "orders", Order{
		ID:    "order-1",
		Items: []string{"widget", "gadget"},
	}))

	var received []LineItem

	for range 2 {
		select {
		case msg := <-dstCh:
			received = append(received, msg.Payload)
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for message")
		}
	}

	assert.Len(t, received, 2)
	assert.Equal(t, "widget", received[0].Item)
	assert.Equal(t, "gadget", received[1].Item)
	assert.Equal(t, "order-1", received[0].OrderID)
}

func TestNewFlatMap_flatMapErrorReturnsToTransport(t *testing.T) {
	flatMapErr := errors.New("flatmap failed")

	var dlCalled atomic.Bool

	deadLetter := func(_ context.Context, _ goflux.Message[Order], _ error) {
		dlCalled.Store(true)
	}

	flatMapFn := func(_ context.Context, _ goflux.Message[Order]) ([]LineItem, error) {
		return nil, flatMapErr
	}

	dstPub := &failPublisher[LineItem]{}

	handler := pipe.NewFlatMap[Order, LineItem](dstPub, flatMapFn,
		pipe.WithMapDeadLetter[Order, LineItem](deadLetter),
	)

	err := handler(context.Background(), goflux.NewMessage("orders", Order{ID: "1"}))

	require.Error(t, err)
	require.ErrorIs(t, err, flatMapErr)
	assert.True(t, dlCalled.Load())
}

func TestNewFlatMap_publishErrorMidBatch(t *testing.T) {
	pub := &countingFailPublisher[LineItem]{
		failAfter: 1,
		err:       errors.New("publish failed on item 2"),
	}

	flatMapFn := func(_ context.Context, msg goflux.Message[Order]) ([]LineItem, error) {
		items := make([]LineItem, len(msg.Payload.Items))
		for i, item := range msg.Payload.Items {
			items[i] = LineItem{OrderID: msg.Payload.ID, Item: item}
		}

		return items, nil
	}

	handler := pipe.NewFlatMap[Order, LineItem](pub, flatMapFn)

	err := handler(context.Background(), goflux.NewMessage("orders", Order{
		ID:    "order-1",
		Items: []string{"widget", "gadget", "doohickey"},
	}))

	require.Error(t, err)
	// First item published successfully, second failed.
	assert.Equal(t, int32(2), pub.count.Load())
}

// ---------------------------------------------------------------------------
// Examples (kept for godoc)
// ---------------------------------------------------------------------------

func ExampleNew() {
	srcBus := channel.NewBus[Event]()
	srcPub := channel.NewPublisher(srcBus)

	srcSub, err := channel.NewSubscriber(srcBus, 1)
	if err != nil {
		panic(err)
	}

	dstBus := channel.NewBus[Event]()
	dstPub := channel.NewPublisher(dstBus)

	dstSub, err := channel.NewSubscriber(dstBus, 1)
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return dstSub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Event]) error {
			fmt.Println(msg.Subject, msg.Payload)
			cancel()

			return nil
		})
	}, gofuncy.WithName("dst-subscriber"))

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return srcSub.Subscribe(ctx, "events", pipe.New[Event](dstPub))
	}, gofuncy.WithName("pipe"))

	if err := srcPub.Publish(ctx, "events", Event{ID: "1", Name: "hello"}); err != nil {
		panic(err)
	}

	<-ctx.Done()
	// Output: events {1 hello}
}

func ExampleNewMap() {
	ctx, cancel := context.WithCancel(context.Background())

	srcBus := channel.NewBus[Event]()
	srcPub := channel.NewPublisher(srcBus)

	srcSub, err := channel.NewSubscriber(srcBus, 1)
	if err != nil {
		panic(err)
	}

	dstBus := channel.NewBus[Summary]()
	dstPub := channel.NewPublisher(dstBus)

	dstSub, err := channel.NewSubscriber(dstBus, 1)
	if err != nil {
		panic(err)
	}

	mapFn := func(_ context.Context, msg goflux.Message[Event]) (Summary, error) {
		return Summary{Label: msg.Payload.Name}, nil
	}

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return dstSub.Subscribe(ctx, "events", func(_ context.Context, msg goflux.Message[Summary]) error {
			fmt.Println(msg.Subject, msg.Payload)
			cancel()

			return nil
		})
	}, gofuncy.WithName("dst-subscriber"))

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return srcSub.Subscribe(ctx, "events", pipe.NewMap[Event, Summary](dstPub, mapFn))
	}, gofuncy.WithName("pipe-map"))

	if err := srcPub.Publish(ctx, "events", Event{ID: "1", Name: "hello"}); err != nil {
		panic(err)
	}

	<-ctx.Done()
	// Output: events {hello}
}

func ExampleNewFlatMap() {
	ctx, cancel := context.WithCancel(context.Background())

	srcBus := channel.NewBus[Order]()
	srcPub := channel.NewPublisher(srcBus)

	srcSub, err := channel.NewSubscriber(srcBus, 1)
	if err != nil {
		panic(err)
	}

	dstBus := channel.NewBus[LineItem]()
	dstPub := channel.NewPublisher(dstBus)

	dstSub, err := channel.NewSubscriber(dstBus, 1)
	if err != nil {
		panic(err)
	}

	flatMapFn := func(_ context.Context, msg goflux.Message[Order]) ([]LineItem, error) {
		items := make([]LineItem, len(msg.Payload.Items))
		for i, item := range msg.Payload.Items {
			items[i] = LineItem{OrderID: msg.Payload.ID, Item: item}
		}

		return items, nil
	}

	var count int

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return dstSub.Subscribe(ctx, "orders", func(_ context.Context, msg goflux.Message[LineItem]) error {
			fmt.Println(msg.Payload.OrderID, msg.Payload.Item)

			count++
			if count == 2 {
				cancel()
			}

			return nil
		})
	}, gofuncy.WithName("dst-subscriber"))

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		ready()

		return srcSub.Subscribe(ctx, "orders", pipe.NewFlatMap[Order, LineItem](dstPub, flatMapFn))
	}, gofuncy.WithName("pipe-flatmap"))

	if err := srcPub.Publish(ctx, "orders", Order{ID: "o1", Items: []string{"widget", "gadget"}}); err != nil {
		panic(err)
	}

	<-ctx.Done()
	// Output:
	// o1 widget
	// o1 gadget
}
