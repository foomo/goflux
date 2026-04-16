# Pipeline Operators

Pipeline operators wire subscribers to publishers, composing `Handler[T]` and `Publisher[T]` into message-processing topologies. They handle filtering, transformation, and bridging to [goflow](https://github.com/foomo/goflow) streams.

::: tip Stream Processing
For fan-out, fan-in, round-robin, and other stream-processing patterns use [goflow](https://github.com/foomo/goflow) operators via the `ToStream` / `FromStream` bridge functions.
:::

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

## ToStream

```go
func ToStream[T any](ctx context.Context, sub Subscriber[T], subject string, bufSize int) goflow.Stream[Message[T]]
```

Bridges a `Subscriber[T]` into a `goflow.Stream`. All [goflow](https://github.com/foomo/goflow) operators (Filter, Map, Distinct, FanOut, etc.) can be applied to the returned stream.

```go
stream := goflux.ToStream[Event](ctx, sub, "orders.>", 16)

// Apply goflow operators
stream.
    Filter(func(ctx context.Context, msg goflux.Message[Event]) bool {
        return msg.Payload.Total > 100
    }).
    Process(4, func(ctx context.Context, msg goflux.Message[Event]) error {
        return handleOrder(ctx, msg.Payload)
    })
```

## FromStream

```go
func FromStream[T any](ctx context.Context, stream goflow.Stream[Message[T]], pub Publisher[T]) error
```

Consumes a `goflow.Stream` of messages and publishes each one via the provided `Publisher`. Blocks until the stream is exhausted or the context is cancelled.

```go
stream := goflux.ToStream[RawEvent](ctx, sub, "events.raw", 16)

// Transform with goflow, then publish to a different transport
mapped := goflow.Map(stream, func(ctx context.Context, msg goflux.Message[RawEvent]) (goflux.Message[RawEvent], error) {
    msg.Payload.Processed = true
    return msg, nil
})

err := goflux.FromStream(ctx, mapped, pub)
```

## BoundPublisher

```go
func Bind[T any](pub Publisher[T], subject string) *BoundPublisher[T]
```

Wraps a `Publisher[T]` with a fixed subject. Useful when a component always publishes to the same destination and the subject should not leak into business logic.

```go
orderPub := goflux.Bind[OrderEvent](pub, "orders.created")

// The subject argument is ignored -- "orders.created" is always used
err := orderPub.Publish(ctx, "", event)
```

`BoundPublisher` implements `Publisher[T]` — the subject parameter in `Publish` is ignored. `Close()` delegates to the underlying publisher.

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

