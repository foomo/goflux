package goflux_test

import (
	"context"
	"errors"
	"testing"

	"github.com/foomo/goflux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconvmsg "go.opentelemetry.io/otel/semconv/v1.40.0/messagingconv"
	"go.opentelemetry.io/otel/trace"
)

func setupTelemetry(t *testing.T) (*goflux.Telemetry, *tracetest.InMemoryExporter, *metric.ManualReader) {
	t.Helper()

	spanExporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(spanExporter))

	metricReader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(metricReader))

	tel, err := goflux.NewTelemetry(
		goflux.WithTracerProvider(tp),
		goflux.WithMeterProvider(mp),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = tp.Shutdown(context.Background())
		_ = mp.Shutdown(context.Background())
	})

	return tel, spanExporter, metricReader
}

func TestRecordPublish_SpanAndMetrics(t *testing.T) {
	tel, spanExporter, metricReader := setupTelemetry(t)

	ctx := context.Background()
	ctx = goflux.WithMessageID(ctx, "msg-123")

	err := tel.RecordPublish(ctx, "orders.created", semconvmsg.SystemAttr("test"), func(ctx context.Context) error {
		return nil
	})
	require.NoError(t, err)

	// Verify span
	spans := spanExporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "orders.created publish", spans[0].Name)
	assert.Equal(t, trace.SpanKindProducer, spans[0].SpanKind)

	// Verify span has message ID attribute
	found := false

	for _, attr := range spans[0].Attributes {
		if string(attr.Key) == "goflux.message.id" {
			assert.Equal(t, "msg-123", attr.Value.AsString())

			found = true
		}
	}

	assert.True(t, found, "expected goflux.message.id attribute on span")

	// Verify metrics
	var rm metricdata.ResourceMetrics
	require.NoError(t, metricReader.Collect(ctx, &rm))

	metricNames := make(map[string]bool)

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			metricNames[m.Name] = true
		}
	}

	assert.True(t, metricNames["messaging.client.sent.messages"], "expected sent messages counter")
	assert.True(t, metricNames["messaging.client.operation.duration"], "expected operation duration histogram")
}

func TestRecordPublish_Error(t *testing.T) {
	tel, spanExporter, _ := setupTelemetry(t)

	testErr := errors.New("publish failed")

	err := tel.RecordPublish(context.Background(), "orders.created", semconvmsg.SystemAttr("test"), func(ctx context.Context) error {
		return testErr
	})
	require.ErrorIs(t, err, testErr)

	spans := spanExporter.GetSpans()
	require.Len(t, spans, 1)

	// Span should have error status
	assert.NotEmpty(t, spans[0].Events, "expected error event on span")
}

func TestRecordProcess_SpanKindConsumer(t *testing.T) {
	tel, spanExporter, metricReader := setupTelemetry(t)
	ctx := context.Background()

	err := tel.RecordProcess(ctx, "orders.created", semconvmsg.SystemAttr("test"), func(ctx context.Context) error {
		return nil
	})
	require.NoError(t, err)

	spans := spanExporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "orders.created process", spans[0].Name)
	assert.Equal(t, trace.SpanKindConsumer, spans[0].SpanKind)

	// Verify consumed messages metric
	var rm metricdata.ResourceMetrics
	require.NoError(t, metricReader.Collect(ctx, &rm))

	metricNames := make(map[string]bool)

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			metricNames[m.Name] = true
		}
	}

	assert.True(t, metricNames["messaging.client.consumed.messages"], "expected consumed messages counter")
	assert.True(t, metricNames["messaging.process.duration"], "expected process duration histogram")
}

func TestRecordProcess_WithSpanLink(t *testing.T) {
	tel, spanExporter, _ := setupTelemetry(t)
	ctx := context.Background()

	// Create a fake remote span context to link to
	remoteSpanCtx := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    trace.TraceID{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		SpanID:     trace.SpanID{1, 2, 3, 4, 5, 6, 7, 8},
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})

	err := tel.RecordProcess(ctx, "events.stream", semconvmsg.SystemAttr("test"), func(ctx context.Context) error {
		return nil
	}, goflux.WithRemoteSpanContext(remoteSpanCtx))
	require.NoError(t, err)

	spans := spanExporter.GetSpans()
	require.Len(t, spans, 1)

	// Verify the span has a link to the remote span context
	assert.Len(t, spans[0].Links, 1, "expected one span link for async consumer")
	assert.Equal(t, remoteSpanCtx.TraceID(), spans[0].Links[0].SpanContext.TraceID())
	assert.Equal(t, remoteSpanCtx.SpanID(), spans[0].Links[0].SpanContext.SpanID())
}

func TestRecordAckOutcome(t *testing.T) {
	tel, _, metricReader := setupTelemetry(t)
	ctx := context.Background()

	tel.RecordAckOutcome(ctx, "ack", "orders.created", nil)
	tel.RecordAckOutcome(ctx, "nak", "orders.created", errors.New("nak failed"))

	var rm metricdata.ResourceMetrics
	require.NoError(t, metricReader.Collect(ctx, &rm))

	found := false

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name == "goflux.processor.ack.outcome" {
				found = true
			}
		}
	}

	assert.True(t, found, "expected goflux.processor.ack.outcome metric")
}

func TestNewNoopTelemetry_SafeToUse(t *testing.T) {
	tel := goflux.NewNoopTelemetry()
	ctx := context.Background()

	// All Record* calls should be safe with noop telemetry
	err := tel.RecordPublish(ctx, "test", semconvmsg.SystemAttr("test"), func(ctx context.Context) error {
		return nil
	})
	require.NoError(t, err)

	err = tel.RecordProcess(ctx, "test", semconvmsg.SystemAttr("test"), func(ctx context.Context) error {
		return nil
	})
	require.NoError(t, err)

	err = tel.RecordFetch(ctx, "test", semconvmsg.SystemAttr("test"), 5, func(ctx context.Context) error {
		return nil
	})
	require.NoError(t, err)

	err = tel.RecordRequest(ctx, "test", semconvmsg.SystemAttr("test"), func(ctx context.Context) error {
		return nil
	})
	require.NoError(t, err)

	tel.RecordAckOutcome(ctx, "ack", "test", nil)
}
