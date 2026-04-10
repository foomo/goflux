# Telemetry

goflux has OpenTelemetry built into every transport. Tracing and metrics are not middleware -- they are part of the transport layer and activate automatically when OTel providers are configured.

## Architecture

Telemetry is implemented as a package-level singleton, initialised once via `sync.Once` against the OTel global providers (`otel.GetTracerProvider()` and `otel.GetMeterProvider()`). Transports call the internal `tel()` function on first use, which triggers initialisation.

This means:

1. Set up your OTel providers (e.g. via [keel](https://github.com/foomo/keel)) **before** any publish or subscribe call.
2. Transports automatically pick up the configured providers.
3. No constructor arguments or configuration flags are needed.

## Setup

If you use keel, OTel providers are set up for you:

```go
svr := keel.NewServer(
    keel.WithTracerProvider(tp),
    keel.WithMeterProvider(mp),
)
// From this point, all goflux transports emit traces and metrics.
```

Without keel, set the global providers directly:

```go
otel.SetTracerProvider(tp)
otel.SetMeterProvider(mp)

// Now publish/subscribe -- telemetry is active.
pub.Publish(ctx, "orders", event)
```

## InitErr

Call `InitErr()` after setting up providers to surface any initialisation failures:

```go
func InitErr() error
```

```go
otel.SetMeterProvider(mp)

if err := goflux.InitErr(); err != nil {
    log.Fatal("telemetry init failed:", err)
}
```

## ResetForTest

Reset the singleton for test isolation:

```go
func ResetForTest()
```

```go
func TestSomething(t *testing.T) {
    goflux.ResetForTest()
    // Set up test providers...
}
```

::: warning
`ResetForTest` must only be called from test code. It is not safe for concurrent use.
:::

## Metrics

All metrics follow the `messaging.*` naming convention and use OpenTelemetry semantic conventions (semconv v1.40.0).

| Metric | Type | Description |
|--------|------|-------------|
| `messaging.client.sent.messages` | Counter | Number of messages published |
| `messaging.client.consumed.messages` | Counter | Number of messages consumed |
| `messaging.client.operation.duration` | Histogram (ms) | Duration of publish operations |
| `messaging.process.duration` | Histogram (ms) | Duration of handler invocations |
| `messaging.consumer.lag` | Gauge | Number of messages waiting in the subscriber buffer |

### Metric Attributes

All metrics (except `messaging.consumer.lag`) carry the following attributes:

| Attribute | Description | Example |
|-----------|-------------|---------|
| `messaging.destination.name` | The subject/topic name | `"orders.created"` |
| `messaging.operation` | The operation type | `"publish"`, `"receive"`, `"process"` |
| `messaging.system` | The transport system | `"go_channel"`, `"nats"`, `"http"` |
| `error.type` | Error type (empty on success) | `"*errors.errorString"` |

### Span Attributes

In addition to the metric attributes above, spans carry:

| Attribute | Description | When |
|-----------|-------------|------|
| `messaging.message.id` | Application-level message ID | When `WithMessageID` is used |

The `messaging.consumer.lag` gauge carries only `messaging.destination.name`.

## Tracing

Every publish and process operation creates a span:

| Span Name | Span Kind | When |
|-----------|-----------|------|
| `messaging.publish` | `SpanKindProducer` | On each `Publisher.Publish` call |
| `messaging.process` | `SpanKindConsumer` | On each handler invocation |

Spans carry `messaging.destination.name` as an attribute. On error, the span records the error and sets status to `codes.Error`.

### Distributed Trace Propagation

Transports that cross process boundaries (NATS and HTTP) automatically propagate OTel trace context. The publisher injects the current span context into transport headers (NATS message headers / HTTP request headers), and the subscriber extracts it on the other side. This creates proper parent-child span relationships across services without any manual wiring.

The propagation uses whichever `TextMapPropagator` is registered globally via `otel.SetTextMapPropagator()`. By default this is W3C Trace Context (`traceparent` / `tracestate` headers).

The `chan/` transport does not need propagation because publish and subscribe happen in the same process and share the same trace provider.

### Message ID

goflux supports an optional, application-level message ID for business correlation. When set on the context, it is:

1. Attached to publish and process spans as the `messaging.message.id` attribute.
2. Propagated via transport headers (`X-Message-ID` for HTTP, a NATS header of the same name).

```go
// Attach a message ID before publishing.
ctx = goflux.WithMessageID(ctx, "order-42-abc123")
pub.Publish(ctx, "orders.created", event)
```

On the subscriber side, the message ID is automatically extracted from transport headers and placed on the handler's context:

```go
handler := func(ctx context.Context, msg goflux.Message[OrderEvent]) error {
    id := goflux.MessageID(ctx) // "order-42-abc123"
    log.Info("processing", "message_id", id)
    return nil
}
```

Message IDs are purely opt-in. If you do not call `WithMessageID`, no header is sent and no span attribute is added.

## RecordPublish and RecordProcess

These are the internal functions that transports call to emit telemetry. They are exported so that [custom transports](./transports.md#writing-a-custom-transport) can participate in the same telemetry:

```go
func RecordPublish(ctx context.Context, subject string, system semconvmsg.SystemAttr, fn func(context.Context) error) error

func RecordProcess(ctx context.Context, subject string, system semconvmsg.SystemAttr, fn func(context.Context) error) error
```

Both functions:

1. Start a trace span (producer or consumer).
2. Call `fn` with the span's context.
3. Record duration and counter metrics.
4. Record span status (ok or error).

If the telemetry singleton failed to initialise (nil), the functions still call `fn` -- your transport works without telemetry, just without spans and metrics.

## RegisterLag

`RegisterLag` creates the `messaging.consumer.lag` observable gauge for a subscriber:

```go
func RegisterLag(mp metric.MeterProvider, subject string, lagFn func() int64) (metric.Int64ObservableGauge, error)
```

The `lagFn` callback is invoked by the OTel SDK on each collection interval, returning the current buffer depth. The `chan/` transport calls this in `NewSubscriber`, using `Subscriber.Len()` as the lag function.

## System Attribute Values

Each transport declares its `messaging.system` attribute value:

| Transport | System Value |
|-----------|-------------|
| `chan/` | `go_channel` |
| `nats/` | `nats` |
| `http/` | `http` |

Custom transports must declare their own system variable:

```go
import semconvmsg "go.opentelemetry.io/otel/semconv/v1.40.0/messagingconv"

var system = semconvmsg.SystemAttr("my_transport")
```

## For Custom Transports

To ensure your custom transport participates in goflux telemetry:

1. Declare a `system` variable as shown above.
2. Wrap your publish logic with `goflux.RecordPublish`.
3. Wrap your handler dispatch with `goflux.RecordProcess`.
4. Optionally call `goflux.RegisterLag` if your subscriber has a buffer with observable depth.

See [Writing a Custom Transport](./transports.md#writing-a-custom-transport) for a full example.

## What's Next

- [Transports](./transports.md) -- see how each transport integrates with telemetry
- [Core Concepts](./core-concepts.md) -- review the fundamental types
- [Distribution](./distribution.md) -- fan-out, fan-in, and round-robin patterns
