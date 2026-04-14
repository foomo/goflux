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

// Requester sends typed requests over HTTP POST and deserializes responses.
type Requester[Req, Resp any] struct {
	baseURL    string
	reqCodec   goencode.Codec[Req]
	respCodec  goencode.Codec[Resp]
	httpClient *http.Client
	tel        *goflux.Telemetry
}

// NewRequester creates an HTTP request-reply client. If client is nil, a
// default client with a 10s timeout is used.
func NewRequester[Req, Resp any](
	baseURL string,
	reqCodec goencode.Codec[Req],
	respCodec goencode.Codec[Resp],
	client *http.Client,
	opts ...PublisherOption,
) *Requester[Req, Resp] {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}

	cfg := applyPublisherOpts(opts)

	return &Requester[Req, Resp]{
		baseURL:    baseURL,
		reqCodec:   reqCodec,
		respCodec:  respCodec,
		httpClient: client,
		tel:        cfg.tel,
	}
}

// Request sends req to {baseURL}/{subject} and returns the decoded response.
func (r *Requester[Req, Resp]) Request(ctx context.Context, subject string, req Req) (Resp, error) {
	var result Resp

	err := r.tel.RecordRequest(ctx, subject, system, func(ctx context.Context) error {
		b, encErr := r.reqCodec.Encode(req)
		if encErr != nil {
			return fmt.Errorf("http requester encode: %w", encErr)
		}

		trace.SpanFromContext(ctx).SetAttributes(
			attribute.Int("messaging.message.body.size", len(b)),
			attribute.String("messaging.operation.type", "publish"),
		)

		url := r.baseURL + "/" + subject

		httpReq, buildErr := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(b))
		if buildErr != nil {
			return fmt.Errorf("http requester build request: %w", buildErr)
		}

		httpReq.Header.Set("Content-Type", "application/json")
		r.tel.InjectContext(ctx, propagation.HeaderCarrier(httpReq.Header))

		if id := goflux.MessageID(ctx); id != "" {
			httpReq.Header.Set(goflux.MessageIDHeader, id)
		}

		httpResp, doErr := r.httpClient.Do(httpReq)
		if doErr != nil {
			return fmt.Errorf("http requester send: %w", doErr)
		}
		defer httpResp.Body.Close()

		respBody, readErr := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
		if readErr != nil {
			return fmt.Errorf("http requester read response: %w", readErr)
		}

		if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
			return fmt.Errorf("http requester: server returned %d for %s", httpResp.StatusCode, url)
		}

		if decErr := r.respCodec.Decode(respBody, &result); decErr != nil {
			return fmt.Errorf("http requester decode response: %w", decErr)
		}

		return nil
	})

	return result, err
}

// Close is a no-op.
func (r *Requester[Req, Resp]) Close() error { return nil }
