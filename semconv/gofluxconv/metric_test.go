package gofluxconv_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/metric/noop"

	"github.com/foomo/goflux/semconv/gofluxconv"
)

func TestAckOutcome(t *testing.T) {
	t.Parallel()

	m, err := gofluxconv.NewAckOutcome(noop.Meter{})
	require.NoError(t, err)

	assert.Equal(t, "goflux.processor.ack.outcome", m.Name())
	assert.Empty(t, m.Unit())
	assert.Equal(t, "Number of message acknowledgment outcomes by action", m.Description())
	assert.NotNil(t, m.Inst())

	// must not panic
	m.Add(context.Background(), 1, "ack", "orders.created", false)
	m.Add(context.Background(), 1, "nak", "orders.created", true)
}

func TestAckOutcome_nilMeter(t *testing.T) {
	t.Parallel()

	m, err := gofluxconv.NewAckOutcome(nil)
	require.NoError(t, err)

	assert.Nil(t, m.Inst())

	// must not panic with nil instrument
	m.Add(context.Background(), 1, "ack", "orders.created", false)
}

func TestConsumerLag(t *testing.T) {
	t.Parallel()

	m, err := gofluxconv.NewConsumerLag(noop.Meter{}, "orders.created", func() int64 { return 42 })
	require.NoError(t, err)

	assert.Equal(t, "goflux.consumer.lag", m.Name())
	assert.Empty(t, m.Unit())
	assert.Equal(t, "Number of messages waiting in the subscriber buffer", m.Description())
	assert.NotNil(t, m.Inst())
}

func TestConsumerLag_nilMeter(t *testing.T) {
	t.Parallel()

	m, err := gofluxconv.NewConsumerLag(nil, "orders.created", func() int64 { return 0 })
	require.NoError(t, err)

	assert.Nil(t, m.Inst())
}
