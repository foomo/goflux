# OpenTelemetry

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
| `WithDestinationTemplate(fn)` | none | Maps subjects to low-cardinality templates for span names and metrics, see [Metric Cardinality](#metric-cardinality) |

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

`messagingconv` is `go.opentelemetry.io/otel/semconv/v1.41.0/messagingconv`.

### RecordPublish

```go
func (t *Telemetry) RecordPublish(ctx context.Context, subject string, system messagingconv.SystemAttr, fn func(context.Context) error) error
```

Opens a **producer** span named `send <subject>` (or `send <template>` when a destination template is derived, `SpanKindProducer`), executes `fn`, then records duration and sent-message counter. The `system` attribute identifies the transport (e.g. `"nats"`, `"http"`).

### RecordProcess

```go
func (t *Telemetry) RecordProcess(ctx context.Context, subject string, system messagingconv.SystemAttr, fn func(context.Context) error, opts ...ProcessOption) error
```

Opens a **consumer** span named `process <subject>` (or `process <template>` when a destination template is derived, `SpanKindConsumer`), executes `fn`, then records duration and consumed-message counter. Pass `WithRemoteSpanContext` to attach the producer span as a link for async transports.

### RecordFetch

```go
func (t *Telemetry) RecordFetch(ctx context.Context, subject string, system messagingconv.SystemAttr, count int, fn func(context.Context) error) error
```

Opens a **consumer** span named `receive <subject>` (or `receive <template>` when a destination template is derived, `SpanKindConsumer`) for pull-based fetch operations. Records the batch message count alongside the standard duration and counter metrics.

### RecordRequest

```go
func (t *Telemetry) RecordRequest(ctx context.Context, subject string, system messagingconv.SystemAttr, fn func(context.Context) error) error
```

Opens a **client** span named `send <subject>` (or `send <template>` when a destination template is derived, `SpanKindClient`) for request-reply calls. Records duration and sent-message counter.

### RegisterLag

```go
func (t *Telemetry) RegisterLag(subject string, lagFn func() int64) func()
```

Reports `lagFn` (the number of messages waiting in a subscriber buffer) on the `goflux.consumer.lag` gauge. The lag of all registrations with the same destination is summed, so subjects that share a destination template report one total. Call the returned function when the subscription ends. The channel transport registers it for the subscribed subject in `SubscribeWithReady`.

## Metrics

All metrics follow OpenTelemetry messaging semantic conventions.

| Metric | Kind | Description |
|--------|------|-------------|
| `messaging.client.sent.messages` | Counter | Messages published or requested |
| `messaging.client.consumed.messages` | Counter | Messages consumed or fetched |
| `messaging.client.operation.duration` | Histogram | Publish and request duration (s) |
| `messaging.process.duration` | Histogram | Handler processing and fetch duration (s) |
| `goflux.consumer.lag` | Observable Gauge | Messages waiting in subscriber buffer |
| `goflux.processor.ack.outcome` | Counter | Ack / nak / nak_with_delay / term outcomes per subject |

The `messaging.*` metrics carry `messaging.destination.name` (subject) and `error.type`. `goflux.processor.ack.outcome` carries `goflux.ack.action`, the destination and `goflux.ack.error` (bool). `goflux.consumer.lag` carries only the destination. With `WithDestinationTemplate`, every metric can carry `messaging.destination.template` instead of `messaging.destination.name`.

## Metric Cardinality {#metric-cardinality}

Every distinct subject on a metric creates its own time series. Subjects that embed unbounded values, such as `orders.<id>.created`, therefore create one series per ID.

Consumer metrics record the subject passed to `Subscribe`, not each message's subject. A single wildcard subscription such as `orders.*.created` produces one series. The exception is `goflux.processor.ack.outcome`: `RetryAck` records each message's concrete subject, so bound it with `WithDestinationTemplate` and `RetryAckWithTelemetry`. Publishers only have the concrete subject, so use `WithDestinationTemplate` to map it to a template:

```go
tel, err := goflux.NewTelemetry(
    goflux.WithDestinationTemplate(func(subject string) string {
        parts := strings.Split(subject, ".")
        if len(parts) == 3 && parts[0] == "orders" {
            return "orders.*." + parts[2]
        }
        return "" // keep the concrete subject
    }),
)
```

- A non-empty result is recorded as `messaging.destination.template` in place of `messaging.destination.name`, never both.
- An empty result keeps `messaging.destination.name` for that call.
- Applies to metrics from `RecordPublish`, `RecordProcess`, `RecordFetch`, `RecordRequest`, `RecordAckOutcome` and `RegisterLag`. `goflux.consumer.lag` sums the lag of all subjects that map to the same template.
- `middleware.RetryAck` records ack outcomes through its own `Telemetry`; pass the transport's instance with `middleware.RetryAckWithTelemetry` so the same template applies.
- Also applies to span names: the same template replaces the subject in `{operation} {subject}` when one is derived. Span attributes always keep `messaging.destination.name` (the concrete subject) and additionally set `messaging.destination.template` when a template is derived — never dropping the concrete name.
- `fn` runs on every recorded operation and must be cheap and safe for concurrent use.

## Span Attributes

Spans follow OpenTelemetry messaging semantic conventions and are named `{operation} {subject}` — e.g. `send orders.created`, `process orders.created`, `receive orders.created` — or `{operation} {template}` when `WithDestinationTemplate` derives one, e.g. `send orders.*.created`. The following attributes are set:

| Attribute | Where | Notes |
|-----------|-------|-------|
| `messaging.system` | all spans | Transport id (`nats`, `nats-jetstream`, `http`, `go_channel`) |
| `messaging.destination.name` | all spans | Subject, always the concrete value |
| `messaging.destination.template` | when a destination template is derived | Low-cardinality template; set alongside, not instead of, `messaging.destination.name` |
| `messaging.operation.name` | all spans | `send` / `process` / `receive` |
| `messaging.operation.type` | transport publish & process spans | `send` / `process` |
| `messaging.message.body.size` | transport spans | Encoded payload size (bytes) |
| `messaging.message.id` | when `WithMessageID` is set | Business-level message ID |
| `messaging.batch.message_count` | fetch spans | Pull batch size |
| `messaging.consumer.group.name` | NATS / JetStream consumer spans | Queue group (NATS) or durable/consumer name (JetStream) |
| `goflux.reply.body.size` | request-reply legs | Reply payload size (bytes); a dedicated key so it does not overwrite the request's `messaging.message.body.size` |
| `goflux.reply.subject` | NATS requester | Subject the reply was received on |
| `goflux.ack.action` | process spans with `middleware.RetryAck` | `ack` / `nak` / `nak_with_delay` / `term` |
| `goflux.ack.delivery_count` | process spans with `middleware.RetryAck`, failed messages | Transport-reported delivery count; omitted when the transport does not report one |

## Context Propagation

goflux provides three propagation methods on `Telemetry` for different transport semantics.

### InjectContext

```go
func (t *Telemetry) InjectContext(ctx context.Context, carrier propagation.TextMapCarrier)
```

Called on the **publish side** to inject the current span context into transport headers (NATS headers, HTTP headers). The NATS, JetStream and HTTP transports call this automatically during `Publish`. The in-process channel transport has no wire headers: it passes the publish span context with the message and links the process span to it.

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

goflux supports an opt-in business-level message ID that identifies one message. It is sent with the message and attached to spans.

```go
// Set before publishing
ctx = goflux.WithMessageID(ctx, "order-42")
pub.Publish(ctx, "orders.created", event)

// Read in a handler
id := msg.ID // "order-42"
```

- Sent via the `X-Message-ID` header (`goflux.MessageIDHeader`); the channel transport passes it with the message
- Attached as `messaging.message.id` span attribute on publish and process spans
- Purely opt-in: if not set, no header or attribute is added
- Not inherited: subscribers expose the received ID as `msg.ID` and never put it into the handler context, so messages published from the handler carry no ID unless the handler sets one. To publish with the same ID, use [`middleware.ForwardMessageID`](/middleware/#forwardmessageid). Request-reply responders record the request ID on the process span only.
- Correlation across messages comes from trace context, not from the message ID

## Transport System Attributes

Each transport declares a system attribute used in metrics and spans:

| Transport | System Attribute |
|-----------|-----------------|
| `transport/channel` | `go_channel` |
| `transport/nats` | `nats` |
| `transport/jetstream` | `nats-jetstream` |
| `transport/http` | `http` |
