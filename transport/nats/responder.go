package nats

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/foomo/goencode"
	"github.com/foomo/goflux"
	gsemconv "github.com/foomo/goflux/semconv"
	"github.com/nats-io/nats.go"
	semconv "go.opentelemetry.io/otel/semconv/v1.41.0"
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

// Serve registers the handler for the given subject. The call blocks until
// ctx is cancelled.
//
// Serve does not report when the subscription reaches the server. A client that
// issues a request immediately after Serve starts may find no responder
// listening yet; core NATS drops the request silently and the requester fails
// with a timeout. Use [Responder.ServeWithReady] to sequence the first request
// after registration.
func (r *Responder[Req, Resp]) Serve(ctx context.Context, subject string, handler goflux.RequestHandler[Req, Resp]) error {
	return r.ServeWithReady(ctx, subject, handler, func() {})
}

// ServeWithReady behaves like [Responder.Serve] but invokes ready once the
// server has acknowledged the subscription, then blocks until ctx is cancelled.
//
// ready is called at most once and never after ServeWithReady returns. If
// registration fails, the error is returned without calling ready.
func (r *Responder[Req, Resp]) ServeWithReady(
	ctx context.Context,
	subject string,
	handler goflux.RequestHandler[Req, Resp],
	ready func(),
) error {
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
			sp := trace.SpanFromContext(ctx)
			sp.SetAttributes(
				semconv.MessagingMessageBodySize(len(msg.Data)),
				semconv.MessagingOperationTypeProcess,
			)

			resp, hErr := handler(ctx, req)
			if hErr != nil {
				return hErr
			}

			b, encErr := r.respCodec.Encode(resp)
			if encErr != nil {
				return errors.Join(goflux.ErrEncode, fmt.Errorf("nats: %w", encErr))
			}

			sp.SetAttributes(gsemconv.ReplyBodySize(len(b)))

			return msg.Respond(b)
		}, goflux.WithRemoteSpanContext(remoteSpanCtx))
	})
	if err != nil {
		return errors.Join(goflux.ErrSubscribe, goflux.ErrTransport, fmt.Errorf("nats: %w", err))
	}

	// See Subscriber.SubscribeWithReady: conn.Subscribe only buffers the SUB
	// frame, so flush before declaring the responder ready.
	if err := r.conn.Flush(); err != nil {
		_ = sub.Unsubscribe()

		return errors.Join(goflux.ErrSubscribe, goflux.ErrTransport, fmt.Errorf("nats: flush: %w", err))
	}

	ready()

	<-ctx.Done()

	return sub.Unsubscribe()
}

// Close is a no-op. The caller owns the *nats.Conn and is responsible for
// draining or closing it.
func (r *Responder[Req, Resp]) Close() error { return nil }
