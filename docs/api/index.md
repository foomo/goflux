---
outline: deep
---

# API Reference

Complete reference for all exported types, functions, and methods.

Package: [`github.com/foomo/goflux`](https://pkg.go.dev/github.com/foomo/goflux)

## Core Types

### Message[T]

```go
// Message is the unit passed to every Handler. Subject carries the routing
// key (e.g. a NATS subject or Redis channel); Payload holds the decoded value.
type Message[T any] struct {
    Subject string `json:"subject"`
    Payload T      `json:"payload"`
}
```

All transports decode fully at the boundary -- `Message[T]` never carries raw bytes.

### NewMessage

```go
// NewMessage creates a new Message.
func NewMessage[T any](subject string, payload T) Message[T]
```

### Publisher[T]

```go
// Publisher sends encoded messages to a subject/topic.
type Publisher[T any] interface {
    // Publish serializes v via the bound Codec and delivers it to the subject.
    Publish(ctx context.Context, subject string, v T) error
    // Close releases any underlying connections.
    Close() error
}
```

### Subscriber[T]

```go
// Subscriber listens on one or more subjects and dispatches decoded messages
// to a Handler.
type Subscriber[T any] interface {
    // Subscribe registers handler for the subject. The call blocks until ctx is
    // canceled or the implementation encounters a fatal error.
    Subscribe(ctx context.Context, subject string, handler Handler[T]) error
    // Close unsubscribes and releases resources.
    Close() error
}
```

`Subscribe` always blocks until the context is cancelled. Run it in a goroutine.

### Handler[T]

```go
// Handler is the callback signature used by Subscriber.Subscribe.
// Returning a non-nil error signals the subscriber to nack / requeue the
// message (behavior is implementation-specific).
type Handler[T any] func(ctx context.Context, msg Message[T]) error
```

### Topic[T]

```go
// Topic bundles a Publisher and Subscriber sharing the same Codec. Use it
// when a service needs to both produce and consume the same message type.
type Topic[T any] struct {
    Publisher[T]
    Subscriber[T]
}
```

### ToChan

```go
// ToChan bridges a Subscriber into a plain channel. It launches Subscribe in a
// goroutine and forwards each message payload into a buffered channel. The
// returned channel closes when ctx is cancelled.
//
// bufSize controls backpressure: a full buffer blocks the subscriber's handler
// until the consumer catches up.
func ToChan[T any](ctx context.Context, sub Subscriber[T], subject string, bufSize int) <-chan T
```

---

## Pipeline

### Filter[T]

```go
// Filter decides whether a message should be forwarded.
// Returning false or a non-nil error drops the message and logs the reason.
type Filter[T any] func(ctx context.Context, msg Message[T]) (bool, error)
```

### MapFunc[T, U]

```go
// MapFunc transforms a Message[T] into a Message[U].
// A non-nil error drops the message and routes it to the DeadLetterFunc if set.
type MapFunc[T, U any] func(ctx context.Context, msg Message[T]) (Message[U], error)
```

### DeadLetterFunc[T]

```go
// DeadLetterFunc receives messages that could not be mapped or published after
// all retries are exhausted. Use it to log, alert, or forward to a DLQ.
type DeadLetterFunc[T any] func(ctx context.Context, msg Message[T], err error)
```

### PipeOption[T]

```go
// PipeOption configures a Pipe or PipeMap call.
type PipeOption[T any] func(*pipeConfig[T])
```

### WithFilter

```go
// WithFilter registers a filter that runs before publish (or before map in
// PipeMap). Messages for which the filter returns false are silently dropped
// and logged. Filter errors are treated as false.
func WithFilter[T any](f Filter[T]) PipeOption[T]
```

### WithDeadLetter

```go
// WithDeadLetter registers a dead-letter handler called when MapFunc returns
// an error or when the publisher fails after all retries are exhausted.
// The original Message[T] and the terminal error are passed to the handler.
func WithDeadLetter[T any](fn DeadLetterFunc[T]) PipeOption[T]
```

### Pipe

```go
// Pipe returns a Handler[T] that forwards every accepted message to pub.
// Filters run first; a dropped message never reaches pub.
// A publish error is returned to the subscriber as-is -- wrap pub with
// NewRetryPublisher to add retry/backoff before that error surfaces.
func Pipe[T any](pub Publisher[T], opts ...PipeOption[T]) Handler[T]
```

### PipeMap

```go
// PipeMap returns a Handler[T] that maps each message from T to U before
// publishing. Filters run on T before the map. A map error routes the original
// Message[T] to the dead-letter handler (if set) and drops the message.
// A publish error after a successful map is also dead-lettered with the
// original T message.
func PipeMap[T, U any](pub Publisher[U], mapFn MapFunc[T, U], opts ...PipeOption[T]) Handler[T]
```

**Error handling summary:**

| Stage   | Error            | Action                                          |
|---------|------------------|-------------------------------------------------|
| Filter  | returns error    | drop + log (warn), subscriber sees nil          |
| Filter  | returns false    | drop + log (debug), subscriber sees nil         |
| MapFunc | returns error    | drop + log (error) + dead-letter, returns nil   |
| Publish | returns error    | dead-letter + return error to subscriber        |

---

## Middleware

### Middleware[T]

```go
// Middleware wraps a Handler[T] to add cross-cutting behaviour such as
// logging, rate-limiting, or circuit-breaking.
type Middleware[T any] func(Handler[T]) Handler[T]
```

### Chain

```go
// Chain composes middlewares left-to-right: the first middleware in the list
// is the outermost wrapper. Chain(a, b)(h) is equivalent to a(b(h)).
func Chain[T any](mws ...Middleware[T]) Middleware[T]
```

### Process

```go
// Process returns a Middleware that limits concurrent handler invocations to n.
// When all n slots are occupied, the handler blocks until a slot frees up or the
// context is cancelled. Each handler invocation runs synchronously -- errors are
// returned to the caller as-is.
func Process[T any](n int) Middleware[T]
```

### Peek

```go
// Peek returns a Middleware that calls fn as a side-effect for each message
// before forwarding to the next handler. The fn function should not modify the
// message. Errors from fn are intentionally ignored -- use a regular middleware
// if error handling is needed.
func Peek[T any](fn func(context.Context, Message[T])) Middleware[T]
```

### Distinct

```go
// Distinct returns a Middleware that deduplicates messages using a key function.
// The first message for each key passes through; subsequent messages with the
// same key are silently dropped. The seen set is not bounded -- use a custom
// middleware if memory is a concern.
func Distinct[T any](key func(Message[T]) string) Middleware[T]
```

### Skip

```go
// Skip returns a Middleware that drops the first n messages and passes the rest
// through to the next handler. The counter is safe for concurrent use.
func Skip[T any](n int) Middleware[T]
```

### Take

```go
// Take returns a Middleware that passes the first n messages through and
// silently drops all subsequent messages. The counter is safe for concurrent use.
func Take[T any](n int) Middleware[T]
```

### Throttle

```go
// Throttle returns a Middleware that rate-limits handler invocations to at most
// one per duration d. The first message passes immediately; subsequent messages
// block until the ticker fires. Context cancellation is respected while waiting.
func Throttle[T any](d time.Duration) Middleware[T]
```

---

## Distribution

### FanOut

```go
// FanOut returns a Publisher[T] that forwards every Publish call to all inner
// publishers. Errors from individual publishers are joined via errors.Join.
//
// Close is a no-op -- the caller owns the inner publishers and is responsible
// for closing them.
func FanOut[T any](publishers []Publisher[T], opts ...FanOutOption[T]) Publisher[T]
```

### FanOutOption[T]

```go
// FanOutOption configures a FanOut publisher.
type FanOutOption[T any] func(*fanoutConfig[T])
```

### WithFanOutAllOrNothing

```go
// WithFanOutAllOrNothing makes the FanOut publisher treat any single publish
// failure as a failure for the entire call. The default is best-effort: publish
// to all, join errors from any that fail.
func WithFanOutAllOrNothing[T any]() FanOutOption[T]
```

### FanIn

```go
// FanIn returns a Subscriber[T] that subscribes to the same subject on all
// provided subscribers and dispatches every message to a single handler.
// Subscribe blocks until all inner subscriptions complete (i.e. all contexts
// are cancelled or all return an error).
//
// Close is a no-op -- the caller owns the inner subscribers and is responsible
// for closing them.
func FanIn[T any](subscribers ...Subscriber[T]) Subscriber[T]
```

### RoundRobin

```go
// RoundRobin returns a Publisher[T] that distributes each Publish call to a
// single inner publisher, cycling through them in round-robin order.
//
// Close is a no-op -- the caller owns the inner publishers and is responsible
// for closing them.
func RoundRobin[T any](publishers ...Publisher[T]) Publisher[T]
```

---

## Transport: chan/

Package `chan` provides an in-process channel-based transport. No serialization occurs -- values of type `T` are passed by value through Go channels. Blocking publish semantics mean slow consumers apply backpressure to the publisher goroutine; no messages are dropped.

Import: `github.com/foomo/goflux/chan`

### Bus[T]

```go
// Bus is a simple in-process pub/sub broker. Subjects are matched by exact
// string equality. A Bus must be created with NewBus and is safe for
// concurrent use.
type Bus[T any] struct {
    // contains filtered or unexported fields
}
```

### NewBus

```go
// NewBus creates a Bus for type T.
func NewBus[T any]() *Bus[T]
```

### NewPublisher (chan)

```go
func NewPublisher[T any](bus *Bus[T]) *Publisher[T]
```

Returns a `Publisher[T]` backed by the given Bus. `Close()` is a no-op.

#### Publisher.Publish

```go
func (p *Publisher[T]) Publish(ctx context.Context, subject string, v T) error
```

#### Publisher.Close

```go
func (p *Publisher[T]) Close() error
```

### NewSubscriber (chan)

```go
func NewSubscriber[T any](bus *Bus[T], bufSize int) (*Subscriber[T], error)
```

Creates a channel-backed subscriber with the given buffer size. Registers a `messaging.consumer.lag` gauge on construction. Returns an error if the OTel gauge registration fails.

#### Subscriber.Subscribe

```go
func (s *Subscriber[T]) Subscribe(ctx context.Context, subject string, handler goflux.Handler[T]) error
```

#### Subscriber.Len

```go
func (s *Subscriber[T]) Len() int64
```

Returns the current number of messages buffered in the subscriber's channel. Safe for concurrent use. Used as the callback for the `messaging.consumer.lag` OTel gauge.

#### Subscriber.Close

```go
func (s *Subscriber[T]) Close() error
```

---

## Transport: nats/

Package `nats` provides Publisher and Subscriber implementations backed by NATS core (not JetStream). The transport does not own the `*nats.Conn` -- the caller is responsible for connecting and closing it.

Import: `github.com/foomo/goflux/nats`

### NewPublisher (nats)

```go
func NewPublisher[T any](conn *nats.Conn, serializer goencode.Codec[T]) *Publisher[T]
```

Wraps a `*nats.Conn` for publishing. `Close()` calls `conn.Drain()`.

#### Publisher.Publish

```go
func (p *Publisher[T]) Publish(ctx context.Context, subject string, v T) error
```

#### Publisher.Close

```go
func (p *Publisher[T]) Close() error
```

### NewSubscriber (nats)

```go
func NewSubscriber[T any](conn *nats.Conn, serializer goencode.Codec[T]) *Subscriber[T]
```

Wraps a `*nats.Conn` for subscribing. Each message is handled in the NATS callback goroutine. `Close()` calls `conn.Drain()`.

#### Subscriber.Subscribe

```go
func (s *Subscriber[T]) Subscribe(ctx context.Context, subject string, handler goflux.Handler[T]) error
```

Blocks until `ctx` is cancelled, then unsubscribes from the NATS subject.

#### Subscriber.Close

```go
func (s *Subscriber[T]) Close() error
```

---

## Transport: http/

Package `http` provides an HTTP POST-based transport. The publisher POSTs encoded messages to `{baseURL}/{subject}`. The subscriber exposes an `http.HandlerFunc` and does not own a `net.Listener` -- lifecycle belongs to the server layer (e.g. keel).

Import: `github.com/foomo/goflux/http`

### Constants

```go
const (
    DefaultBasePath string = ""
    // DefaultMaxBodySize is the maximum request body size the subscriber will
    // read. Override per-subscriber via the WithMaxBodySize option.
    DefaultMaxBodySize int64 = 1 << 20 // 1 MiB
)
```

### NewPublisher (http)

```go
// NewPublisher creates an HTTP publisher.
// baseURL is the target service root, e.g. "https://orders.internal".
// An optional *http.Client may be provided; if nil the default client is used.
func NewPublisher[T any](baseURL string, serializer goencode.Codec[T], client *http.Client) *Publisher[T]
```

The `Publisher[T]` struct also exposes a `ContentType` field (defaults to `"application/json"` if empty).

#### Publisher.Publish

```go
// Publish encodes v and POSTs it to {baseURL}/{subject}.
// A non-2xx response is treated as an error.
func (p *Publisher[T]) Publish(ctx context.Context, subject string, v T) error
```

#### Publisher.Close

```go
func (p *Publisher[T]) Close() error
```

### NewSubscriber (http)

```go
// NewSubscriber creates an HTTP subscriber. Call Subscribe to register subjects,
// then pass Handler() to service.NewHTTP (keel) or http.ListenAndServe.
func NewSubscriber[T any](codec goencode.Codec[T], opts ...SubscriberOption) *Subscriber[T]
```

### SubscriberOption

```go
// SubscriberOption configures a Subscriber.
type SubscriberOption func(*subscriberConfig)
```

### WithMaxBodySize

```go
// WithMaxBodySize sets the maximum request body size the subscriber will read.
// Requests exceeding this limit receive 413 Request Entity Too Large.
func WithMaxBodySize(v int64) SubscriberOption
```

### WithBasePath

```go
// WithBasePath sets the basePath configuration for a Subscriber to the specified value.
func WithBasePath(v string) SubscriberOption
```

#### Subscriber.Subscribe

```go
func (s *Subscriber[T]) Subscribe(ctx context.Context, subject string, handler goflux.Handler[T]) error
```

Registers handler for `POST {basePath}/{subject}` on the internal mux and blocks until `ctx` is cancelled.

**Response codes:** 204 on success, 400 on decode failure, 405 on wrong method, 413 if body exceeds max size, 500 if the handler returns an error.

#### Subscriber.Handler

```go
// Handler returns an http.HandlerFunc that dispatches incoming POST requests
// to the given handler.
func (s *Subscriber[T]) Handler(subject string, handler goflux.Handler[T]) http.HandlerFunc
```

Use this to register routes manually or integrate with keel's `service.NewHTTP`.

#### Subscriber.Close

```go
// Close is a no-op; shutdown is handled by keel / the outer http.Server.
func (s *Subscriber[T]) Close() error
```

---

## Telemetry

OTel is initialized once via `sync.Once` against OTel globals (`otel.GetTracerProvider()`, `otel.GetMeterProvider()`). Transports call package-level helpers directly -- no constructor argument required.

### InitErr

```go
// InitErr returns any error that occurred during package-level telemetry
// initialisation. Call it once after setting up your OTel providers if you
// want to surface init failures explicitly.
func InitErr() error
```

### ResetForTest

```go
// ResetForTest tears down the singleton so tests can inject a fresh provider.
// Must only be called from test code.
func ResetForTest()
```

### RegisterLag

```go
// RegisterLag registers the messaging.consumer.lag observable gauge for a
// given subject against mp. Called once per Subscriber from its constructor.
// Follows the messaging.* naming pattern; no semconv equivalent exists.
func RegisterLag(mp metric.MeterProvider, subject string, lagFn func() int64) (metric.Int64ObservableGauge, error)
```

### RecordPublish

```go
// RecordPublish opens a producer span, calls fn, records duration and counter.
func RecordPublish(ctx context.Context, subject string, system semconvmsg.SystemAttr, fn func(context.Context) error) error
```

Called by transport `Publish` implementations. Opens a span with `SpanKindProducer`, increments `messaging.client.sent.messages`, and records `messaging.client.operation.duration`.

### RecordProcess

```go
// RecordProcess opens a consumer span, calls fn, records duration and counter.
func RecordProcess(ctx context.Context, subject string, system semconvmsg.SystemAttr, fn func(context.Context) error) error
```

Called by transport `Subscribe` implementations. Opens a span with `SpanKindConsumer`, increments `messaging.client.consumed.messages`, and records `messaging.process.duration`.

### Metrics Summary

| Metric                                  | Type            | Recorded by     |
|-----------------------------------------|-----------------|-----------------|
| `messaging.client.sent.messages`        | Counter         | `RecordPublish` |
| `messaging.client.consumed.messages`    | Counter         | `RecordProcess` |
| `messaging.client.operation.duration`   | Histogram (ms)  | `RecordPublish` |
| `messaging.process.duration`            | Histogram (ms)  | `RecordProcess` |
| `messaging.consumer.lag`               | Gauge           | `RegisterLag`   |

All metrics carry `messaging.destination.name` (subject) and `messaging.system` attributes.

---

## Testing

Package `testing` provides goroutine helpers for test code that needs to launch and synchronize with concurrent operations.

Import: `github.com/foomo/goflux/testing`

### GoSync

```go
// GoSync launches fn in a goroutine and blocks until it completes.
// It is intended for test helpers that need to run a function asynchronously
// but wait for its result before proceeding.
func GoSync(ctx context.Context, fn func(ctx context.Context))
```

### GoSyncE

```go
// GoSyncE launches fn in a goroutine and blocks until it completes,
// returning the error from fn. Cancellation is propagated via
// context.WithCancelCause so the returned error is the original cause,
// not a wrapped context.Canceled.
func GoSyncE(ctx context.Context, fn func(ctx context.Context) error) error
```

### GoAsync

```go
// GoAsync launches fn in a goroutine, waits for it to start, and returns
// a channel that is closed when fn calls done. Use this when the goroutine
// should keep running after launch and the caller needs to wait for an
// explicit completion signal later.
func GoAsync(ctx context.Context, fn func(ctx context.Context, done context.CancelFunc)) <-chan struct{}
```

### GoAsyncE

```go
// GoAsyncE launches fn in a goroutine, waits for it to start, and returns
// a channel that receives the error from fn when it completes. Use this when
// the goroutine should keep running after launch and the caller needs to
// collect both the completion signal and the error later.
func GoAsyncE(ctx context.Context, fn func(ctx context.Context) error) <-chan error
```
