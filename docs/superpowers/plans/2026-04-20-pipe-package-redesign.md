# Pipe Package Redesign Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Redesign the pipe package into a production-ready, option-driven handler factory with proper error propagation, OTel observability, and extract stream bridges into a separate `bridge/` module.

**Architecture:** Three handler factories (`New`, `NewMap`, `NewFlatMap`) return `goflux.Handler[T]`. Middleware chain wraps the pipe's internal handler. Filter rejection returns nil (ack). Map/publish errors return to transport. Span events/attributes added to existing transport span. Stream bridges extracted to `bridge/` submodule with own `go.mod`. `ToChan` moves to root package.

**Tech Stack:** Go 1.26, goflux core interfaces, OTel trace API (`go.opentelemetry.io/otel/trace`), goflow (bridge only), gofuncy, testify

---

### Task 1: Rewrite pipe/option.go — new types and options

**Files:**
- Modify: `pipe/option.go` (complete rewrite)

- [ ] **Step 1: Write the new option.go**

Replace the entire file. Key changes: `Filter[T]` signature changes from `(bool, error)` to `bool`. `MapFunc` returns `(U, error)` not `(Message[U], error)`. Add `FlatMapFunc`, `MapOption`, `WithMiddleware`, and separate map option functions.

```go
package pipe

import (
	"context"

	"github.com/foomo/goflux"
)

// Filter decides whether a message should be forwarded.
// Returning false skips the message (returns nil to transport = ack).
type Filter[T any] func(ctx context.Context, msg goflux.Message[T]) bool

// MapFunc transforms a Message[T] payload into a U value.
// A non-nil error is returned to the transport and routes to the dead-letter observer.
type MapFunc[T, U any] func(ctx context.Context, msg goflux.Message[T]) (U, error)

// FlatMapFunc expands a Message[T] into zero or more U values.
// A non-nil error is returned to the transport and routes to the dead-letter observer.
type FlatMapFunc[T, U any] func(ctx context.Context, msg goflux.Message[T]) ([]U, error)

// DeadLetterFunc is an observer called when a map/flatmap/publish operation fails.
// It receives the original message and the error for logging/alerting.
// It does NOT swallow the error — the error is still returned to the transport.
type DeadLetterFunc[T any] func(ctx context.Context, msg goflux.Message[T], err error)

// ---------------------------------------------------------------------------
// Option[T] — for New[T]
// ---------------------------------------------------------------------------

// Option configures a [New] pipe.
type Option[T any] func(*config[T])

type config[T any] struct {
	filter     Filter[T]
	deadLetter DeadLetterFunc[T]
	middleware []goflux.Middleware[T]
}

// WithFilter sets a filter that runs before publish.
// Messages for which the filter returns false are skipped (handler returns nil).
func WithFilter[T any](f Filter[T]) Option[T] {
	return func(c *config[T]) { c.filter = f }
}

// WithDeadLetter sets an observer called when publish fails.
// The observer receives the original message and the error.
func WithDeadLetter[T any](fn DeadLetterFunc[T]) Option[T] {
	return func(c *config[T]) { c.deadLetter = fn }
}

// WithMiddleware registers middleware that wraps the pipe's internal handler.
// Middleware runs before filter/map/publish — it sees the original message and
// can enrich the context that flows into subsequent stages.
func WithMiddleware[T any](mw ...goflux.Middleware[T]) Option[T] {
	return func(c *config[T]) { c.middleware = append(c.middleware, mw...) }
}

func newConfig[T any](opts []Option[T]) *config[T] {
	cfg := &config[T]{}
	for _, o := range opts {
		o(cfg)
	}

	return cfg
}

// ---------------------------------------------------------------------------
// MapOption[T, U] — for NewMap[T, U] and NewFlatMap[T, U]
// ---------------------------------------------------------------------------

// MapOption configures a [NewMap] or [NewFlatMap] pipe.
type MapOption[T, U any] func(*mapConfig[T, U])

type mapConfig[T, U any] struct {
	filter     Filter[T]
	deadLetter DeadLetterFunc[T]
	middleware []goflux.Middleware[T]
}

// WithMapFilter sets a filter that runs before map/flatmap.
func WithMapFilter[T, U any](f Filter[T]) MapOption[T, U] {
	return func(c *mapConfig[T, U]) { c.filter = f }
}

// WithMapDeadLetter sets an observer called when map/flatmap or publish fails.
func WithMapDeadLetter[T, U any](fn DeadLetterFunc[T]) MapOption[T, U] {
	return func(c *mapConfig[T, U]) { c.deadLetter = fn }
}

// WithMapMiddleware registers middleware for a map/flatmap pipe.
func WithMapMiddleware[T, U any](mw ...goflux.Middleware[T]) MapOption[T, U] {
	return func(c *mapConfig[T, U]) { c.middleware = append(c.middleware, mw...) }
}

func newMapConfig[T, U any](opts []MapOption[T, U]) *mapConfig[T, U] {
	cfg := &mapConfig[T, U]{}
	for _, o := range opts {
		o(cfg)
	}

	return cfg
}
```

- [ ] **Step 2: Verify it compiles**

Run: `cd /Users/franklin/Workingcopies/github.com/foomo/goflux && go build ./pipe/...`
Expected: compilation errors from pipe.go (references old types). That's fine — we rewrite pipe.go next.

- [ ] **Step 3: Commit**

```bash
git add pipe/option.go
git commit -m "refactor(pipe): rewrite option types for redesigned API"
```

---

### Task 2: Rewrite pipe/pipe.go — New, NewMap, NewFlatMap with OTel

**Files:**
- Modify: `pipe/pipe.go` (complete rewrite)

- [ ] **Step 1: Write the new pipe.go**

Three handler factories. Middleware wraps the internal handler. Filter returns nil on rejection. Map/publish errors bubble up. Span events and attributes added to existing span.

```go
package pipe

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/foomo/goflux"
)

// New returns a [goflux.Handler] that forwards every accepted message to pub.
// The handler:
//  1. Runs the middleware chain (if any)
//  2. Applies the filter (if set) — false means skip (return nil)
//  3. Forwards msg.Header into the publish context via [goflux.WithHeader]
//  4. Publishes msg.Payload to msg.Subject
//
// Publish errors are returned to the caller (transport decides retry/nak).
// The dead-letter observer is called on publish error for logging/alerting.
func New[T any](pub goflux.Publisher[T], opts ...Option[T]) goflux.Handler[T] {
	cfg := newConfig(opts)

	handler := func(ctx context.Context, msg goflux.Message[T]) error {
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(attribute.String("pipe.type", "forward"))

		if cfg.filter != nil && !cfg.filter(ctx, msg) {
			span.SetAttributes(attribute.Bool("pipe.filtered", true))

			return nil
		}

		ctx = goflux.WithHeader(ctx, msg.Header)

		if err := pub.Publish(ctx, msg.Subject, msg.Payload); err != nil {
			observeDeadLetter(span, cfg.deadLetter, ctx, msg, err)
			span.AddEvent("pipe.publish_error", trace.WithAttributes(
				attribute.String("pipe.error", err.Error()),
			))

			return err
		}

		return nil
	}

	return applyMiddleware(handler, cfg.middleware)
}

// NewMap returns a [goflux.Handler] that transforms each message from T to U
// before publishing. The filter runs on the original Message[T] before the map.
//
// Map errors and publish errors are returned to the caller.
// The dead-letter observer is called on either failure.
func NewMap[T, U any](pub goflux.Publisher[U], mapFn MapFunc[T, U], opts ...MapOption[T, U]) goflux.Handler[T] {
	cfg := newMapConfig(opts)

	handler := func(ctx context.Context, msg goflux.Message[T]) error {
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(attribute.String("pipe.type", "map"))

		if cfg.filter != nil && !cfg.filter(ctx, msg) {
			span.SetAttributes(attribute.Bool("pipe.filtered", true))

			return nil
		}

		mapped, err := mapFn(ctx, msg)
		if err != nil {
			observeDeadLetter(span, cfg.deadLetter, ctx, msg, err)
			span.AddEvent("pipe.map_error", trace.WithAttributes(
				attribute.String("pipe.error", err.Error()),
			))

			return fmt.Errorf("pipe map: %w", err)
		}

		ctx = goflux.WithHeader(ctx, msg.Header)

		if err := pub.Publish(ctx, msg.Subject, mapped); err != nil {
			observeDeadLetter(span, cfg.deadLetter, ctx, msg, err)
			span.AddEvent("pipe.publish_error", trace.WithAttributes(
				attribute.String("pipe.error", err.Error()),
			))

			return err
		}

		return nil
	}

	return applyMiddleware(handler, cfg.middleware)
}

// NewFlatMap returns a [goflux.Handler] that expands each message from T into
// zero or more U values, publishing each one individually.
//
// If the FlatMapFunc fails, the error is returned immediately.
// If a publish fails mid-batch, items already published are NOT rolled back —
// downstream consumers must be idempotent or deduplicate.
func NewFlatMap[T, U any](pub goflux.Publisher[U], fn FlatMapFunc[T, U], opts ...MapOption[T, U]) goflux.Handler[T] {
	cfg := newMapConfig(opts)

	handler := func(ctx context.Context, msg goflux.Message[T]) error {
		span := trace.SpanFromContext(ctx)
		span.SetAttributes(attribute.String("pipe.type", "flatmap"))

		if cfg.filter != nil && !cfg.filter(ctx, msg) {
			span.SetAttributes(attribute.Bool("pipe.filtered", true))

			return nil
		}

		items, err := fn(ctx, msg)
		if err != nil {
			observeDeadLetter(span, cfg.deadLetter, ctx, msg, err)
			span.AddEvent("pipe.map_error", trace.WithAttributes(
				attribute.String("pipe.error", err.Error()),
			))

			return fmt.Errorf("pipe flatmap: %w", err)
		}

		ctx = goflux.WithHeader(ctx, msg.Header)

		for _, item := range items {
			if err := pub.Publish(ctx, msg.Subject, item); err != nil {
				observeDeadLetter(span, cfg.deadLetter, ctx, msg, err)
				span.AddEvent("pipe.publish_error", trace.WithAttributes(
					attribute.String("pipe.error", err.Error()),
				))

				return err
			}
		}

		span.SetAttributes(attribute.Int("pipe.items_published", len(items)))

		return nil
	}

	return applyMiddleware(handler, cfg.middleware)
}

// applyMiddleware wraps handler with the middleware chain (if any).
func applyMiddleware[T any](handler goflux.Handler[T], mws []goflux.Middleware[T]) goflux.Handler[T] {
	if len(mws) == 0 {
		return handler
	}

	return goflux.Chain(mws...)(handler)
}

// observeDeadLetter calls the dead-letter observer (if set) and adds a span event.
func observeDeadLetter[T any](span trace.Span, dl DeadLetterFunc[T], ctx context.Context, msg goflux.Message[T], err error) {
	if dl != nil {
		dl(ctx, msg, err)
	}

	span.AddEvent("pipe.dead_letter", trace.WithAttributes(
		attribute.String("pipe.error", err.Error()),
	))
}
```

- [ ] **Step 2: Update go.mod for OTel trace dependency**

Run: `cd /Users/franklin/Workingcopies/github.com/foomo/goflux && go mod tidy`

OTel trace is already a dependency in root go.mod. This should be a no-op but verify pipe compiles.

- [ ] **Step 3: Verify pipe package compiles (ignoring test errors)**

Run: `cd /Users/franklin/Workingcopies/github.com/foomo/goflux && go build ./pipe/...`
Expected: may fail on tostream.go/fromstream.go/tochan.go references to old types. That's expected — those files move in Task 5/6.

- [ ] **Step 4: Commit**

```bash
git add pipe/pipe.go
git commit -m "refactor(pipe): rewrite handler factories with OTel and error propagation"
```

---

### Task 3: Rewrite pipe tests — New, NewMap, NewFlatMap

**Files:**
- Modify: `pipe/pipe_test.go` (complete rewrite)

- [ ] **Step 1: Write failing tests for New**

Tests: basic forward, with filter (rejected returns nil), with dead-letter observer on publish error, with middleware.

```go
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

	var dlCalled atomic.Bool
	var dlMsg goflux.Message[Event]
	var dlErr error

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
	var middlewareSawCtx atomic.Bool
	mw := func(next goflux.Handler[Event]) goflux.Handler[Event] {
		return func(ctx context.Context, msg goflux.Message[Event]) error {
			middlewareSawCtx.Store(true)
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
		assert.True(t, middlewareSawCtx.Load(), "middleware should have been called")
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
	assert.ErrorIs(t, err, mapErr)
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

		return dstSub.Subscribe(ctx, "items", func(_ context.Context, msg goflux.Message[LineItem]) error {
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
	assert.ErrorIs(t, err, flatMapErr)
	assert.True(t, dlCalled.Load())
}

func TestNewFlatMap_publishErrorMidBatch(t *testing.T) {
	var publishCount atomic.Int32

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
```

- [ ] **Step 2: Run tests to verify they pass**

Run: `cd /Users/franklin/Workingcopies/github.com/foomo/goflux && go test -tags=safe -v -run "TestNew|TestNewMap|TestNewFlatMap|ExampleNew" ./pipe/...`
Expected: all pass

- [ ] **Step 3: Commit**

```bash
git add pipe/pipe_test.go
git commit -m "test(pipe): rewrite tests for New, NewMap, NewFlatMap"
```

---

### Task 4: Move ToChan to root package

**Files:**
- Create: `tochan.go` (root package)
- Create: `tochan_test.go` (root package)
- Delete: `pipe/tochan.go`
- Delete: `pipe/tochan_test.go`

- [ ] **Step 1: Create root tochan.go**

```go
package goflux

import (
	"context"

	"github.com/foomo/gofuncy"
)

// ToChan bridges a [Subscriber] into a plain channel. It launches Subscribe in
// a goroutine and forwards each message (including acker) into a buffered
// channel. The returned channel closes when ctx is cancelled.
//
// bufSize controls backpressure: a full buffer blocks the subscriber's handler
// until the consumer catches up.
func ToChan[T any](ctx context.Context, sub Subscriber[T], subject string, bufSize int) <-chan Message[T] {
	ch := make(chan Message[T], bufSize)

	gofuncy.Go(ctx, func(ctx context.Context) error {
		defer close(ch)

		return sub.Subscribe(ctx, subject, func(ctx context.Context, msg Message[T]) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case ch <- msg:
				return nil
			}
		})
	}, gofuncy.WithName("goflux.tochan"))

	return ch
}
```

- [ ] **Step 2: Create root tochan_test.go**

```go
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

	sub, err := channel.NewSubscriber(bus, 1)
	require.NoError(t, err)

	ch := goflux.ToChan[string](ctx, sub, "test", 1)

	cancel()

	// Channel should close after context cancellation.
	select {
	case _, ok := <-ch:
		assert.False(t, ok, "channel should be closed")
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for channel to close")
	}
}
```

- [ ] **Step 3: Delete pipe/tochan.go and pipe/tochan_test.go**

```bash
rm pipe/tochan.go pipe/tochan_test.go
```

- [ ] **Step 4: Run tests**

Run: `cd /Users/franklin/Workingcopies/github.com/foomo/goflux && go test -tags=safe -v -run TestToChan ./...`
Expected: root tests pass

- [ ] **Step 5: Commit**

```bash
git add tochan.go tochan_test.go
git add pipe/tochan.go pipe/tochan_test.go
git commit -m "refactor: move ToChan from pipe to root package"
```

---

### Task 5: Create bridge/ submodule with ToStream and FromStream

**Files:**
- Create: `bridge/go.mod`
- Create: `bridge/tostream.go`
- Create: `bridge/fromstream.go`
- Create: `bridge/tostream_test.go`
- Create: `bridge/fromstream_test.go`
- Delete: `pipe/tostream.go`, `pipe/tostream_test.go`
- Delete: `pipe/fromstream.go`, `pipe/fromstream_test.go`
- Modify: `go.work` (add `./bridge`)

- [ ] **Step 1: Create bridge/go.mod**

```
module github.com/foomo/goflux/bridge

go 1.26.0

require (
	github.com/foomo/goflow v0.2.2
	github.com/foomo/goflux v0.6.0
	github.com/foomo/gofuncy v0.2.0
	github.com/stretchr/testify v1.11.1
)
```

Then run: `cd /Users/franklin/Workingcopies/github.com/foomo/goflux/bridge && go mod tidy`

- [ ] **Step 2: Create bridge/tostream.go**

```go
package bridge

import (
	"context"

	"github.com/foomo/goflow"
	"github.com/foomo/goflux"
)

// ToStream bridges a [goflux.Subscriber] into a [goflow.Stream]. It launches
// Subscribe in a goroutine via [goflux.ToChan] and wraps the resulting channel
// as a Stream.
func ToStream[T any](ctx context.Context, sub goflux.Subscriber[T], subject string, bufSize int) goflow.Stream[goflux.Message[T]] {
	return goflow.From(ctx, goflux.ToChan(ctx, sub, subject, bufSize))
}
```

- [ ] **Step 3: Create bridge/fromstream.go**

```go
package bridge

import (
	"context"

	"github.com/foomo/goflow"
	"github.com/foomo/goflux"
)

// FromStream consumes a [goflow.Stream] of messages and publishes each one via
// the provided [goflux.Publisher]. The original message nats is used for
// publishing.
func FromStream[T any](stream goflow.Stream[goflux.Message[T]], pub goflux.Publisher[T]) error {
	return stream.ForEach(func(ctx context.Context, msg goflux.Message[T]) error {
		return pub.Publish(ctx, msg.Subject, msg.Payload)
	})
}
```

- [ ] **Step 4: Create bridge/tostream_test.go**

```go
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
```

- [ ] **Step 5: Create bridge/fromstream_test.go**

```go
package bridge_test

import (
	"testing"
	"time"

	"github.com/foomo/goflow"
	"github.com/foomo/goflux"
	"github.com/foomo/goflux/bridge"
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

	dstCh := goflux.ToChan[Event](ctx, dstSub, "events", 4)

	time.Sleep(50 * time.Millisecond)

	msgs := []goflux.Message[Event]{
		goflux.NewMessage("events", Event{ID: "1", Name: "alpha"}),
		goflux.NewMessage("events", Event{ID: "2", Name: "bravo"}),
	}
	stream := goflow.Of(ctx, msgs...)

	err = bridge.FromStream(stream, dstPub)
	require.NoError(t, err)

	msg1 := <-dstCh
	msg2 := <-dstCh

	assert.Equal(t, "alpha", msg1.Payload.Name)
	assert.Equal(t, "bravo", msg2.Payload.Name)
}
```

- [ ] **Step 6: Delete old pipe stream files**

```bash
rm pipe/tostream.go pipe/tostream_test.go pipe/fromstream.go pipe/fromstream_test.go
```

- [ ] **Step 7: Add bridge to go.work**

Add `./bridge` to the `use` block in `go.work`. Also add replace directives for local dependencies.

- [ ] **Step 8: Tidy and test**

Run:
```bash
cd /Users/franklin/Workingcopies/github.com/foomo/goflux && go work sync
cd bridge && go mod tidy
cd .. && go test -tags=safe -v ./bridge/...
```
Expected: all bridge tests pass

- [ ] **Step 9: Commit**

```bash
git add bridge/ go.work
git add pipe/tostream.go pipe/tostream_test.go pipe/fromstream.go pipe/fromstream_test.go
git commit -m "refactor: extract stream bridges to bridge/ submodule"
```

---

### Task 6: Add middleware.ForwardMessageID

**Files:**
- Modify: `middleware/injectheader.go` (add ForwardMessageID to existing file)
- Create: `middleware/forwardmessageid_test.go`

- [ ] **Step 1: Write the failing test**

```go
package middleware_test

import (
	"context"
	"testing"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/middleware"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestForwardMessageID(t *testing.T) {
	var capturedCtx context.Context

	inner := func(ctx context.Context, msg goflux.Message[string]) error {
		capturedCtx = ctx

		return nil
	}

	handler := middleware.ForwardMessageID[string]()(inner)

	// Set message ID in incoming context.
	ctx := goflux.WithMessageID(context.Background(), "msg-123")
	msg := goflux.NewMessage("test", "payload")

	err := handler(ctx, msg)
	require.NoError(t, err)

	// Message ID should be forwarded through context.
	assert.Equal(t, "msg-123", goflux.MessageID(capturedCtx))
}

func TestForwardMessageID_noID(t *testing.T) {
	var capturedCtx context.Context

	inner := func(ctx context.Context, msg goflux.Message[string]) error {
		capturedCtx = ctx

		return nil
	}

	handler := middleware.ForwardMessageID[string]()(inner)

	err := handler(context.Background(), goflux.NewMessage("test", "payload"))
	require.NoError(t, err)

	// No message ID set — should remain empty.
	assert.Empty(t, goflux.MessageID(capturedCtx))
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd /Users/franklin/Workingcopies/github.com/foomo/goflux && go test -tags=safe -v -run TestForwardMessageID ./middleware/...`
Expected: FAIL — `ForwardMessageID` not defined

- [ ] **Step 3: Add ForwardMessageID to middleware/injectheader.go**

Append to the end of `middleware/injectheader.go`:

```go
// ForwardMessageID returns a [goflux.Middleware] that preserves the message ID
// from the incoming context through the handler chain. Use this with pipe's
// [pipe.WithMiddleware] to forward message IDs across pipe stages.
//
// Unlike [InjectMessageID] (which reads the ID from message headers set by
// transports), ForwardMessageID reads from context — suitable for in-process
// handler chains where the ID is already in context.
func ForwardMessageID[T any]() goflux.Middleware[T] {
	return func(next goflux.Handler[T]) goflux.Handler[T] {
		return func(ctx context.Context, msg goflux.Message[T]) error {
			if id := goflux.MessageID(ctx); id != "" {
				ctx = goflux.WithMessageID(ctx, id)
			}

			return next(ctx, msg)
		}
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd /Users/franklin/Workingcopies/github.com/foomo/goflux && go test -tags=safe -v -run TestForwardMessageID ./middleware/...`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add middleware/injectheader.go middleware/forwardmessageid_test.go
git commit -m "feat(middleware): add ForwardMessageID for pipe context propagation"
```

---

### Task 7: Remove goflow dependency from root go.mod

**Files:**
- Modify: `go.mod` (root)

- [ ] **Step 1: Remove goflow from root module**

After moving ToStream/FromStream to bridge/ (which has own go.mod), root no longer imports goflow. Run:

```bash
cd /Users/franklin/Workingcopies/github.com/foomo/goflux && go mod tidy
```

- [ ] **Step 2: Verify root builds and tests pass**

Run: `cd /Users/franklin/Workingcopies/github.com/foomo/goflux && go test -tags=safe ./...`
Expected: all pass, no goflow import in root

- [ ] **Step 3: Commit**

```bash
git add go.mod go.sum
git commit -m "chore: remove goflow dependency from root module"
```

---

### Task 8: Full test suite and lint

**Files:** None — verification only

- [ ] **Step 1: Run full test suite**

Run: `cd /Users/franklin/Workingcopies/github.com/foomo/goflux && make test`
Expected: all pass

- [ ] **Step 2: Run linter**

Run: `cd /Users/franklin/Workingcopies/github.com/foomo/goflux && make lint.fix`
Expected: clean or auto-fixed

- [ ] **Step 3: Fix any issues and commit if needed**

```bash
git add -u
git commit -m "chore: fix lint issues after pipe package redesign"
```

---

### Task 9: Update CLAUDE.md

**Files:**
- Modify: `CLAUDE.md`

- [ ] **Step 1: Update Pipeline Operators section**

Update the "Pipeline Operators (root package)" section in CLAUDE.md to reflect new structure:

Replace the pipeline operators section with:

```markdown
### Pipeline Operators (`pipe/` subpackage)

Handler factories that compose `Handler[T]` and `Publisher[T]`:

- `pipe.New[T]` — forward messages through optional filter to a publisher
- `pipe.NewMap[T, U]` — transform T→U before publishing
- `pipe.NewFlatMap[T, U]` — expand T into []U, publish each individually
- Options: `WithFilter`, `WithDeadLetter`, `WithMiddleware` (and `WithMapFilter`, `WithMapDeadLetter`, `WithMapMiddleware` for map variants)
- Error semantics: filter rejection returns nil (ack), map/publish errors return to transport
- OTel: span events (`pipe.dead_letter`, `pipe.publish_error`, `pipe.map_error`) and attributes (`pipe.type`, `pipe.filtered`, `pipe.items_published`) on existing transport span

### Adapters

- `ToChan[T]` (root package) — bridge subscriber to `<-chan Message[T]`
- `bridge.ToStream[T]` / `bridge.FromStream[T]` (`bridge/` submodule) — bridge to/from `goflow.Stream[Message[T]]`
- `Bind[T]` — fixed-subject publisher wrapper
```

- [ ] **Step 2: Update submodules table if bridge/ is listed**

Add bridge to any module listing:

```markdown
| `bridge/` | goflow stream | Own `go.mod`, isolates goflow dependency |
```

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md
git commit -m "docs: update CLAUDE.md for pipe package redesign"
```
