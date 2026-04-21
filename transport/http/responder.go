package http

import (
	"context"
	"io"
	"net/http"

	"github.com/foomo/goencode"
	"github.com/foomo/goflux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Responder handles incoming HTTP requests and sends typed responses.
type Responder[Req, Resp any] struct {
	reqCodec    goencode.Codec[Req, []byte]
	respCodec   goencode.Codec[Resp, []byte]
	mux         *http.ServeMux
	tel         *goflux.Telemetry
	basePath    string
	maxBodySize int64
}

// NewResponder creates an HTTP request-reply server. Call Serve to register
// subjects, then pass Mux() to your HTTP server.
func NewResponder[Req, Resp any](
	reqCodec goencode.Codec[Req, []byte],
	respCodec goencode.Codec[Resp, []byte],
	opts ...SubscriberOption,
) *Responder[Req, Resp] {
	cfg := &subscriberConfig{
		basePath:    DefaultBasePath,
		maxBodySize: DefaultMaxBodySize,
	}
	for _, o := range opts {
		o(cfg)
	}

	cfg.tel = goflux.DefaultTelemetry(cfg.tel)

	return &Responder[Req, Resp]{
		reqCodec:    reqCodec,
		respCodec:   respCodec,
		mux:         http.NewServeMux(),
		tel:         cfg.tel,
		basePath:    cfg.basePath,
		maxBodySize: cfg.maxBodySize,
	}
}

// Serve registers the handler for POST {basePath}/{nats}. The call blocks
// until ctx is cancelled.
func (r *Responder[Req, Resp]) Serve(ctx context.Context, subject string, handler goflux.RequestHandler[Req, Resp]) error {
	r.mux.HandleFunc(r.basePath+"/"+subject, func(w http.ResponseWriter, httpReq *http.Request) {
		if httpReq.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)

			return
		}
		defer httpReq.Body.Close()

		body, err := io.ReadAll(io.LimitReader(httpReq.Body, r.maxBodySize+1))
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)

			return
		}

		if int64(len(body)) > r.maxBodySize {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)

			return
		}

		var req Req
		if err := r.reqCodec.Decode(body, &req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)

			return
		}

		msgCtx := r.tel.ExtractContext(httpReq.Context(), propagation.HeaderCarrier(httpReq.Header))

		if id := httpReq.Header.Get(goflux.MessageIDHeader); id != "" {
			msgCtx = goflux.WithMessageID(msgCtx, id)
		}

		_ = r.tel.RecordProcess(msgCtx, subject, system, func(ctx context.Context) error {
			trace.SpanFromContext(ctx).SetAttributes(
				attribute.Int("messaging.message.body.size", len(body)),
				attribute.String("messaging.operation.type", "process"),
			)

			resp, hErr := handler(ctx, req)
			if hErr != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)

				return hErr
			}

			respBody, encErr := r.respCodec.Encode(resp)
			if encErr != nil {
				http.Error(w, "internal server error", http.StatusInternalServerError)

				return encErr
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(respBody)

			return nil
		})
	})

	<-ctx.Done()

	return nil
}

// Mux returns the underlying ServeMux for mounting on an HTTP server.
func (r *Responder[Req, Resp]) Mux() *http.ServeMux {
	return r.mux
}

// Close is a no-op.
func (r *Responder[Req, Resp]) Close() error { return nil }
