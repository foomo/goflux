package nats

import (
	"context"
	"errors"
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
	reqCodec  goencode.Codec[Req, []byte]
	respCodec goencode.Codec[Resp, []byte]
	tel       *goflux.Telemetry
}

// NewRequester creates a NATS request-reply client.
func NewRequester[Req, Resp any](
	conn *nats.Conn,
	reqCodec goencode.Codec[Req, []byte],
	respCodec goencode.Codec[Resp, []byte],
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

// Request sends req to the given nats and waits for a response.
func (r *Requester[Req, Resp]) Request(ctx context.Context, subject string, req Req) (Resp, error) {
	var result Resp

	err := r.tel.RecordRequest(ctx, subject, system, func(ctx context.Context) error {
		b, encErr := r.reqCodec.Encode(req)
		if encErr != nil {
			return errors.Join(goflux.ErrPublish, goflux.ErrEncode, fmt.Errorf("nats: %w", encErr))
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
			return errors.Join(goflux.ErrTransport, fmt.Errorf("nats: %w", reqErr))
		}

		if decErr := r.respCodec.Decode(reply.Data, &result); decErr != nil {
			return errors.Join(goflux.ErrDecode, fmt.Errorf("nats: %w", decErr))
		}

		return nil
	})

	return result, err
}

// Close is a no-op. The caller owns the *nats.Conn and is responsible for
// draining or closing it.
func (r *Requester[Req, Resp]) Close() error { return nil }
