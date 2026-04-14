package jetstream

import (
	"context"
	"errors"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

func NewStream(ctx context.Context, nc *nats.Conn, cfg jetstream.StreamConfig, opts ...jetstream.JetStreamOpt) (jetstream.JetStream, error) {
	js, err := jetstream.New(nc, opts...)
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if _, err := js.CreateStream(ctx, cfg); err != nil && !errors.Is(err, jetstream.ErrStreamNameAlreadyInUse) {
		return nil, err
	}

	return js, nil
}
