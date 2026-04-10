# Transports

Transports implement `Publisher[T]` and `Subscriber[T]`. Your handler code is identical regardless of which transport you use -- only the setup changes.

## Channels (`chan/`)

The channel transport uses Go channels for in-process messaging. No codec is needed because values are passed directly in memory.

### Key Characteristics

- **Bus[T]** acts as the in-process message broker, routing messages by exact subject match.
- **No serialisation** -- the payload is passed by value.
- **Backpressure** -- the publisher blocks when the subscriber buffer is full. No messages are dropped.
- **Close is a no-op** on both Publisher and Subscriber.

### Setup

```go
import _chan "github.com/foomo/goflux/chan"

// Create the bus (shared broker).
bus := _chan.NewBus[OrderEvent]()

// Publisher -- wraps the bus.
pub := _chan.NewPublisher(bus)

// Subscriber -- bufSize controls backpressure.
sub, err := _chan.NewSubscriber(bus, 16)
if err != nil {
    return err
}

// Subscribe blocks -- run in a goroutine.
go func() {
    _ = sub.Subscribe(ctx, "orders", handler)
}()

// Publish a message.
err = pub.Publish(ctx, "orders", OrderEvent{ID: "1"})
```

### API

| Function | Signature |
|----------|-----------|
| `NewBus[T]` | `func NewBus[T any]() *Bus[T]` |
| `NewPublisher[T]` | `func NewPublisher[T any](bus *Bus[T]) *Publisher[T]` |
| `NewSubscriber[T]` | `func NewSubscriber[T any](bus *Bus[T], bufSize int) (*Subscriber[T], error)` |

`Subscriber` also exposes `Len() int64`, which returns the current buffer depth. This value is reported as the `messaging.consumer.lag` gauge via OpenTelemetry.

## NATS (`nats/`)

The NATS transport wraps a `*nats.Conn` and uses NATS core pub/sub. It requires a `goencode.Codec[T]` for serialisation.

### Key Characteristics

- **NATS core** -- no JetStream, no persistence, no ack/nack.
- **Caller owns the connection** -- the transport does not create or manage the `*nats.Conn`.
- **Close drains the connection** -- `Close()` calls `conn.Drain()`.
- Each message is handled in the NATS callback goroutine.
- **Trace propagation** -- OTel span context and message IDs are automatically propagated via NATS message headers.

### Setup

```go
import (
    "github.com/foomo/goencode/json"
    "github.com/foomo/goflux"
    gofluxnats "github.com/foomo/goflux/nats"
    "github.com/nats-io/nats.go"
)

// Connect to NATS (caller owns the connection).
conn, err := nats.Connect("nats://localhost:4222")
if err != nil {
    return err
}
defer conn.Drain()

codec := json.NewCodec[OrderEvent]()

// Publisher
pub := gofluxnats.NewPublisher(conn, codec)

// Subscriber
sub := gofluxnats.NewSubscriber(conn, codec)

go func() {
    _ = sub.Subscribe(ctx, "orders.>", handler)
}()

err = pub.Publish(ctx, "orders.created", OrderEvent{ID: "1"})
```

### API

| Function | Signature |
|----------|-----------|
| `NewPublisher[T]` | `func NewPublisher[T any](conn *nats.Conn, serializer goencode.Codec[T]) *Publisher[T]` |
| `NewSubscriber[T]` | `func NewSubscriber[T any](conn *nats.Conn, serializer goencode.Codec[T]) *Subscriber[T]` |

## HTTP (`http/`)

The HTTP transport publishes by POSTing to a URL and subscribes by exposing an `http.ServeMux`. The subscriber does **not** own a `net.Listener` -- you hand its mux to your HTTP server.

### Key Characteristics

- **Publisher** POSTs to `{baseURL}/{subject}`.
- **Subscriber** registers routes on an internal `*http.ServeMux`, exposed via `Handler()`.
- `WithMaxBodySize` limits request body size (default: 1 MiB). Requests exceeding the limit receive HTTP 413.
- `WithBasePath` prefixes all routes with a base path.
- **Close is a no-op** -- shutdown is handled by the outer HTTP server.
- **Trace propagation** -- OTel span context is injected into HTTP request headers on publish and extracted on subscribe. Message IDs are propagated via the `X-Message-ID` header.

### Setup

```go
import (
    "net/http"

    "github.com/foomo/goencode/json"
    "github.com/foomo/goflux"
    gofluxhttp "github.com/foomo/goflux/http"
)

codec := json.NewCodec[OrderEvent]()

// --- Publisher ---
pub := gofluxhttp.NewPublisher("https://orders.internal", codec, nil)

err := pub.Publish(ctx, "orders.created", OrderEvent{ID: "1"})

// --- Subscriber ---
sub := gofluxhttp.NewSubscriber(codec,
    gofluxhttp.WithMaxBodySize(2 << 20), // 2 MiB
    gofluxhttp.WithBasePath("/webhooks"),
)

// Subscribe registers POST /webhooks/orders.created on the mux.
go func() {
    _ = sub.Subscribe(ctx, "orders.created", handler)
}()

// Hand the mux to your HTTP server.
// The subscriber's Handler method returns an http.HandlerFunc for a given subject.
server := &http.Server{Addr: ":8080", Handler: sub.Handler("orders.created", handler)}
```

### HTTP Response Codes

| Code | Meaning |
|------|---------|
| 204 | Handler returned nil (success) |
| 400 | Body could not be decoded |
| 405 | Method was not POST |
| 413 | Body exceeds `MaxBodySize` |
| 500 | Handler returned an error |

### API

| Function | Signature |
|----------|-----------|
| `NewPublisher[T]` | `func NewPublisher[T any](baseURL string, serializer goencode.Codec[T], client *http.Client) *Publisher[T]` |
| `NewSubscriber[T]` | `func NewSubscriber[T any](codec goencode.Codec[T], opts ...SubscriberOption) *Subscriber[T]` |
| `WithMaxBodySize` | `func WithMaxBodySize(v int64) SubscriberOption` |
| `WithBasePath` | `func WithBasePath(v string) SubscriberOption` |
| `Handler` | `func (s *Subscriber[T]) Handler(subject string, handler Handler[T]) http.HandlerFunc` |

## Comparison

| Feature | chan/ | nats/ | http/ |
|---------|-------|-------|-------|
| Codec required | No | Yes | Yes |
| Network | In-process | TCP | HTTP |
| Publish blocks | Yes (backpressure) | No (fire-and-forget) | Yes (HTTP roundtrip) |
| Subscribe blocks | Yes (event loop) | Yes (ctx.Done) | Yes (ctx.Done) |
| Persistence | None | None (core) | None |
| Trace propagation | N/A (in-process) | NATS headers | HTTP headers |
| System attr | `go_channel` | `nats` | `http` |
| Best for | Testing, in-app | Microservices | Webhooks, inter-service |

## Writing a Custom Transport

To implement a custom transport:

1. **Implement `Publisher[T]`** -- `Publish(ctx, subject, v)` and `Close()`.
2. **Implement `Subscriber[T]`** -- `Subscribe(ctx, subject, handler)` that blocks until ctx is cancelled, and `Close()`.
3. **Declare a `system` variable** for the `messaging.system` OTel attribute:
   ```go
   var system = semconvmsg.SystemAttr("my_transport")
   ```
4. **Call `goflux.RecordPublish`** in your `Publish` method:
   ```go
   func (p *MyPublisher[T]) Publish(ctx context.Context, subject string, v T) error {
       return goflux.RecordPublish(ctx, subject, system, func(ctx context.Context) error {
           // your publish logic here
       })
   }
   ```
5. **Call `goflux.RecordProcess`** in your subscriber's message dispatch loop:
   ```go
   err := goflux.RecordProcess(ctx, subject, system, func(ctx context.Context) error {
       return handler(ctx, msg)
   })
   ```

6. **Propagate trace context** (for network transports). Use `goflux.InjectContext` on the publish side and `goflux.ExtractContext` on the subscribe side to propagate OTel span context across the wire:
   ```go
   // Publisher side — inject into outgoing headers
   goflux.InjectContext(ctx, propagation.HeaderCarrier(headers))

   // Subscriber side — extract from incoming headers
   msgCtx := goflux.ExtractContext(ctx, propagation.HeaderCarrier(headers))
   ```
7. **Propagate message ID** (optional). Read `goflux.MessageID(ctx)` and write it as a header on publish; on subscribe, read the header and call `goflux.WithMessageID(ctx, id)`.

This ensures your transport automatically participates in tracing, metrics, and distributed trace propagation without any additional configuration.

## What's Next

- [Pipelines](./pipelines.md) -- wire subscribers to publishers with filtering and mapping
- [Middleware](./middleware.md) -- add concurrency limiting, deduplication, and more
- [Telemetry](./telemetry.md) -- understand the metrics and traces produced by transports
