package http

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/foomo/goencode"
	"github.com/foomo/goflux"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Publisher POSTs encoded messages to a base URL.
// Subject is appended to BaseURL as the path: POST {BaseURL}/{subject}
type Publisher[T any] struct {
	baseURL    string
	serializer goencode.Codec[T]
	httpClient *http.Client
	tel        *goflux.Telemetry
	// ContentType is sent as the Content-Type header. Defaults to
	// "application/json" if empty.
	ContentType string
}

// NewPublisher creates an HTTP publisher.
// baseURL is the target service root, e.g. "https://orders.internal".
// An optional *http.Client may be provided; if nil the default client is used.
func NewPublisher[T any](baseURL string, serializer goencode.Codec[T], client *http.Client, opts ...PublisherOption) *Publisher[T] {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	cfg := applyPublisherOpts(opts)

	return &Publisher[T]{baseURL: baseURL, serializer: serializer, httpClient: client, tel: cfg.tel}
}

// Publish encodes v and POSTs it to {baseURL}/{subject}.
// A non-2xx response is treated as an error.
func (p *Publisher[T]) Publish(ctx context.Context, subject string, v T) error {
	return p.tel.RecordPublish(ctx, subject, system, func(ctx context.Context) error {
		return p.post(ctx, subject, v)
	})
}

func (p *Publisher[T]) post(ctx context.Context, subject string, v T) error {
	b, err := p.serializer.Encode(v)
	if err != nil {
		return fmt.Errorf("http publisher encode: %w", err)
	}

	trace.SpanFromContext(ctx).SetAttributes(
		attribute.Int("messaging.message.body.size", len(b)),
		attribute.String("messaging.operation.type", "publish"),
	)

	ct := p.ContentType
	if ct == "" {
		ct = "application/json"
	}

	url := p.baseURL + "/" + subject

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		return fmt.Errorf("http publisher build request: %w", err)
	}

	req.Header.Set("Content-Type", ct)

	p.tel.InjectContext(ctx, propagation.HeaderCarrier(req.Header))

	if id := goflux.MessageID(ctx); id != "" {
		req.Header.Set(goflux.MessageIDHeader, id)
	}

	if h := goflux.HeaderFromContext(ctx); h != nil {
		for k, vs := range h {
			for _, v := range vs {
				req.Header.Add("X-Goflux-"+k, v)
			}
		}
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http publisher send: %w", err)
	}
	defer resp.Body.Close()
	// drain body so the connection can be reused (limit to 1 MiB)
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("http publisher: server returned %d for %s", resp.StatusCode, url)
	}

	return nil
}

func (p *Publisher[T]) Close() error { return nil }
