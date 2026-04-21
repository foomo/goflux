package http

import (
	"context"
	"io"
	"net/http"
	"strings"

	"github.com/foomo/goencode"
	"github.com/foomo/goflux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	DefaultBasePath string = ""
	// DefaultMaxBodySize is the maximum request body size the subscriber will
	// read. Override per-subscriber via the WithMaxBodySize option.
	DefaultMaxBodySize int64 = 1 << 20 // 1 MiB
)

// Subscriber decodes incoming POST requests and dispatches them to a Handler.
// It owns an *http.ServeMux that routes are registered on; the mux is exposed
// via Handler() so the caller can hand it to any http.Server — including keel's
// service.NewHTTP.
//
// Subscriber intentionally does not start its own net.Listener. Lifecycle
// (start, graceful shutdown) belongs to the server layer above it.
type Subscriber[T any] struct {
	decoder     goencode.Decoder[T, []byte]
	mux         *http.ServeMux
	tel         *goflux.Telemetry
	basePath    string
	maxBodySize int64
}

// SubscriberOption configures a Subscriber.
type SubscriberOption func(*subscriberConfig)

type subscriberConfig struct {
	basePath    string
	maxBodySize int64
	tel         *goflux.Telemetry
}

// WithMaxBodySize sets the maximum request body size the subscriber will read.
// Requests exceeding this limit receive 413 Request Entity Too Large.
func WithMaxBodySize(v int64) SubscriberOption {
	return func(c *subscriberConfig) { c.maxBodySize = v }
}

// WithBasePath sets the basePath configuration for a Subscriber to the specified value.
func WithBasePath(v string) SubscriberOption {
	return func(c *subscriberConfig) { c.basePath = v }
}

// WithTelemetry sets the Telemetry instance for the subscriber. If not
// provided, a default instance is created from the current OTel globals.
func WithTelemetry(t *goflux.Telemetry) SubscriberOption {
	return func(c *subscriberConfig) { c.tel = t }
}

// NewSubscriber creates an HTTP subscriber. Call Subscribe to register subjects,
// then pass Mux() to service.NewHTTP (keel) or http.ListenAndServe.
func NewSubscriber[T any](decoder goencode.Decoder[T, []byte], opts ...SubscriberOption) *Subscriber[T] {
	cfg := &subscriberConfig{
		basePath:    DefaultBasePath,
		maxBodySize: DefaultMaxBodySize,
	}
	for _, o := range opts {
		o(cfg)
	}

	cfg.tel = goflux.DefaultTelemetry(cfg.tel)

	return &Subscriber[T]{
		mux:         http.NewServeMux(),
		decoder:     decoder,
		tel:         cfg.tel,
		maxBodySize: cfg.maxBodySize,
	}
}

func (s *Subscriber[T]) Mux() *http.ServeMux {
	return s.mux
}

// Subscribe registers handler for POST {basePath}/{nats} on the mux.
// Multiple subjects can be registered before the server starts.
// ctx is used as the base context for all handler invocations.
//
// Responses: 204 on success, 400 on decode failure, 405 on wrong method,
// 413 if body exceeds max size, 500 if the handler returns an error.
func (s *Subscriber[T]) Subscribe(ctx context.Context, subject string, handler goflux.Handler[T]) error {
	s.mux.Handle(s.basePath+"/"+subject, s.Handler(subject, handler))
	// Subscribe is non-blocking here — the server is started externally.
	// Block until ctx is cancelled so the caller's goroutine stays alive.
	<-ctx.Done()

	return nil
}

// Handler returns an http.HandlerFunc that dispatches incoming POST requests
// to the given handler.
func (s *Subscriber[T]) Handler(subject string, handler goflux.Handler[T]) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		defer r.Body.Close()

		body, err := io.ReadAll(io.LimitReader(r.Body, s.maxBodySize+1))
		if err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)

			return
		}

		if int64(len(body)) > s.maxBodySize {
			http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
			return
		}

		var v T
		if err := s.decoder(body, &v); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)

			return
		}

		msg := goflux.Message[T]{Subject: subject, Payload: v, Header: extractGofluxHeaders(r.Header)}

		// Extract OTel trace context from incoming HTTP headers to link
		// consumer spans to the originating producer span.
		msgCtx := s.tel.ExtractContext(r.Context(), propagation.HeaderCarrier(r.Header))

		if id := r.Header.Get(goflux.MessageIDHeader); id != "" {
			msgCtx = goflux.WithMessageID(msgCtx, id)
		}

		err = s.tel.RecordProcess(msgCtx, subject, system, func(hCtx context.Context) error {
			trace.SpanFromContext(hCtx).SetAttributes(
				attribute.Int("messaging.message.body.size", len(body)),
				attribute.String("messaging.operation.type", "process"),
			)

			return handler(hCtx, msg)
		})
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)

			return
		}

		w.WriteHeader(http.StatusNoContent)
	}
}

// Close is a no-op; shutdown is handled by keel / the outer http.Server.
func (s *Subscriber[T]) Close() error { return nil }

const gofluxHeaderPrefix = "X-Goflux-"

// extractGofluxHeaders extracts X-Goflux-* headers into a goflux.Header,
// stripping the prefix. Returns nil if no goflux headers are present.
func extractGofluxHeaders(h http.Header) goflux.Header {
	var out goflux.Header

	for k, vs := range h {
		if strings.HasPrefix(k, gofluxHeaderPrefix) {
			if out == nil {
				out = make(goflux.Header)
			}

			out[strings.TrimPrefix(k, gofluxHeaderPrefix)] = vs
		}
	}

	return out
}
