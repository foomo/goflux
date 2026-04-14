# Telemetry

OpenTelemetry instrumentation is built into goflux transports, not added as middleware. Each transport creates a `Telemetry` instance and calls its recording methods around publish and process operations.

## Telemetry Struct

```go
func NewTelemetry(opts ...TelemetryOption) (*Telemetry, error)
```

Creates a `Telemetry` instance that holds an OTel tracer, meter, and propagator. Without options it reads from the current OTel globals, so callers that have already called `otel.SetTracerProvider` / `otel.SetMeterProvider` need not pass anything.

### Options

| Option | Default | Description |
|--------|---------|-------------|
| `WithTracerProvider(tp)` | `otel.GetTracerProvider()` | Sets the tracer provider |
| `WithMeterProvider(mp)` | `otel.GetMeterProvider()` | Sets the meter provider |
| `WithPropagator(p)` | `otel.GetTextMapPropagator()` | Sets the text-map propagator |

```go
tel, err := goflux.NewTelemetry(
    goflux.WithTracerProvider(tp),
    goflux.WithMeterProvider(mp),
)
if err != nil {
    return err
}
```

## Recording Methods

### RecordPublish

```go
func (t *Telemetry) RecordPublish(ctx context.Context, subject string, system SystemAttr, fn func(context.Context) error) error
```

Opens a **producer** span (`SpanKindProducer`), executes `fn`, then records duration and sent-message counter. The `system` attribute identifies the transport (e.g. `"nats"`, `"http"`).

### RecordProcess

```go
func (t *Telemetry) RecordProcess(ctx context.Context, subject string, system SystemAttr, fn func(context.Context) error, opts ...ProcessOption) error
```

Opens a **consumer** span (`SpanKindConsumer`), executes `fn`, then records duration and consumed-message counter. Pass `WithRemoteSpanContext` to attach the producer span as a link for async transports.

### RecordFetch

```go
func (t *Telemetry) RecordFetch(ctx context.Context, subject string, system SystemAttr, count int, fn func(context.Context) error) error
```

Opens a **consumer** span for pull-based fetch operations. Records the batch message count alongside the standard duration and counter metrics.

### RecordRequest

```go
func (t *Telemetry) RecordRequest(ctx context.Context, subject string, system SystemAttr, fn func(context.Context) error) error
```

Opens a **client** span (`SpanKindClient`) for request-reply calls. Records duration and sent-message counter.

### RegisterLag

```go
func (t *Telemetry) RegisterLag(subject string, lagFn func() int64) (metric.Int64ObservableGauge, error)
```

Registers an observable gauge that periodically reports the number of messages waiting in a subscriber buffer.

## Metrics

All metrics follow OpenTelemetry messaging semantic conventions.

| Metric | Kind | Description |
|--------|------|-------------|
| `goflux.client.sent.messages` | Counter | Messages published |
| `goflux.client.consumed.messages` | Counter | Messages consumed |
| `goflux.client.operation.duration` | Histogram | Publish operation duration (ms) |
| `goflux.process.duration` | Histogram | Handler processing duration (ms) |
| `goflux.consumer.lag` | Observable Gauge | Messages waiting in subscriber buffer |

Each metric carries `destination.name` (subject) and `error.type` attributes.

## Context Propagation

goflux provides three propagation methods on `Telemetry` for different transport semantics.

### InjectContext

```go
func (t *Telemetry) InjectContext(ctx context.Context, carrier propagation.TextMapCarrier)
```

Called on the **publish side** to inject the current span context into transport headers (NATS headers, HTTP headers). All transports call this automatically during `Publish`.

### ExtractContext (synchronous transports)

```go
func (t *Telemetry) ExtractContext(ctx context.Context, carrier propagation.TextMapCarrier) context.Context
```

Extracts the remote span context from the carrier and returns a context with the remote span as **parent**. Used by synchronous transports like HTTP where parent-child relationship is appropriate.

### ExtractSpanContext (asynchronous transports)

```go
func (t *Telemetry) ExtractSpanContext(ctx context.Context, carrier propagation.TextMapCarrier) trace.SpanContext
```

Extracts the remote span context **without** injecting it as parent. Returns the `SpanContext` to be used with `WithRemoteSpanContext` as a span **link**. Used by NATS and JetStream where the consumer is temporally decoupled from the producer.

### Span Links vs Parent-Child

Asynchronous messaging transports (NATS, JetStream) use **span links** to correlate producer and consumer spans. The consumer span becomes a root span linked to the producer, rather than a child of it. This is the correct approach because the consumer may run seconds, minutes, or hours after the producer, making a parent-child relationship misleading for latency analysis.

Synchronous transports (HTTP) use **parent-child** relationships because the producer blocks until the consumer responds.

```go
// Async transport (NATS/JetStream) -- extract as link
sc := tel.ExtractSpanContext(ctx, carrier)
err := tel.RecordProcess(ctx, subject, system, handler,
    goflux.WithRemoteSpanContext(sc),
)

// Sync transport (HTTP) -- extract as parent
ctx = tel.ExtractContext(ctx, carrier)
err := tel.RecordProcess(ctx, subject, system, handler)
```

## Message ID {#message-id}

goflux supports an opt-in business-level message ID that is propagated across transports and attached to spans.

```go
// Set before publishing
ctx = goflux.WithMessageID(ctx, "order-42")

// Read in a handler
id := goflux.MessageID(ctx)
```

- Propagated via the `X-Message-ID` header (`goflux.MessageIDHeader`)
- Attached as `goflux.message.id` span attribute on publish and process spans
- Purely opt-in: if not set, no header or attribute is added

## Transport System Attributes

Each transport declares a system attribute used in metrics and spans:

| Transport | System Attribute |
|-----------|-----------------|
| `pkg/channel` | `go_channel` |
| `pkg/nats` | `nats` |
| `pkg/jetstream` | `nats-jetstream` |
| `pkg/http` | `http` |
