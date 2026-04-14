package jetstream

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/foomo/goencode"
	"github.com/foomo/goflux"
	"github.com/nats-io/nats.go/jetstream"
)

// Consumer provides pull-based message consumption from a JetStream consumer.
// Each fetched message MUST be explicitly acknowledged.
type Consumer[T any] struct {
	consumer jetstream.Consumer
	codec    goencode.Codec[T]
	tel      *goflux.Telemetry
}

// NewConsumer creates a pull-based JetStream consumer.
func NewConsumer[T any](consumer jetstream.Consumer, codec goencode.Codec[T], opts ...Option) *Consumer[T] {
	cfg := applyOpts(opts)

	return &Consumer[T]{
		consumer: consumer,
		codec:    codec,
		tel:      cfg.tel,
	}
}

// Fetch retrieves up to n messages from the JetStream consumer. It blocks
// until at least one message is available or ctx is cancelled. Each returned
// message must be explicitly acked via msg.Ack(), msg.Nak(), or msg.Term().
func (c *Consumer[T]) Fetch(ctx context.Context, n int) ([]goflux.Message[T], error) {
	batch, err := c.consumer.Fetch(n, jetstream.FetchContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("jetstream consumer fetch: %w", err)
	}

	var msgs []goflux.Message[T]

	for raw := range batch.Messages() {
		var v T
		if err := c.codec.Decode(raw.Data(), &v); err != nil {
			slog.ErrorContext(ctx, "jetstream consumer: decode failed, terminating message",
				slog.String("subject", raw.Subject()),
				slog.Any("error", err),
			)
			_ = raw.Term()

			continue
		}

		m := goflux.Message[T]{
			Subject: raw.Subject(),
			Payload: v,
			Header:  goflux.Header(raw.Headers()),
		}.WithAcker(&jsAcker{msg: raw})

		msgs = append(msgs, m)
	}

	if err := batch.Error(); err != nil {
		return msgs, fmt.Errorf("jetstream consumer fetch: %w", err)
	}

	return msgs, nil
}

// Close is a no-op. The caller owns the jetstream.Consumer and the
// underlying *nats.Conn.
func (c *Consumer[T]) Close() error { return nil }
