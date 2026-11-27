package channel_test

import (
	"context"
	"testing"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/transport/channel"
	"github.com/foomo/gofuncy"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

// lagPoints collects the goflux.consumer.lag data points by destination name.
func lagPoints(t *testing.T, reader metric.Reader) map[string]int64 {
	t.Helper()

	var rm metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &rm))

	points := map[string]int64{}

	for _, sm := range rm.ScopeMetrics {
		for _, m := range sm.Metrics {
			if m.Name != "goflux.consumer.lag" {
				continue
			}

			gauge, ok := m.Data.(metricdata.Gauge[int64])
			require.True(t, ok, "goflux.consumer.lag is not an int64 gauge")

			for _, dp := range gauge.DataPoints {
				name, _ := dp.Attributes.Value(attribute.Key("messaging.destination.name"))
				points[name.AsString()] = dp.Value
			}
		}
	}

	return points
}

// TestSubscriber_lagGauge asserts that the lag gauge reports the subscribed
// subject while SubscribeWithReady runs, and stops after it returns.
func TestSubscriber_lagGauge(t *testing.T) {
	t.Parallel()

	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))

	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })

	tel, err := goflux.NewTelemetry(goflux.WithMeterProvider(mp))
	require.NoError(t, err)

	bus := channel.NewBus[Event]()
	sub := channel.NewSubscriber(bus, 4, channel.WithTelemetry(tel))

	require.Empty(t, lagPoints(t, reader))

	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan struct{})

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		defer close(done)

		return sub.SubscribeWithReady(ctx, "orders", func(context.Context, goflux.Message[Event]) error { return nil }, ready)
	}, gofuncy.WithName("test.subscriber"))

	require.Equal(t, map[string]int64{"orders": 0}, lagPoints(t, reader))

	cancel()
	<-done

	require.Empty(t, lagPoints(t, reader))
}

// TestSubscriber_lagGaugePerSubscription asserts that each subscription of one
// Subscriber reports its own buffer, not the latest subscription's.
func TestSubscriber_lagGaugePerSubscription(t *testing.T) {
	t.Parallel()

	reader := metric.NewManualReader()
	mp := metric.NewMeterProvider(metric.WithReader(reader))

	t.Cleanup(func() { _ = mp.Shutdown(context.Background()) })

	tel, err := goflux.NewTelemetry(goflux.WithMeterProvider(mp))
	require.NoError(t, err)

	bus := channel.NewBus[Event]()
	sub := channel.NewSubscriber(bus, 4, channel.WithTelemetry(tel))
	pub := channel.NewPublisher(bus, channel.WithTelemetry(tel))

	ctx, cancel := context.WithCancel(t.Context())
	started := make(chan struct{}, 1)
	release := make(chan struct{})
	doneA, doneB := make(chan struct{}), make(chan struct{})

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		defer close(doneA)

		return sub.SubscribeWithReady(ctx, "a", func(context.Context, goflux.Message[Event]) error {
			select {
			case started <- struct{}{}:
			default:
			}

			<-release

			return nil
		}, ready)
	}, gofuncy.WithName("test.subscriber.a"))

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		defer close(doneB)

		return sub.SubscribeWithReady(ctx, "b", func(context.Context, goflux.Message[Event]) error { return nil }, ready)
	}, gofuncy.WithName("test.subscriber.b"))

	// The first message blocks the handler of "a"; the next two stay buffered.
	for range 3 {
		require.NoError(t, pub.Publish(t.Context(), "a", Event{}))
	}

	<-started
	require.Equal(t, map[string]int64{"a": 2, "b": 0}, lagPoints(t, reader))

	cancel()
	close(release)
	<-doneA
	<-doneB
}
