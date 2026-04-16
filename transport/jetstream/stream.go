package jetstream

import (
	"context"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// NewStream creates or updates a JetStream stream configuration and returns a JetStream instance for further operations.
func NewStream(ctx context.Context, nc *nats.Conn, cfg jetstream.StreamConfig, opts ...jetstream.JetStreamOpt) (jetstream.JetStream, error) {
	js, err := jetstream.New(nc, opts...)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if _, err := js.CreateOrUpdateStream(ctx, cfg); err != nil {
		return nil, err
	}

	return js, nil
}
