# Pipeline Operators

Pipeline operators wire subscribers to publishers, composing `Handler[T]` and `Publisher[T]` into message-processing topologies. They handle filtering, transformation, fan-out, fan-in, and round-robin distribution.

## Pipe

```go
func Pipe[T any](pub Publisher[T], opts ...PipeOption[T]) Handler[T]
```

Returns a `Handler[T]` that forwards every accepted message to `pub`, preserving the original subject. Filters run first; a dropped message never reaches the publisher. Publish errors are returned to the subscriber as-is.

```go
// Forward all order events from sub to pub
err := sub.Subscribe(ctx, "orders.>", goflux.Pipe[OrderEvent](pub))
```

## PipeMap

```go
func PipeMap[T, U any](pub Publisher[U], mapFn MapFunc[T, U], opts ...PipeOption[T]) Handler[T]
```

Type-transforming pipe. Maps each `Message[T]` to a `Message[U]` before publishing. Filters run on `T` before the map. A map error routes the original `Message[T]` to the dead-letter handler (if set) and drops the message -- map errors are non-fatal to the subscriber.

```go
mapFn := func(ctx context.Context, msg goflux.Message[OrderEvent]) (goflux.Message[Invoice], error) {
    inv := Invoice{OrderID: msg.Payload.ID, Amount: msg.Payload.Total}
    return goflux.NewMessage(msg.Subject, inv), nil
}

err := sub.Subscribe(ctx, "orders.created", goflux.PipeMap[OrderEvent, Invoice](
    invoicePub,
    mapFn,
))
```

## Options

### WithFilter

```go
func WithFilter[T any](f Filter[T]) PipeOption[T]
```

Registers a filter that runs before publish (or before map in `PipeMap`). Messages for which the filter returns `false` or an error are silently dropped and logged.

```go
type Filter[T any] func(ctx context.Context, msg Message[T]) (bool, error)
```

Multiple filters can be stacked -- they evaluate in order and short-circuit on the first `false` or error.

### WithDeadLetter

```go
func WithDeadLetter[T any](fn DeadLetterFunc[T]) PipeOption[T]
```

Registers a dead-letter handler called when `MapFunc` returns an error or when the publisher fails. The original `Message[T]` and the terminal error are passed to the handler.

```go
type DeadLetterFunc[T any] func(ctx context.Context, msg Message[T], err error)
```

### Combined Example

```go
handler := goflux.PipeMap[RawEvent, CleanEvent](
    cleanPub,
    transformFn,
    goflux.WithFilter[RawEvent](func(ctx context.Context, msg goflux.Message[RawEvent]) (bool, error) {
        return msg.Payload.Valid, nil
    }),
    goflux.WithDeadLetter[RawEvent](func(ctx context.Context, msg goflux.Message[RawEvent], err error) {
        slog.ErrorContext(ctx, "dead letter",
            slog.String("subject", msg.Subject),
            slog.Any("error", err),
        )
    }),
)

err := sub.Subscribe(ctx, "events.raw", handler)
```

## FanOut

```go
func FanOut[T any](publishers []Publisher[T], opts ...FanOutOption[T]) Publisher[T]
```

Returns a `Publisher[T]` that broadcasts every `Publish` call to all inner publishers. By default, errors from individual publishers are joined via `errors.Join` (best-effort). With `WithFanOutAllOrNothing`, the first error short-circuits.

```go
broadcast := goflux.FanOut[Event]([]goflux.Publisher[Event]{pubA, pubB, pubC})

err := broadcast.Publish(ctx, "events.order", event)
```

With all-or-nothing semantics:

```go
broadcast := goflux.FanOut[Event](
    []goflux.Publisher[Event]{pubA, pubB},
    goflux.WithFanOutAllOrNothing[Event](),
)
```

## FanIn

```go
func FanIn[T any](subscribers ...Subscriber[T]) Subscriber[T]
```

Returns a `Subscriber[T]` that subscribes to the same subject on all provided subscribers and dispatches every message to a single handler. `Subscribe` blocks until all inner subscriptions complete.

```go
merged := goflux.FanIn[Event](subNATS, subHTTP)

err := merged.Subscribe(ctx, "events.>", handler)
```

## RoundRobin

```go
func RoundRobin[T any](publishers ...Publisher[T]) Publisher[T]
```

Returns a `Publisher[T]` that distributes each `Publish` call to a single inner publisher, cycling through them via an atomic counter.

```go
loadBalanced := goflux.RoundRobin[Event](pub1, pub2, pub3)

// First call goes to pub1, second to pub2, third to pub3, fourth to pub1, ...
err := loadBalanced.Publish(ctx, "events.order", event)
```

## BoundPublisher

```go
func Bind[T any](pub Publisher[T], subject string) *BoundPublisher[T]
```

Wraps a `Publisher[T]` with a fixed subject. Useful when a component always publishes to the same destination and the subject should not leak into business logic.

```go
orderPub := goflux.Bind(pub, "orders.created")

// No subject argument needed
err := orderPub.Publish(ctx, event)
```

`BoundPublisher` exposes `Publish(ctx, v)` (no subject parameter) and delegates `Close()` to the underlying publisher.

## ToChan

```go
func ToChan[T any](ctx context.Context, sub Subscriber[T], subject string, bufSize int) <-chan T
```

Bridges a `Subscriber[T]` into a plain Go channel. Launches `Subscribe` in a goroutine and forwards each message payload into a buffered channel. The returned channel closes when `ctx` is cancelled.

```go
ch := goflux.ToChan(ctx, sub, "orders.>", 16)

for event := range ch {
    fmt.Println("order:", event.ID)
}
```

::: warning Ownership
`Close()` on `FanOut`, `FanIn`, and `RoundRobin` is a **no-op**. The caller owns the inner publishers and subscribers and is responsible for closing them.
:::
