package http

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	reqCodec   goencode.Codec[Req, []byte]
	respCodec  goencode.Codec[Resp, []byte]
	httpClient *http.Client
	tel        *goflux.Telemetry
}

// NewRequester creates an HTTP request-reply client. If client is nil, a
// default client with a 10s timeout is used.
func NewRequester[Req, Resp any](
	baseURL string,
	reqCodec goencode.Codec[Req, []byte],
	respCodec goencode.Codec[Resp, []byte],
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

// Request sends req to {baseURL}/{nats} and returns the decoded response.
func (r *Requester[Req, Resp]) Request(ctx context.Context, subject string, req Req) (Resp, error) {
	var result Resp

	err := r.tel.RecordRequest(ctx, subject, system, func(ctx context.Context) error {
		b, encErr := r.reqCodec.Encode(req)
		if encErr != nil {
			return errors.Join(goflux.ErrPublish, goflux.ErrEncode, fmt.Errorf("http: %w", encErr))
		}

		trace.SpanFromContext(ctx).SetAttributes(
			attribute.Int("messaging.message.body.size", len(b)),
			attribute.String("messaging.operation.type", "publish"),
		)

		target := r.baseURL + "/" + url.PathEscape(subject)

		httpReq, buildErr := http.NewRequestWithContext(ctx, http.MethodPost, target, bytes.NewReader(b))
		if buildErr != nil {
			return errors.Join(goflux.ErrPublish, goflux.ErrTransport, fmt.Errorf("http: %w", buildErr))
		}

		httpReq.Header.Set("Content-Type", "application/json")
		r.tel.InjectContext(ctx, propagation.HeaderCarrier(httpReq.Header))

		if id := goflux.MessageID(ctx); id != "" {
			httpReq.Header.Set(goflux.MessageIDHeader, id)
		}

		httpResp, doErr := r.httpClient.Do(httpReq)
		if doErr != nil {
			return errors.Join(goflux.ErrPublish, goflux.ErrTransport, fmt.Errorf("http: %w", doErr))
		}
		defer httpResp.Body.Close()

		respBody, readErr := io.ReadAll(io.LimitReader(httpResp.Body, 1<<20))
		if readErr != nil {
			return errors.Join(goflux.ErrTransport, fmt.Errorf("http: %w", readErr))
		}

		if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
			return errors.Join(goflux.ErrTransport, fmt.Errorf("http: server returned %d for %s", httpResp.StatusCode, target))
		}

		if decErr := r.respCodec.Decode(respBody, &result); decErr != nil {
			return errors.Join(goflux.ErrDecode, fmt.Errorf("http: %w", decErr))
		}

		return nil
	})

	return result, err
}

// Close is a no-op.
func (r *Requester[Req, Resp]) Close() error { return nil }
