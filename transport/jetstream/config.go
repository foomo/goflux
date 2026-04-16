package jetstream

import (
	"strings"

	"github.com/nats-io/nats.go/jetstream"
)

func DefaultStreamConfig(topic string) *jetstream.StreamConfig {
	return &jetstream.StreamConfig{
		Name:     topic,
		Subjects: []string{topic},
	}
}

func WorkQueueStreamConfig(topic string) jetstream.StreamConfig {
	return jetstream.StreamConfig{
		Name:      topic,
		Subjects:  []string{topic},
		Retention: jetstream.WorkQueuePolicy,
	}
}

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
