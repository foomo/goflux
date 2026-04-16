# API Reference

Quick reference for all goflux types, interfaces, and operators.

## Transport Support Matrix

| Interface | Channel | NATS | JetStream | HTTP |
|-----------|---------|------|-----------|------|
| `Publisher[T]` | yes | yes | yes | yes |
| `Subscriber[T]` | yes | yes | yes | yes |
| `Requester[Req, Resp]` | - | yes | - | yes |
| `Responder[Req, Resp]` | - | yes | - | yes |

## Messaging Patterns

| Pattern | Core Interface(s) | Transport(s) |
|---------|-------------------|--------------|
| Fire and Forget | `Publisher` + `Subscriber`, acker=nil | channel, nats |
| At-Least-Once (push) | `Publisher` + `Subscriber`, auto-ack | jetstream |
| At-Least-Once (pull) | `Publisher` + `Subscriber` + `WithManualAck`, middleware ack | jetstream |
| Stream Processing | `ToStream` + goflow operators | any |
| Manual Ack/Nak/Term | `Subscriber` + `WithManualAck()` | jetstream |
| Exactly-Once | `Publisher` with `Nats-Msg-Id` header | jetstream |
| Request-Reply | `Requester` + `Responder` | nats, http |
| Queue Groups | `Subscriber` + `WithQueueGroup` | nats |
| Fan-out | `ToStream` + goflow `Tee` / `FanOut` | any |
| Fan-in | `ToStream` + goflow `FanIn` | any |

## Core Types

### Interfaces

```go
// Publisher sends encoded messages to a subject/topic.
type Publisher[T any] interface {
    Publish(ctx context.Context, subject string, v T) error
    Close() error
}

// Subscriber listens on a subject and dispatches decoded messages to a Handler.
// Subscribe blocks until ctx is cancelled or a fatal error occurs.
type Subscriber[T any] interface {
    Subscribe(ctx context.Context, subject string, handler Handler[T]) error
    Close() error
}

// Requester sends a typed request and waits for a typed response.
type Requester[Req, Resp any] interface {
    Request(ctx context.Context, subject string, req Req) (Resp, error)
    Close() error
}

// Responder handles incoming requests and produces typed responses.
type Responder[Req, Resp any] interface {
    Serve(ctx context.Context, subject string, handler RequestHandler[Req, Resp]) error
    Close() error
}
```

### Handler and RequestHandler

```go
// Handler is the callback for Subscriber.Subscribe.
type Handler[T any] func(ctx context.Context, msg Message[T]) error

// RequestHandler processes a request and returns a response.
type RequestHandler[Req, Resp any] func(ctx context.Context, req Req) (Resp, error)
```

### Message[T]

```go
type Message[T any] struct {
    Subject string
    Payload T
    Header  Header
}
```

| Method | Description |
|--------|-------------|
| `NewMessage(subject, payload)` | Create a message |
| `NewMessageWithHeader(subject, payload, header)` | Create a message with header |
| `msg.WithAcker(a)` | Attach an acker (transport use only) |
| `msg.HasAcker()` | Check if acker is attached |
| `msg.Ack()` | Acknowledge successful processing |
| `msg.Nak()` | Signal processing failure (redelivery) |
| `msg.NakWithDelay(d)` | Nak with redelivery delay hint |
| `msg.Term()` | Terminal rejection (no redelivery) |

### Header

```go
type Header map[string][]string
```

| Method | Description |
|--------|-------------|
| `h.Get(key)` | First value for key, or `""` |
| `h.Set(key, value)` | Replace all values for key |
| `h.Add(key, value)` | Append a value for key |
| `h.Del(key)` | Remove key |
| `h.Clone()` | Deep copy |

### Acknowledgment Interfaces

```go
// Acker -- minimal acknowledgment (Ack + Nak).
type Acker interface {
    Ack() error
    Nak() error
}

// DelayedNaker -- extends Acker with delayed negative acknowledgment.
type DelayedNaker interface {
    Acker
    NakWithDelay(d time.Duration) error
}

// Terminator -- extends Acker with terminal rejection.
type Terminator interface {
    Acker
    Term() error
}
```

### Convenience Types

| Type | Description |
|------|-------------|
| `Topic[T]` | Bundles `Publisher[T]` + `Subscriber[T]` |
| `BoundPublisher[T]` | Wraps `Publisher[T]` with a fixed subject |
| `Middleware[T]` | `func(Handler[T]) Handler[T]` |
| `PublisherMiddleware[T]` | `func(Publisher[T]) Publisher[T]` |
| `Filter[T]` | `func(ctx, msg Message[T]) (bool, error)` |
| `MapFunc[T, U]` | `func(ctx, msg Message[T]) (Message[U], error)` |
| `DeadLetterFunc[T]` | `func(ctx, msg Message[T], err error)` |

### Context Helpers

| Function | Description |
|----------|-------------|
| `WithMessageID(ctx, id)` | Attach business-level message ID |
| `MessageID(ctx)` | Read message ID from context |
| `WithHeader(ctx, h)` | Attach header to context (read by transports on Publish) |
| `HeaderFromContext(ctx)` | Read header from context |

## Middleware

All middleware have the signature `Middleware[T]` = `func(Handler[T]) Handler[T]`.

Built-in implementations live in the `github.com/foomo/goflux/middleware` package. The `Middleware[T]` type and `Chain[T]` remain in the root package.

| Middleware | Package | Signature | Description |
|------------|---------|-----------|-------------|
| `AutoAck[T]()` | `middleware` | `-> Middleware[T]` | Ack on nil error, Nak on non-nil |
| `RetryAck[T](policy)` | `middleware` | `RetryPolicy -> Middleware[T]` | Classify errors into ack/nak/term via policy |
| `InjectMessageID[T]()` | `middleware` | `-> Middleware[T]` | Header message ID → context |
| `InjectHeader[T]()` | `middleware` | `-> Middleware[T]` | Message header → context |
| `Chain[T](mws...)` | `goflux` | `...Middleware[T] -> Middleware[T]` | Compose left-to-right |

::: tip Stream Processing Operators
For concurrency limiting, deduplication, throttling, skip/take, and peek, use [goflow](https://github.com/foomo/goflow) operators via `ToStream`.
:::

See [Middleware](/middleware/) for details and examples.

## Pipeline Operators

| Operator | Description |
|----------|-------------|
| `Pipe[T](pub, opts...)` | Forward messages to a publisher |
| `PipeMap[T, U](pub, mapFn, opts...)` | Transform and forward |
| `Bind[T](pub, subject)` | Fix subject on a publisher |
| `ToChan[T](ctx, sub, subject, bufSize)` | Bridge subscriber to `<-chan Message[T]` |
| `ToStream[T](ctx, sub, subject, bufSize)` | Bridge subscriber to `goflow.Stream[Message[T]]` |
| `FromStream[T](ctx, stream, pub)` | Consume goflow stream and publish each message |
| `RetryPublisher[T](pub, maxAttempts, backoff)` | Wrap publisher with retry logic |

### Pipe Options

| Option | Description |
|--------|-------------|
| `WithFilter[T](f)` | Drop messages where filter returns false |
| `WithDeadLetter[T](fn)` | Handle failed messages |

See [Pipeline Operators](/pipeline/) for details and examples.

## Group Coordination

```go
// Group coordinates the lifecycle of multiple message handlers.
// Fail-fast: first error cancels all sibling tasks.
type Group struct{ /* ... */ }
```

| Function / Method | Description |
|-------------------|-------------|
| `NewGroup(opts...)` | Create a lifecycle coordinator |
| `WithGroupOptions(opts...)` | Pass `gofuncy.GroupOption` to the underlying group |
| `g.Go(name, fn)` | Register a named blocking task |
| `g.GoWithReady(name, fn)` | Register a task that signals readiness via `ready()` |
| `g.Run(ctx)` | Start all tasks, block until done (fail-fast) |
| `g.RunWithReady(ctx, onReady)` | Start all tasks, call `onReady` when all are ready |
| `GroupSubscribe[T](g, name, sub, subject, handler, mws...)` | Register a Subscriber handler on the group |

```go
g := goflux.NewGroup()

goflux.GroupSubscribe(g, "orders", orderSub, "orders.new", orderHandler)

err := g.Run(ctx) // blocks, fail-fast on first error
```

## RetryPublisher

```go
func RetryPublisher[T any](pub Publisher[T], maxAttempts int, backoff BackoffFunc) Publisher[T]

type BackoffFunc func(attempt int) time.Duration
```

Wraps a `Publisher[T]` with retry logic. On publish failure, retries up to `maxAttempts` times with delays from `backoff`. Context cancellation aborts immediately.

```go
pub := goflux.RetryPublisher[Event](innerPub, 3, func(attempt int) time.Duration {
    return time.Duration(attempt+1) * time.Second // linear backoff
})
```

## Sentinel Errors

| Error | Description |
|-------|-------------|
| `ErrPublish` | Failure in the publish path |
| `ErrSubscribe` | Failure in the subscribe path |
| `ErrEncode` | Serialization failure |
| `ErrDecode` | Deserialization failure |
| `ErrTransport` | Transport-level failure (network, protocol) |

Transports join these with causal errors via `errors.Join`, enabling:

```go
if errors.Is(err, goflux.ErrEncode) { /* handle codec problem */ }
```

## Retry Policy (`middleware` package)

```go
type RetryPolicy func(err error) RetryDecision

type RetryDecision struct {
    Action RetryAction
    Delay  time.Duration
}
```

| Value | Package | Description |
|-------|---------|-------------|
| `RetryNak` | `middleware` | Immediate redelivery via `Nak()` |
| `RetryNakWithDelay` | `middleware` | Delayed redelivery via `NakWithDelay(d)` |
| `RetryTerm` | `middleware` | Terminal rejection via `Term()` |

| Function | Package | Description |
|----------|---------|-------------|
| `NewRetryPolicy(delay)` | `middleware` | Term non-retryable, nak-with-delay otherwise |
| `RetryAck[T](policy)` | `middleware` | Middleware that applies retry policy to ack decisions |
| `ErrNonRetryable` | `goflux` | Sentinel for permanent failures |
| `NonRetryable(err)` | `goflux` | Wrap error as non-retryable |
| `IsNonRetryable(err)` | `goflux` | Check if error is non-retryable |

See [Stream Processing](/guide/patterns/stream-processor) for details and examples.

## Telemetry

| Function / Method | Description |
|-------------------|-------------|
| `NewTelemetry(opts...)` | Create instrumentation instance |
| `DefaultTelemetry(tel)` | Return `tel` if non-nil, else create from OTel globals |
| `NewNoopTelemetry()` | Create noop instance (safe calls, no output) |
| `WithTracerProvider(tp)` | Set tracer provider |
| `WithMeterProvider(mp)` | Set meter provider |
| `WithPropagator(p)` | Set text-map propagator |
| `RecordPublish(ctx, subject, system, fn)` | Producer span |
| `RecordProcess(ctx, subject, system, fn, opts...)` | Consumer span |
| `RecordRequest(ctx, subject, system, fn)` | Request-reply client span |
| `RecordAckOutcome(ctx, action, subject, err)` | Record ack/nak/term outcome |
| `RegisterLag(subject, lagFn)` | Observable gauge |
| `InjectContext(ctx, carrier)` | Inject span context (publish side) |
| `ExtractContext(ctx, carrier)` | Extract as parent (sync transports) |
| `ExtractSpanContext(ctx, carrier)` | Extract as link (async transports) |
| `WithRemoteSpanContext(sc)` | Attach producer span as link |

See [Telemetry](/telemetry/) for details.
