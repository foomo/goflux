# API Reference

Quick reference for all goflux types, interfaces, and operators.

## Transport Support Matrix

| Interface | Channel | NATS | JetStream | HTTP |
|-----------|---------|------|-----------|------|
| `Publisher[T]` | yes | yes | yes | yes |
| `Subscriber[T]` | yes | yes | yes | yes |
| `Consumer[T]` | - | - | yes | - |
| `Requester[Req, Resp]` | - | yes | - | yes |
| `Responder[Req, Resp]` | - | yes | - | yes |

## Messaging Patterns

| Pattern | Core Interface(s) | Transport(s) |
|---------|-------------------|--------------|
| Fire and Forget | `Publisher` + `Subscriber`, acker=nil | channel, nats |
| At-Least-Once (push) | `Publisher` + `Subscriber`, auto-ack | jetstream |
| At-Least-Once (pull) | `Publisher` + `Consumer`, explicit ack | jetstream |
| Manual Ack/Nak/Term | `Subscriber` + `WithManualAck()` | jetstream |
| Exactly-Once | `Publisher` with `Nats-Msg-Id` header | jetstream |
| Request-Reply | `Requester` + `Responder` | nats, http |
| Queue Groups | `Subscriber` + `WithQueueGroup` | nats |
| Fan-out | `FanOut` operator | any |
| Fan-in | `FanIn` operator | any |

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

// Consumer provides pull-based message consumption.
// Each fetched message MUST be explicitly acknowledged.
type Consumer[T any] interface {
    Fetch(ctx context.Context, n int) ([]Message[T], error)
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

| Middleware | Signature | Description |
|------------|-----------|-------------|
| `Process[T](n)` | `int -> Middleware[T]` | Concurrency limiter (semaphore, blocks when full) |
| `Peek[T](fn)` | `func(ctx, msg) -> Middleware[T]` | Side-effect tap, errors ignored |
| `Distinct[T](key)` | `func(msg) string -> Middleware[T]` | Dedup by key (unbounded set, first wins) |
| `Skip[T](n)` | `int -> Middleware[T]` | Drop first n, pass rest |
| `Take[T](n)` | `int -> Middleware[T]` | Pass first n, drop rest |
| `Throttle[T](d)` | `time.Duration -> Middleware[T]` | Rate-limit to one per duration |
| `AutoAck[T]()` | `-> Middleware[T]` | Ack on nil error, Nak on non-nil |
| `Chain[T](mws...)` | `...Middleware[T] -> Middleware[T]` | Compose left-to-right |

See [Middleware](/middleware/) for details and examples.

## Pipeline Operators

| Operator | Description |
|----------|-------------|
| `Pipe[T](pub, opts...)` | Forward messages to a publisher |
| `PipeMap[T, U](pub, mapFn, opts...)` | Transform and forward |
| `FanOut[T](pubs, opts...)` | Broadcast to N publishers |
| `FanIn[T](subs...)` | Merge N subscribers into one handler |
| `RoundRobin[T](pubs...)` | Cycle through publishers |
| `Bind[T](pub, subject)` | Fix subject on a publisher |
| `ToChan[T](ctx, sub, subject, bufSize)` | Bridge subscriber to channel |

### Pipe Options

| Option | Description |
|--------|-------------|
| `WithFilter[T](f)` | Drop messages where filter returns false |
| `WithDeadLetter[T](fn)` | Handle failed messages |

### FanOut Options

| Option | Description |
|--------|-------------|
| `WithFanOutAllOrNothing[T]()` | First error short-circuits |

See [Pipeline Operators](/pipeline/) for details and examples.

## Telemetry

| Function / Method | Description |
|-------------------|-------------|
| `NewTelemetry(opts...)` | Create instrumentation instance |
| `WithTracerProvider(tp)` | Set tracer provider |
| `WithMeterProvider(mp)` | Set meter provider |
| `WithPropagator(p)` | Set text-map propagator |
| `RecordPublish(ctx, subject, system, fn)` | Producer span |
| `RecordProcess(ctx, subject, system, fn, opts...)` | Consumer span |
| `RecordFetch(ctx, subject, system, count, fn)` | Pull consumer span |
| `RecordRequest(ctx, subject, system, fn)` | Request-reply client span |
| `RegisterLag(subject, lagFn)` | Observable gauge |
| `InjectContext(ctx, carrier)` | Inject span context (publish side) |
| `ExtractContext(ctx, carrier)` | Extract as parent (sync transports) |
| `ExtractSpanContext(ctx, carrier)` | Extract as link (async transports) |
| `WithRemoteSpanContext(sc)` | Attach producer span as link |

See [Telemetry](/telemetry/) for details.
