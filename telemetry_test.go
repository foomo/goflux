package goflux_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/foomo/goflux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconvmsg "go.opentelemetry.io/otel/semconv/v1.41.0/messagingconv"
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

	err := tel.RecordPublish(ctx, "orders.created", "test", func(ctx context.Context) error {
		return nil
	})
	require.NoError(t, err)

	// Verify span
	spans := spanExporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "send orders.created", spans[0].Name)
	assert.Equal(t, trace.SpanKindProducer, spans[0].SpanKind)

	// Verify span has message ID attribute
	found := false

	for _, attr := range spans[0].Attributes {
		if string(attr.Key) == "messaging.message.id" {
			assert.Equal(t, "msg-123", attr.Value.AsString())

			found = true
		}
	}

	assert.True(t, found, "expected messaging.message.id attribute on span")

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

	err := tel.RecordProcess(ctx, "orders.created", "test", func(ctx context.Context) error {
		return nil
	})
	require.NoError(t, err)

	spans := spanExporter.GetSpans()
	require.Len(t, spans, 1)
	assert.Equal(t, "process orders.created", spans[0].Name)
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

// orderTemplate maps "orders.<id>.<event>" to "orders.*.<event>".
func orderTemplate(subject string) string {
	parts := strings.Split(subject, ".")
	if len(parts) != 3 || parts[0] != "orders" {
		return ""
	}

	return "orders.*." + parts[2]
}

// TestWithDestinationTemplate_CollapsesConcreteSubjects checks that subjects
// differing only by ID share one metric attribute set and the IDs never reach
// a metric point.
func TestWithDestinationTemplate_CollapsesConcreteSubjects(t *testing.T) {
	metricReader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(metricReader))

	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })

	tel, err := goflux.NewTelemetry(
		goflux.WithMeterProvider(mp),
		goflux.WithDestinationTemplate(orderTemplate),
	)
	require.NoError(t, err)

	ctx := context.Background()
	ids := []string{"id-1001", "id-1002"}

	for _, id := range ids {
		subject := "orders." + id + ".created"

		require.NoError(t, tel.RecordPublish(ctx, subject, "test", func(context.Context) error { return nil }))
		require.NoError(t, tel.RecordProcess(ctx, subject, "test", func(context.Context) error { return nil }))
	}

	var rm metricdata.ResourceMetrics
	require.NoError(t, metricReader.Collect(ctx, &rm))

	attrSets := make(map[string]map[string]struct{}) // metric name -> set of attribute-set fingerprints

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			for _, fp := range dataPointAttrFingerprints(t, m) {
				if attrSets[m.Name] == nil {
					attrSets[m.Name] = make(map[string]struct{})
				}

				attrSets[m.Name][fp] = struct{}{}
			}
		}
	}

	for _, name := range []string{"messaging.client.sent.messages", "messaging.client.operation.duration", "messaging.client.consumed.messages", "messaging.process.duration"} {
		fps := attrSets[name]
		require.NotEmpty(t, fps, "expected data points for %s", name)

		for fp := range fps {
			for _, id := range ids {
				assert.NotContains(t, fp, id, "%s: id leaked into metric attributes", name)
			}

			assert.Contains(t, fp, "messaging.destination.template=orders.*.created", "%s: expected the shared template", name)
		}

		assert.Len(t, fps, 1, "%s: different IDs must collapse to one attribute set", name)
	}
}

// TestWithDestinationTemplate_NonMatchingSubjectKeepsConcreteName checks that
// a subject the template func returns "" for keeps messaging.destination.name.
func TestWithDestinationTemplate_NonMatchingSubjectKeepsConcreteName(t *testing.T) {
	metricReader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(metricReader))

	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })

	tel, err := goflux.NewTelemetry(
		goflux.WithMeterProvider(mp),
		goflux.WithDestinationTemplate(orderTemplate),
	)
	require.NoError(t, err)

	ctx := context.Background()
	require.NoError(t, tel.RecordPublish(ctx, "orders.created", "test", func(context.Context) error { return nil }))

	var rm metricdata.ResourceMetrics
	require.NoError(t, metricReader.Collect(ctx, &rm))

	found := false

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "messaging.client.sent.messages" {
				continue
			}

			for _, fp := range dataPointAttrFingerprints(t, m) {
				if strings.Contains(fp, "messaging.destination.name=orders.created;") {
					found = true
				}

				assert.NotContains(t, fp, "messaging.destination.template", "unmatched subject must not get a template attribute")
			}
		}
	}

	assert.True(t, found, "expected orders.created to keep its concrete destination name")
}

// dataPointAttrFingerprints returns one deterministic string per data point,
// built from its attribute set, so two data points can be compared for
// attribute-set equality without depending on metricdata's internal types.
func dataPointAttrFingerprints(t *testing.T, m metricdata.Metrics) []string {
	t.Helper()

	var fps []string

	switch d := m.Data.(type) {
	case metricdata.Sum[int64]:
		for _, dp := range d.DataPoints {
			fps = append(fps, fingerprintAttrs(dp.Attributes))
		}
	case metricdata.Histogram[float64]:
		for _, dp := range d.DataPoints {
			fps = append(fps, fingerprintAttrs(dp.Attributes))
		}
	}

	return fps
}

func fingerprintAttrs(set attribute.Set) string {
	var b strings.Builder

	iter := set.Iter()
	for iter.Next() {
		kv := iter.Attribute()
		b.WriteString(string(kv.Key))
		b.WriteString("=")
		b.WriteString(kv.Value.String())
		b.WriteString(";")
	}

	return b.String()
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
