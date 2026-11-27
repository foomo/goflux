package gofluxconv

import (
	"context"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	msgsemconv "go.opentelemetry.io/otel/semconv/v1.41.0"

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

// AttrDestinationName returns the messaging.destination.name attribute.
func (AckOutcome) AttrDestinationName(v string) attribute.KeyValue {
	return msgsemconv.MessagingDestinationName(v)
}

// AttrDestinationTemplate returns the messaging.destination.template attribute.
func (AckOutcome) AttrDestinationTemplate(v string) attribute.KeyValue {
	return msgsemconv.MessagingDestinationTemplate(v)
}

// Add records an acknowledgment outcome for the given action, with hasError
// indicating whether the ack operation itself failed. destination is either
// messaging.destination.name or, to bound cardinality,
// messaging.destination.template (see [AckOutcome.AttrDestinationName] and
// [AckOutcome.AttrDestinationTemplate]).
func (g AckOutcome) Add(ctx context.Context, incr int64, action string, destination attribute.KeyValue, hasError bool, attrs ...attribute.KeyValue) {
	if g.inst == nil {
		return
	}

	base := []attribute.KeyValue{
		semconv.AckAction(action),
		destination,
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

// NewConsumerLag creates the consumer lag gauge. Report values with
// [ConsumerLag.Observe] from a callback registered for [ConsumerLag.Inst] via
// metric.Meter.RegisterCallback.
func NewConsumerLag(m metric.Meter) (ConsumerLag, error) {
	if m == nil {
		return ConsumerLag{}, nil
	}

	g, err := m.Int64ObservableGauge(consumerLagName, metric.WithDescription(consumerLagDesc))

	return ConsumerLag{inst: g}, err
}

// AttrDestinationName returns the messaging.destination.name attribute.
func (ConsumerLag) AttrDestinationName(v string) attribute.KeyValue {
	return msgsemconv.MessagingDestinationName(v)
}

// AttrDestinationTemplate returns the messaging.destination.template attribute.
func (ConsumerLag) AttrDestinationTemplate(v string) attribute.KeyValue {
	return msgsemconv.MessagingDestinationTemplate(v)
}

// Observe reports lag for destination, which is either
// messaging.destination.name or, to bound cardinality,
// messaging.destination.template (see [ConsumerLag.AttrDestinationName] and
// [ConsumerLag.AttrDestinationTemplate]).
func (g ConsumerLag) Observe(obs metric.Observer, lag int64, destination attribute.KeyValue) {
	if g.inst == nil {
		return
	}

	obs.ObserveInt64(g.inst, lag, metric.WithAttributes(destination))
}

func (ConsumerLag) Name() string                        { return consumerLagName }
func (ConsumerLag) Unit() string                        { return "" }
func (ConsumerLag) Description() string                 { return consumerLagDesc }
func (g ConsumerLag) Inst() metric.Int64ObservableGauge { return g.inst }
