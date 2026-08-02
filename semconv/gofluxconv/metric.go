package gofluxconv

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/foomo/goflux/semconv"
)

// ------------------------------------------------------------------------------------------------
// ~ Constants
// ------------------------------------------------------------------------------------------------

const (
	ackOutcomeName = "goflux.processor.ack.outcome"
	ackOutcomeDesc = "Number of message acknowledgment outcomes by action"

	consumerLagName = "goflux.consumer.lag"
	consumerLagDesc = "Number of messages waiting in the subscriber buffer"
)

// ------------------------------------------------------------------------------------------------
// ~ AckOutcome
// ------------------------------------------------------------------------------------------------

// AckOutcome counts message acknowledgment outcomes by action.
type AckOutcome struct {
	inst metric.Int64Counter
}

// NewAckOutcome creates a new ack outcome counter.
func NewAckOutcome(m metric.Meter) (AckOutcome, error) {
	if m == nil {
		return AckOutcome{}, nil
	}

	c, err := m.Int64Counter(ackOutcomeName,
		metric.WithDescription(ackOutcomeDesc),
	)

	return AckOutcome{inst: c}, err
}

func (AckOutcome) Name() string                { return ackOutcomeName }
func (AckOutcome) Unit() string                { return "" }
func (AckOutcome) Description() string         { return ackOutcomeDesc }
func (g AckOutcome) Inst() metric.Int64Counter { return g.inst }

// Add records an acknowledgment outcome for the given action and subject, with
// hasError indicating whether the ack operation itself failed.
func (g AckOutcome) Add(ctx context.Context, incr int64, action, subject string, hasError bool, attrs ...attribute.KeyValue) {
	if g.inst == nil {
		return
	}

	base := []attribute.KeyValue{
		semconv.AckAction(action),
		semconv.DestinationName(subject),
		semconv.AckError(hasError),
	}

	if len(attrs) == 0 {
		g.inst.Add(ctx, incr, metric.WithAttributes(base...))
		return
	}

	g.inst.Add(ctx, incr, metric.WithAttributes(append(attrs, base...)...))
}

// ------------------------------------------------------------------------------------------------
// ~ ConsumerLag
// ------------------------------------------------------------------------------------------------

// ConsumerLag observes the number of messages waiting in the subscriber buffer.
type ConsumerLag struct {
	inst metric.Int64ObservableGauge
}

// NewConsumerLag registers an observable gauge that reports the current lag for
// the given subject via lagFn on each collection.
func NewConsumerLag(m metric.Meter, subject string, lagFn func() int64) (ConsumerLag, error) {
	if m == nil {
		return ConsumerLag{}, nil
	}

	g, err := m.Int64ObservableGauge(consumerLagName,
		metric.WithDescription(consumerLagDesc),
		metric.WithInt64Callback(func(_ context.Context, obs metric.Int64Observer) error {
			obs.Observe(lagFn(),
				metric.WithAttributes(semconv.DestinationName(subject)),
			)

			return nil
		}),
	)

	return ConsumerLag{inst: g}, err
}

func (ConsumerLag) Name() string                        { return consumerLagName }
func (ConsumerLag) Unit() string                        { return "" }
func (ConsumerLag) Description() string                 { return consumerLagDesc }
func (g ConsumerLag) Inst() metric.Int64ObservableGauge { return g.inst }
