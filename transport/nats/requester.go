package nats

import (
	"context"
	"fmt"

	"github.com/foomo/goencode"
	"github.com/foomo/goflux"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Requester sends typed requests over NATS and waits for typed responses.
type Requester[Req, Resp any] struct {
	conn      *nats.Conn
	reqCodec  goencode.Codec[Req]
	respCodec goencode.Codec[Resp]
	tel       *goflux.Telemetry
}

// NewRequester creates a NATS request-reply client.
func NewRequester[Req, Resp any](
	conn *nats.Conn,
	reqCodec goencode.Codec[Req],
	respCodec goencode.Codec[Resp],
	opts ...Option,
) *Requester[Req, Resp] {
	cfg := applyOpts(opts)

	return &Requester[Req, Resp]{
		conn:      conn,
		reqCodec:  reqCodec,
		respCodec: respCodec,
		tel:       cfg.tel,
	}
}

// Request sends req to the given subject and waits for a response.
func (r *Requester[Req, Resp]) Request(ctx context.Context, subject string, req Req) (Resp, error) {
	var result Resp

	err := r.tel.RecordRequest(ctx, subject, system, func(ctx context.Context) error {
		b, encErr := r.reqCodec.Encode(req)
		if encErr != nil {
			return fmt.Errorf("nats requester encode: %w", encErr)
		}

		trace.SpanFromContext(ctx).SetAttributes(
			attribute.Int("messaging.message.body.size", len(b)),
			attribute.String("messaging.operation.type", "publish"),
		)

		msg := nats.NewMsg(subject)
		msg.Data = b
		msg.Header = make(nats.Header)

		r.tel.InjectContext(ctx, natsHeaderCarrier{Headers: msg.Header})

		if id := goflux.MessageID(ctx); id != "" {
			msg.Header.Set(goflux.MessageIDHeader, id)
		}

		reply, reqErr := r.conn.RequestMsgWithContext(ctx, msg)
		if reqErr != nil {
			return fmt.Errorf("nats requester: %w", reqErr)
		}

		if decErr := r.respCodec.Decode(reply.Data, &result); decErr != nil {
			return fmt.Errorf("nats requester decode response: %w", decErr)
		}

		return nil
	})

	return result, err
}

// Close drains the underlying NATS connection.
func (r *Requester[Req, Resp]) Close() error { return r.conn.Drain() }
