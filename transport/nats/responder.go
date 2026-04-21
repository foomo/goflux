package nats

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/foomo/goencode"
	"github.com/foomo/goflux"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Responder handles incoming NATS requests and sends typed responses.
type Responder[Req, Resp any] struct {
	conn      *nats.Conn
	reqCodec  goencode.Codec[Req, []byte]
	respCodec goencode.Codec[Resp, []byte]
	tel       *goflux.Telemetry
}

// NewResponder creates a NATS request-reply server.
func NewResponder[Req, Resp any](
	conn *nats.Conn,
	reqCodec goencode.Codec[Req, []byte],
	respCodec goencode.Codec[Resp, []byte],
	opts ...Option,
) *Responder[Req, Resp] {
	cfg := applyOpts(opts)

	return &Responder[Req, Resp]{
		conn:      conn,
		reqCodec:  reqCodec,
		respCodec: respCodec,
		tel:       cfg.tel,
	}
}

// Serve registers the handler for the given nats. The call blocks until
// ctx is cancelled.
func (r *Responder[Req, Resp]) Serve(ctx context.Context, subject string, handler goflux.RequestHandler[Req, Resp]) error {
	sub, err := r.conn.Subscribe(subject, func(msg *nats.Msg) {
		var req Req
		if err := r.reqCodec.Decode(msg.Data, &req); err != nil {
			slog.ErrorContext(ctx, "nats responder: decode failed, dropping request",
				slog.String("nats", msg.Subject),
				slog.Any("error", err),
			)

			return
		}

		remoteSpanCtx := r.tel.ExtractSpanContext(ctx, natsHeaderCarrier{Headers: msg.Header})

		msgCtx := ctx
		if id := msg.Header.Get(goflux.MessageIDHeader); id != "" {
			msgCtx = goflux.WithMessageID(msgCtx, id)
		}

		_ = r.tel.RecordProcess(msgCtx, subject, system, func(ctx context.Context) error {
			trace.SpanFromContext(ctx).SetAttributes(
				attribute.Int("messaging.message.body.size", len(msg.Data)),
				attribute.String("messaging.operation.type", "process"),
			)

			resp, hErr := handler(ctx, req)
			if hErr != nil {
				return hErr
			}

			b, encErr := r.respCodec.Encode(resp)
			if encErr != nil {
				return errors.Join(goflux.ErrEncode, fmt.Errorf("nats: %w", encErr))
			}

			return msg.Respond(b)
		}, goflux.WithRemoteSpanContext(remoteSpanCtx))
	})
	if err != nil {
		return errors.Join(goflux.ErrSubscribe, goflux.ErrTransport, fmt.Errorf("nats: %w", err))
	}

	<-ctx.Done()

	return sub.Unsubscribe()
}

// Close is a no-op. The caller owns the *nats.Conn and is responsible for
// draining or closing it.
func (r *Responder[Req, Resp]) Close() error { return nil }
