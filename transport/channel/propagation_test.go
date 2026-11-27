package channel_test

import (
	"context"
	"testing"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/transport/channel"
	"github.com/foomo/gofuncy"
	"github.com/stretchr/testify/require"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
	"go.opentelemetry.io/otel/trace"
)

// TestSubscriber_propagatesMessageIDAndSpan asserts that the handler receives
// the publisher's message ID as msg.ID but not in its context, that the
// process span records it, and that the process span links to the publish
// span, like on network transports.
func TestSubscriber_propagatesMessageIDAndSpan(t *testing.T) {
	t.Parallel()

	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exporter))

	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })

	tel, err := goflux.NewTelemetry(goflux.WithTracerProvider(tp))
	require.NoError(t, err)

	bus := channel.NewBus[Event]()
	sub := channel.NewSubscriber(bus, 1, channel.WithTelemetry(tel))

	pub := channel.NewPublisher(bus, channel.WithTelemetry(tel))

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	ids := make(chan [2]string, 1)
	done := make(chan struct{})

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		defer close(done)

		return sub.SubscribeWithReady(ctx, "orders", func(ctx context.Context, msg goflux.Message[Event]) error {
			ids <- [2]string{msg.ID, goflux.MessageID(ctx)}

			return nil
		}, ready)
	}, gofuncy.WithName("test.subscriber"))

	require.NoError(t, pub.Publish(goflux.WithMessageID(t.Context(), "id-1"), "orders", Event{}))
	require.Equal(t, [2]string{"id-1", ""}, <-ids)

	cancel()
	<-done

	var sendSpan, processSpan *tracetest.SpanStub

	for _, s := range exporter.GetSpans() {
		switch s.SpanKind {
		case trace.SpanKindProducer:
			sendSpan = &s
		case trace.SpanKindConsumer:
			processSpan = &s
		default:
		}
	}

	require.NotNil(t, sendSpan)
	require.NotNil(t, processSpan)
	require.Len(t, processSpan.Links, 1)
	require.Equal(t, sendSpan.SpanContext.SpanID(), processSpan.Links[0].SpanContext.SpanID())
	require.Contains(t, processSpan.Attributes, semconv.MessagingMessageID("id-1"))
}
