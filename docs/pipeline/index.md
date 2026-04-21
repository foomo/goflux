# Pipeline Operators

Pipeline operators wire subscribers to publishers, composing `Handler[T]` and `Publisher[T]` into message-processing topologies. They handle filtering, transformation, and bridging to [goflow](https://github.com/foomo/goflow) streams.

::: tip Stream Processing
For fan-out, fan-in, round-robin, and other stream-processing patterns use [goflow](https://github.com/foomo/goflow) operators via the `bridge.ToStream` / `bridge.FromStream` functions in the `bridge/` submodule.
:::

## pipe.New

```go
import "github.com/foomo/goflux/pipe"

func New[T any](pub Publisher[T], opts ...Option[T]) Handler[T]
```

Returns a `Handler[T]` that forwards every accepted message to `pub`, preserving the original subject. The middleware chain runs first, then the filter; a filtered message returns `nil` to the transport (ack). Publish errors are returned to the subscriber as-is. The dead-letter observer is called on publish error for logging/alerting.

```go
// Forward all order events from sub to pub
err := sub.Subscribe(ctx, "orders.>", pipe.New[OrderEvent](pub))
```

## pipe.NewMap

```go
func NewMap[T, U any](pub Publisher[U], mapFn MapFunc[T, U], opts ...MapOption[T, U]) Handler[T]
```

Type-transforming pipe. Maps each `Message[T]` payload to a `U` value before publishing. The filter runs on `T` before the map. Map errors and publish errors are returned to the transport. The dead-letter observer is called on either failure.

```go
mapFn := func(ctx context.Context, msg goflux.Message[OrderEvent]) (Invoice, error) {
    return Invoice{OrderID: msg.Payload.ID, Amount: msg.Payload.Total}, nil
}

err := sub.Subscribe(ctx, "orders.created", pipe.NewMap[OrderEvent, Invoice](
    invoicePub,
    mapFn,
))
```

## pipe.NewFlatMap

```go
func NewFlatMap[T, U any](pub Publisher[U], fn FlatMapFunc[T, U], opts ...MapOption[T, U]) Handler[T]
```

Expands each `Message[T]` into zero or more `U` values, publishing each individually. If a publish fails mid-batch, items already published are NOT rolled back -- downstream consumers must be idempotent or deduplicate.

```go
flatMapFn := func(ctx context.Context, msg goflux.Message[Order]) ([]LineItem, error) {
    items := make([]LineItem, len(msg.Payload.Items))
    for i, item := range msg.Payload.Items {
        items[i] = LineItem{OrderID: msg.Payload.ID, Item: item}
    }
    return items, nil
}

err := sub.Subscribe(ctx, "orders", pipe.NewFlatMap[Order, LineItem](itemPub, flatMapFn))
```

## Options

### Option[T] (for pipe.New)

#### WithFilter

```go
func WithFilter[T any](f Filter[T]) Option[T]
```

Sets a filter that runs before publish. Messages for which the filter returns `false` are skipped (handler returns `nil` to the transport).

```go
type Filter[T any] func(ctx context.Context, msg Message[T]) bool
```

#### WithDeadLetter

```go
func WithDeadLetter[T any](fn DeadLetterFunc[T]) Option[T]
```

Sets an observer called when publish fails. The observer receives the original message and the error. It does NOT swallow the error -- the error is still returned to the transport.

```go
type DeadLetterFunc[T any] func(ctx context.Context, msg Message[T], err error)
```

#### WithMiddleware

```go
func WithMiddleware[T any](mw ...Middleware[T]) Option[T]
```

Registers middleware that wraps the pipe's internal handler. Middleware runs before filter/map/publish -- it sees the original message and can enrich the context that flows into subsequent stages.

### MapOption[T, U] (for pipe.NewMap, pipe.NewFlatMap)

- `WithMapFilter[T, U](f)` -- filter before map/flatmap
- `WithMapDeadLetter[T, U](fn)` -- observer on map/flatmap or publish failure
- `WithMapMiddleware[T, U](mw...)` -- middleware for map/flatmap pipes

### Combined Example

```go
handler := pipe.NewMap[RawEvent, CleanEvent](
    cleanPub,
    transformFn,
    pipe.WithMapFilter[RawEvent, CleanEvent](func(ctx context.Context, msg goflux.Message[RawEvent]) bool {
        return msg.Payload.Valid
    }),
    pipe.WithMapDeadLetter[RawEvent, CleanEvent](func(ctx context.Context, msg goflux.Message[RawEvent], err error) {
        slog.ErrorContext(ctx, "dead letter",
            slog.String("subject", msg.Subject),
            slog.Any("error", err),
        )
    }),
    pipe.WithMapMiddleware[RawEvent, CleanEvent](
        middleware.ForwardMessageID[RawEvent](),
    ),
)

err := sub.Subscribe(ctx, "events.raw", handler)
```

## Observability

Pipe adds span events and attributes to the existing transport span. No child spans are created.

### Span Attributes

| Attribute | Value | When |
|-----------|-------|------|
| `pipe.type` | `"forward"` / `"map"` / `"flatmap"` | Always |
| `pipe.filtered` | `true` | Filter rejected message |
| `pipe.items_published` | `int` | FlatMap only |

### Span Events

| Event | Attributes | When |
|-------|-----------|------|
| `pipe.dead_letter` | `pipe.error` | Dead-letter observer called |
| `pipe.publish_error` | `pipe.error` | Publish failed |
| `pipe.map_error` | `pipe.error` | Map/FlatMap func failed |

## Error Semantics

| Stage | On failure | Return value |
|-------|-----------|--------------|
| Filter | Rejects message | `nil` -- intentional skip, transport acks |
| MapFunc | Transform fails | `error` -- transport decides retry/nak. Dead-letter observer called. |
| FlatMapFunc | Transform fails | `error` -- transport decides retry/nak. Dead-letter observer called. |
| Publish | Publish fails | `error` -- transport decides retry/nak. Dead-letter observer called. |

## bridge.ToStream

```go
import "github.com/foomo/goflux/bridge"

func ToStream[T any](ctx context.Context, sub Subscriber[T], subject string, bufSize int) goflow.Stream[Message[T]]
```

Bridges a `Subscriber[T]` into a `goflow.Stream`. All [goflow](https://github.com/foomo/goflow) operators (Filter, Map, Distinct, FanOut, etc.) can be applied to the returned stream. Lives in the `bridge/` submodule (own `go.mod`) to isolate the goflow dependency.

```go
stream := bridge.ToStream[Event](ctx, sub, "orders.>", 16)

stream.
    Filter(func(ctx context.Context, msg goflux.Message[Event]) bool {
        return msg.Payload.Total > 100
    }).
    Process(4, func(ctx context.Context, msg goflux.Message[Event]) error {
        return handleOrder(ctx, msg.Payload)
    })
```

## bridge.FromStream

```go
func FromStream[T any](stream goflow.Stream[Message[T]], pub Publisher[T]) error
```

Consumes a `goflow.Stream` of messages and publishes each one via the provided `Publisher`. The original message subject is used for publishing. Blocks until the stream is exhausted or the context is cancelled.

```go
stream := bridge.ToStream[RawEvent](ctx, sub, "events.raw", 16)

mapped := goflow.Map(stream, func(ctx context.Context, msg goflux.Message[RawEvent]) (goflux.Message[RawEvent], error) {
    msg.Payload.Processed = true
    return msg, nil
})

err := bridge.FromStream(mapped, pub)
```

## BindPublisher

```go
func BindPublisher[T any](pub Publisher[T], subject string) BoundPublisher[T]
```

Wraps a `Publisher[T]` with a fixed subject. Useful when a component always publishes to the same destination and the subject should not leak into business logic.

```go
orderPub := goflux.BindPublisher[OrderEvent](pub, "orders.created")

err := orderPub.Publish(ctx, event)
```

## ToChan

```go
func ToChan[T any](ctx context.Context, sub Subscriber[T], subject string, bufSize int) <-chan Message[T]
```

Bridges a `Subscriber[T]` into a plain Go channel. Launches `Subscribe` in a goroutine and forwards each `Message[T]` (including acker) into a buffered channel. The returned channel closes when `ctx` is cancelled.

```go
ch := goflux.ToChan[Event](ctx, sub, "orders.>", 16)

for msg := range ch {
    fmt.Println("order:", msg.Payload.ID)
}
```

## RetryPublisher

```go
func RetryPublisher[T any](pub Publisher[T], maxAttempts int, backoff BackoffFunc) Publisher[T]

type BackoffFunc func(attempt int) time.Duration
```

Wraps a `Publisher[T]` with retry logic. On publish failure, retries up to `maxAttempts` times with delays determined by `backoff`. Context cancellation aborts the retry loop immediately. If all attempts fail, the last error is returned.

```go
retrying := goflux.RetryPublisher[Event](pub, 3, func(attempt int) time.Duration {
    return time.Duration(attempt+1) * time.Second // 1s, 2s, 3s
})

err := retrying.Publish(ctx, "events.order", event)
```
