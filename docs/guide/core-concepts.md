# Core Concepts

This page covers the fundamental types in goflux and the rules that govern how they interact.

## Message[T]

`Message[T]` is the unit passed to every handler. It carries two fields:

```go
type Message[T any] struct {
    Subject string `json:"subject"`
    Payload T      `json:"payload"`
}
```

- **Subject** -- the routing key (e.g. a NATS subject, an HTTP path segment, or a channel topic name).
- **Payload** -- the fully decoded value. Transports decode at the boundary; handlers never see raw bytes.

Use `NewMessage` to construct a message:

```go
msg := goflux.NewMessage("orders.created", OrderEvent{ID: "42", Total: 99.95})
```

## Handler[T]

A handler is the callback that processes incoming messages:

```go
type Handler[T any] func(ctx context.Context, msg Message[T]) error
```

- Returning `nil` signals success.
- Returning a non-nil error signals the transport to nack or requeue the message. The exact behaviour is transport-specific.

```go
handler := func(ctx context.Context, msg goflux.Message[OrderEvent]) error {
    fmt.Println("received:", msg.Payload.ID)
    return nil
}
```

## Publisher[T]

A publisher sends messages to a subject:

```go
type Publisher[T any] interface {
    Publish(ctx context.Context, subject string, v T) error
    Close() error
}
```

- `Publish` serialises `v` via the transport's codec (if any) and delivers it to the subject.
- `Close` releases underlying connections. The exact behaviour depends on the transport.
- **The caller owns the connection.** Transport constructors for NATS and HTTP do not create or own their underlying connection; the caller connects and closes.

```go
bus := _chan.NewBus[OrderEvent]()
pub := _chan.NewPublisher(bus)

err := pub.Publish(ctx, "orders.created", OrderEvent{ID: "42"})
```

## Subscriber[T]

A subscriber listens on a subject and dispatches decoded messages to a handler:

```go
type Subscriber[T any] interface {
    Subscribe(ctx context.Context, subject string, handler Handler[T]) error
    Close() error
}
```

::: warning
`Subscribe` **blocks** until the context is cancelled or the transport encounters a fatal error. Always run it in a goroutine.
:::

```go
sub, err := _chan.NewSubscriber(bus, 8)
if err != nil {
    return err
}

go func() {
    _ = sub.Subscribe(ctx, "orders.created", handler)
}()
```

## Topic[T]

`Topic[T]` is a convenience struct that bundles a `Publisher[T]` and a `Subscriber[T]` for services that need to both produce and consume the same message type:

```go
type Topic[T any] struct {
    Publisher[T]
    Subscriber[T]
}
```

```go
topic := goflux.Topic[OrderEvent]{
    Publisher:  pub,
    Subscriber: sub,
}

// Use as a publisher
topic.Publish(ctx, "orders", event)

// Use as a subscriber
go func() {
    _ = topic.Subscribe(ctx, "orders", handler)
}()
```

## ToChan[T]

`ToChan` bridges a `Subscriber[T]` into a plain Go channel. It launches `Subscribe` in a goroutine and forwards each message payload into a buffered channel:

```go
func ToChan[T any](ctx context.Context, sub Subscriber[T], subject string, bufSize int) <-chan T
```

- `bufSize` controls backpressure: a full buffer blocks the subscriber's handler until the consumer reads.
- The returned channel closes when `ctx` is cancelled.

```go
bus := _chan.NewBus[OrderEvent]()
sub, _ := _chan.NewSubscriber(bus, 8)

ch := goflux.ToChan(ctx, sub, "orders", 16)

for event := range ch {
    fmt.Println("got order:", event.ID)
}
```

## Context Helpers

goflux provides opt-in context utilities for message correlation:

```go
// Attach a business-level message ID before publishing.
ctx = goflux.WithMessageID(ctx, "order-42")

// Read it back in a handler.
id := goflux.MessageID(ctx)
```

When set, the message ID is automatically:
- Added to publish/process spans as `messaging.message.id`
- Propagated via transport headers (`X-Message-ID` for HTTP, NATS headers)

See [Telemetry](./telemetry.md#message-id) for details.

## Key Rules

::: tip Rules to Remember
1. **Subscribe blocks.** Always run it in a goroutine.
2. **Caller owns connections.** Transport constructors do not create or close underlying connections (NATS `*nats.Conn`, HTTP `*http.Client`).
3. **Close semantics vary.** `Close()` on `FanOut`, `FanIn`, `RoundRobin`, and `chan/` types is a no-op. NATS transports call `conn.Drain()`. Check the transport documentation.
4. **No raw bytes in handlers.** `Message[T]` always carries the fully decoded payload. Decoding happens at the transport boundary.
5. **Codecs are stateless.** Share them freely across publishers and subscribers.
:::

## What's Next

- [Transports](./transports.md) -- choose between channels, NATS, and HTTP
- [Pipelines](./pipelines.md) -- wire subscribers to publishers with filtering and transformation
- [Middleware](./middleware.md) -- wrap handlers with cross-cutting behaviour
