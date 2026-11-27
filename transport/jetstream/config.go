package jetstream

import (
	"strings"

	"github.com/nats-io/nats.go/jetstream"
)

// DefaultStreamConfig returns a stream configuration named after topic that
// accepts messages published to that subject, using JetStream's default
// (limits-based) retention.
func DefaultStreamConfig(topic string) *jetstream.StreamConfig {
	return &jetstream.StreamConfig{
		Name:     topic,
		Subjects: []string{topic},
	}
}

// WorkQueueStreamConfig returns a stream configuration named after topic
// with work-queue retention: each message is removed once acknowledged by
// any consumer, so at most one consumer group may process the stream.
func WorkQueueStreamConfig(topic string) jetstream.StreamConfig {
	return jetstream.StreamConfig{
		Name:      topic,
		Subjects:  []string{topic},
		Retention: jetstream.WorkQueuePolicy,
	}
}

// ConsumerConfig returns a consumer configuration for topic with explicit
// ack policy. If group is non-empty it is included in the generated name, so
// distinct groups on the same topic get distinct consumers.
//
// The consumer is ephemeral: the server deletes it after its inactive
// threshold, and [Subscriber.Subscribe] returns [goflux.ErrSubscribe] when
// that happens while a pull is pending. A deletion without a pending pull,
// for example during a connection outage, is not reported by nats.go, so
// Subscribe keeps retrying instead. For a consumer that outlives its subscribers, opt in by
// setting Durable to the generated name:
//
//	cfg := ConsumerConfig(topic, group)
//	cfg.Durable = cfg.Name
//
// Durable consumers stay on the server until deleted, so derive group from a
// stable service name, never from a per-instance value such as a hostname.
func ConsumerConfig(topic, group string) jetstream.ConsumerConfig {
	sb := strings.Builder{}
	sb.WriteString("goflux_")

	if group != "" {
		sb.WriteString(group + "_")
	}

	sb.WriteString(topic)

	return jetstream.ConsumerConfig{
		Name:      sb.String(),
		AckPolicy: jetstream.AckExplicitPolicy,
	}
}
