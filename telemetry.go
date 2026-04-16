package goflux

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	metricnoop "go.opentelemetry.io/otel/metric/noop"
	"go.opentelemetry.io/otel/propagation"
	semconvmsg "go.opentelemetry.io/otel/semconv/v1.40.0/messagingconv"
	"go.opentelemetry.io/otel/trace"
	tracenoop "go.opentelemetry.io/otel/trace/noop"
)

const instrName = "github.com/foomo/goflux"

// ---------------------------------------------------------------------------
// Telemetry — constructible, per-instance OTel instrumentation
// ---------------------------------------------------------------------------

// Telemetry holds OTel instruments (tracer, metrics, propagator) for a single
// transport instance. Construct with [NewTelemetry].
type Telemetry struct {
	tracer     trace.Tracer
	propagator propagation.TextMapPropagator
	mp         metric.MeterProvider

	// semconv-defined metrics
	sentMessages     semconvmsg.ClientSentMessages      // goflux.client.sent.messages
	consumedMessages semconvmsg.ClientConsumedMessages  // goflux.client.consumed.messages
	publishDuration  semconvmsg.ClientOperationDuration // goflux.client.operation.duration
	processDuration  semconvmsg.ProcessDuration         // goflux.process.duration

	// goflux-specific metrics
	ackOutcome metric.Int64Counter // goflux.processor.ack.outcome
}

// ---------------------------------------------------------------------------
// Options
// ---------------------------------------------------------------------------

type telemetryConfig struct {
	tp         trace.TracerProvider
	mp         metric.MeterProvider
	propagator propagation.TextMapPropagator
}

// TelemetryOption configures a [Telemetry] instance.
type TelemetryOption func(*telemetryConfig)

// WithTracerProvider sets the tracer provider. Defaults to [otel.GetTracerProvider].
func WithTracerProvider(tp trace.TracerProvider) TelemetryOption {
	return func(c *telemetryConfig) { c.tp = tp }
}

// WithMeterProvider sets the meter provider. Defaults to [otel.GetMeterProvider].
func WithMeterProvider(mp metric.MeterProvider) TelemetryOption {
	return func(c *telemetryConfig) { c.mp = mp }
}

// WithPropagator sets the text-map propagator. Defaults to [otel.GetTextMapPropagator].
func WithPropagator(p propagation.TextMapPropagator) TelemetryOption {
	return func(c *telemetryConfig) { c.propagator = p }
}

// DefaultTelemetry returns tel if non-nil, otherwise creates a new Telemetry
// from OTel globals. If that fails, it falls back to a noop implementation.
// This is the standard fallback logic used by all transports.
func DefaultTelemetry(tel *Telemetry) *Telemetry {
	if tel != nil {
		return tel
	}

	t, err := NewTelemetry()
	if err != nil {
		otel.Handle(err)

		return NewNoopTelemetry()
	}

	return t
}

// NewNoopTelemetry returns a Telemetry backed by OTel's noop implementations.
// All Record* calls are safe but produce no spans or metrics.
func NewNoopTelemetry() *Telemetry {
	mp := metricnoop.NewMeterProvider()
	m := mp.Meter(instrName)

	// Noop meter never returns errors.
	sent, _ := semconvmsg.NewClientSentMessages(m)
	consumed, _ := semconvmsg.NewClientConsumedMessages(m)
	pubDur, _ := semconvmsg.NewClientOperationDuration(m)
	procDur, _ := semconvmsg.NewProcessDuration(m)

	ackOutcome, _ := m.Int64Counter("goflux.processor.ack.outcome")

	return &Telemetry{
		tracer:           tracenoop.NewTracerProvider().Tracer(instrName),
		propagator:       propagation.NewCompositeTextMapPropagator(),
		mp:               mp,
		sentMessages:     sent,
		consumedMessages: consumed,
		publishDuration:  pubDur,
		processDuration:  procDur,
		ackOutcome:       ackOutcome,
	}
}

// NewTelemetry creates a Telemetry instance. Without options it reads from the
// current OTel globals, so callers that have already called
// [otel.SetTracerProvider] / [otel.SetMeterProvider] need not pass anything.
func NewTelemetry(opts ...TelemetryOption) (*Telemetry, error) {
	cfg := &telemetryConfig{
		tp:         otel.GetTracerProvider(),
		mp:         otel.GetMeterProvider(),
		propagator: otel.GetTextMapPropagator(),
	}
	for _, o := range opts {
		o(cfg)
	}

	m := cfg.mp.Meter(instrName)
	t := &Telemetry{
		tracer:     cfg.tp.Tracer(instrName),
		propagator: cfg.propagator,
		mp:         cfg.mp,
	}

	var err error

	t.sentMessages, err = semconvmsg.NewClientSentMessages(m)
	if err != nil {
		return nil, fmt.Errorf("messaging telemetry: sent messages: %w", err)
	}

	t.consumedMessages, err = semconvmsg.NewClientConsumedMessages(m)
	if err != nil {
		return nil, fmt.Errorf("messaging telemetry: consumed messages: %w", err)
	}

	t.publishDuration, err = semconvmsg.NewClientOperationDuration(m)
	if err != nil {
		return nil, fmt.Errorf("messaging telemetry: publish duration: %w", err)
	}

	t.processDuration, err = semconvmsg.NewProcessDuration(m)
	if err != nil {
		return nil, fmt.Errorf("messaging telemetry: process duration: %w", err)
	}

	t.ackOutcome, err = m.Int64Counter("goflux.processor.ack.outcome",
		metric.WithDescription("Number of message acknowledgment outcomes by action"),
	)
	if err != nil {
		return nil, fmt.Errorf("messaging telemetry: ack outcome: %w", err)
	}

	return t, nil
}

// ---------------------------------------------------------------------------
// Recording methods
// ---------------------------------------------------------------------------

// RecordPublish opens a producer span, calls fn, records duration and counter.
func (t *Telemetry) RecordPublish(ctx context.Context, subject string, system semconvmsg.SystemAttr, fn func(context.Context) error) error {
	attrs := []attribute.KeyValue{
		semconvmsg.ClientSentMessages{}.AttrDestinationName(subject),
	}
	if id := MessageID(ctx); id != "" {
		attrs = append(attrs, attribute.String("goflux.message.id", id))
	}

	ctx, span := t.tracer.Start(ctx, subject+" publish",
		trace.WithSpanKind(trace.SpanKindProducer),
		trace.WithAttributes(attrs...),
	)
	defer span.End()

	start := time.Now()
	err := fn(ctx)
	s := secondsSince(start)

	errType := errorType(err)
	t.sentMessages.Add(ctx, 1,
		"publish",
		system,
		t.sentMessages.AttrDestinationName(subject),
		t.sentMessages.AttrErrorType(errType),
	)
	t.publishDuration.Record(ctx, s,
		"publish",
		system,
		t.publishDuration.AttrDestinationName(subject),
		t.publishDuration.AttrErrorType(errType),
	)
	recordSpanResult(span, err)

	return err
}

// ProcessOption configures [Telemetry.RecordProcess].
type ProcessOption func(*processConfig)

type processConfig struct {
	linkedSpanCtx trace.SpanContext
}

// WithRemoteSpanContext attaches the given span context as a span link instead
// of using it as the parent. Use this for async transports (e.g. NATS) where
// the producer and consumer are temporally decoupled — the consumer span
// becomes a root span linked to the producer, rather than a child of it.
func WithRemoteSpanContext(sc trace.SpanContext) ProcessOption {
	return func(c *processConfig) { c.linkedSpanCtx = sc }
}

// RecordProcess opens a consumer span, calls fn, records duration and counter.
// Pass [WithRemoteSpanContext] to attach the producer span as a link rather
// than a parent (recommended for async transports like NATS).
func (t *Telemetry) RecordProcess(ctx context.Context, subject string, system semconvmsg.SystemAttr, fn func(context.Context) error, opts ...ProcessOption) error {
	var cfg processConfig
	for _, o := range opts {
		o(&cfg)
	}

	attrs := []attribute.KeyValue{
		semconvmsg.ClientConsumedMessages{}.AttrDestinationName(subject),
	}
	if id := MessageID(ctx); id != "" {
		attrs = append(attrs, attribute.String("goflux.message.id", id))
	}

	startOpts := []trace.SpanStartOption{
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(attrs...),
	}
	if cfg.linkedSpanCtx.IsValid() {
		startOpts = append(startOpts, trace.WithLinks(trace.Link{SpanContext: cfg.linkedSpanCtx}))
	}

	ctx, span := t.tracer.Start(ctx, subject+" process", startOpts...)
	defer span.End()

	start := time.Now()
	err := fn(ctx)
	s := secondsSince(start)

	errType := errorType(err)
	t.consumedMessages.Add(ctx, 1,
		"receive",
		system,
		t.consumedMessages.AttrDestinationName(subject),
		t.consumedMessages.AttrErrorType(errType),
	)
	t.processDuration.Record(ctx, s,
		"process",
		system,
		t.processDuration.AttrDestinationName(subject),
		t.processDuration.AttrErrorType(errType),
	)
	recordSpanResult(span, err)

	return err
}

// RecordFetch opens a consumer span for a pull-based fetch operation.
func (t *Telemetry) RecordFetch(ctx context.Context, subject string, system semconvmsg.SystemAttr, count int, fn func(context.Context) error) error {
	attrs := []attribute.KeyValue{
		semconvmsg.ClientConsumedMessages{}.AttrDestinationName(subject),
		attribute.Int("messaging.batch.message_count", count),
	}
	if id := MessageID(ctx); id != "" {
		attrs = append(attrs, attribute.String("goflux.message.id", id))
	}

	ctx, span := t.tracer.Start(ctx, subject+" fetch",
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(attrs...),
	)
	defer span.End()

	start := time.Now()
	err := fn(ctx)
	s := secondsSince(start)

	errType := errorType(err)
	t.consumedMessages.Add(ctx, int64(count),
		"receive",
		system,
		t.consumedMessages.AttrDestinationName(subject),
		t.consumedMessages.AttrErrorType(errType),
	)
	t.processDuration.Record(ctx, s,
		"process",
		system,
		t.processDuration.AttrDestinationName(subject),
		t.processDuration.AttrErrorType(errType),
	)
	recordSpanResult(span, err)

	return err
}

// RecordRequest opens a client span for a request-reply call.
func (t *Telemetry) RecordRequest(ctx context.Context, subject string, system semconvmsg.SystemAttr, fn func(context.Context) error) error {
	attrs := []attribute.KeyValue{
		semconvmsg.ClientSentMessages{}.AttrDestinationName(subject),
	}
	if id := MessageID(ctx); id != "" {
		attrs = append(attrs, attribute.String("goflux.message.id", id))
	}

	ctx, span := t.tracer.Start(ctx, subject+" request",
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)
	defer span.End()

	start := time.Now()
	err := fn(ctx)
	s := secondsSince(start)

	errType := errorType(err)
	t.sentMessages.Add(ctx, 1,
		"publish",
		system,
		t.sentMessages.AttrDestinationName(subject),
		t.sentMessages.AttrErrorType(errType),
	)
	t.publishDuration.Record(ctx, s,
		"publish",
		system,
		t.publishDuration.AttrDestinationName(subject),
		t.publishDuration.AttrErrorType(errType),
	)
	recordSpanResult(span, err)

	return err
}

// RegisterLag registers the goflux.consumer.lag observable gauge.
// Uses the meter provider that was passed to [NewTelemetry].
func (t *Telemetry) RegisterLag(subject string, lagFn func() int64) (metric.Int64ObservableGauge, error) {
	return t.mp.Meter(instrName).Int64ObservableGauge(
		"goflux.consumer.lag",
		metric.WithDescription("Number of messages waiting in the subscriber buffer"),
		metric.WithInt64Callback(func(_ context.Context, obs metric.Int64Observer) error {
			obs.Observe(lagFn(),
				metric.WithAttributes(attribute.String("goflux.destination.name", subject)),
			)

			return nil
		}),
	)
}

// RecordAckOutcome records an acknowledgment outcome (ack, nak, nak_with_delay,
// term) with an optional error label when the ack operation itself fails.
func (t *Telemetry) RecordAckOutcome(ctx context.Context, action, subject string, err error) {
	attrs := metric.WithAttributes(
		attribute.String("goflux.ack.action", action),
		attribute.String("goflux.destination.name", subject),
		attribute.Bool("goflux.ack.error", err != nil),
	)
	t.ackOutcome.Add(ctx, 1, attrs)
}

// ---------------------------------------------------------------------------
// Propagation methods
// ---------------------------------------------------------------------------

// InjectContext injects the span context from ctx into the carrier.
// Transports call this on the publish side to propagate trace context
// across wire boundaries (e.g. NATS headers, HTTP headers).
func (t *Telemetry) InjectContext(ctx context.Context, carrier propagation.TextMapCarrier) {
	t.propagator.Inject(ctx, carrier)
}

// ExtractContext extracts span context from carrier and returns an enriched
// context with the remote span as parent. Use this for synchronous transports
// (e.g. HTTP) where parent-child relationship is appropriate.
func (t *Telemetry) ExtractContext(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	return t.propagator.Extract(ctx, carrier)
}

// ExtractSpanContext extracts the remote span context from carrier without
// injecting it as parent into ctx. Use this with [WithRemoteSpanContext] for
// async transports where the consumer span should link to (not be a child of)
// the producer span.
func (t *Telemetry) ExtractSpanContext(ctx context.Context, carrier propagation.TextMapCarrier) trace.SpanContext {
	return trace.SpanContextFromContext(t.propagator.Extract(ctx, carrier))
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func secondsSince(start time.Time) float64 {
	return time.Since(start).Seconds()
}

func errorType(err error) semconvmsg.ErrorTypeAttr {
	if err == nil {
		return semconvmsg.ErrorTypeAttr("")
	}

	return semconvmsg.ErrorTypeAttr("error")
}

func recordSpanResult(span trace.Span, err error) {
	if span == nil {
		return
	}

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "")
	}
}
