# HTTP Transport

Package `github.com/foomo/goflux/transport/http`

The HTTP transport sends messages as HTTP POST requests and receives them via an `*http.ServeMux`. The subscriber does not own a listener -- the caller mounts the mux on any `http.Server`.

## Interfaces

| Interface | Implemented |
|-----------|-------------|
| `Publisher[T]` | Yes |
| `Subscriber[T]` | Yes |
| `Requester[Req, Resp]` | Yes |
| `Responder[Req, Resp]` | Yes |

## Publisher

```go
func NewPublisher[T any](baseURL string, codec goencode.Codec[T], client *http.Client, opts ...PublisherOption) *Publisher[T]
```

`Publish` encodes the value and POSTs it to `{baseURL}/{subject}`. OTel context is injected into HTTP headers via the standard W3C `propagation.HeaderCarrier`. goflux context headers are sent as `X-Goflux-{key}` HTTP headers.

If `client` is nil, a default `*http.Client` with a 10-second timeout is used. A non-2xx response is treated as an error.

`Close` is a no-op.

## Subscriber

```go
func NewSubscriber[T any](codec goencode.Codec[T], opts ...SubscriberOption) *Subscriber[T]
```

`Subscribe` registers a handler for `POST {basePath}/{subject}` on the subscriber's internal `*http.ServeMux`, then blocks until the context is cancelled. The subscriber exposes two ways to integrate with an HTTP server:

- **`Subscribe`** -- registers the route and blocks. Use when running in a goroutine alongside the server.
- **`Handler(subject, handler)`** -- returns an `http.HandlerFunc` for the subject, allowing direct registration on an external mux.

`Close` is a no-op. Shutdown is handled by the outer `http.Server`.

### Response Codes

| Code | Meaning |
|------|---------|
| 204 | Handler returned nil (success) |
| 400 | Request body decode failure |
| 405 | Method other than POST |
| 413 | Body exceeds `MaxBodySize` |
| 500 | Handler returned an error |

## Requester

```go
func NewRequester[Req, Resp any](
    baseURL string,
    reqCodec goencode.Codec[Req],
    respCodec goencode.Codec[Resp],
    client *http.Client,
    opts ...PublisherOption,
) *Requester[Req, Resp]
```

`Request` encodes the request, POSTs it to `{baseURL}/{subject}`, reads the response body, and decodes it. If `client` is nil, a default `*http.Client` with a 10-second timeout is used.

`Close` is a no-op.

## Responder

```go
func NewResponder[Req, Resp any](
    reqCodec goencode.Codec[Req],
    respCodec goencode.Codec[Resp],
    opts ...SubscriberOption,
) *Responder[Req, Resp]
```

`Serve` registers the handler for `POST {basePath}/{subject}` and blocks until the context is cancelled. The responder decodes the incoming request body, passes it to the `goflux.RequestHandler[Req, Resp]`, encodes the response, and writes it back with `Content-Type: application/json` and status 200.

`Mux()` returns the underlying `*http.ServeMux` for mounting on an HTTP server.

`Close` is a no-op.

## Options

### Publisher Options

| Option | Description |
|--------|-------------|
| `WithPublisherTelemetry(t *goflux.Telemetry)` | Sets the OTel telemetry instance. A default is created from OTel globals if not provided. |

### Subscriber Options

| Option | Description |
|--------|-------------|
| `WithTelemetry(t *goflux.Telemetry)` | Sets the OTel telemetry instance. A default is created from OTel globals if not provided. |
| `WithMaxBodySize(n int64)` | Maximum request body size. Requests exceeding this limit receive 413. Default: 1 MiB. |
| `WithBasePath(path string)` | Prefix for all registered routes. Default: empty string. |

## Behavior

- **Caller owns the server** -- the subscriber and responder expose a mux or handler; they do not start a listener. Mount them on your own `http.Server` or framework.
- **Headers** -- goflux context headers are prefixed with `X-Goflux-` in HTTP headers. On the subscriber side, `X-Goflux-*` headers are extracted and stripped of the prefix.
- **OTel context propagation** -- HTTP is synchronous, so the transport uses parent-child spans (not links) via `ExtractContext` / `InjectContext` with the standard `propagation.HeaderCarrier`.
- **Message ID** -- if `goflux.MessageID(ctx)` is set, it is propagated via the `X-Message-ID` header.
- **Close** -- all `Close()` methods are no-ops. Lifecycle management belongs to the outer HTTP server.

## Pub/Sub Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/foomo/goencode/json"
	"github.com/foomo/goflux"
	gofluxhttp "github.com/foomo/goflux/transport/http"
)

type Event struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	codec := json.NewCodec[Event]()

	// Subscriber -- exposes an HTTP handler, does not own a listener.
	sub := gofluxhttp.NewSubscriber[Event](codec, gofluxhttp.WithBasePath("/hooks"))

	// Register a handler for POST /hooks/events.created
	handler := sub.Handler("events.created", func(ctx context.Context, msg goflux.Message[Event]) error {
		fmt.Printf("received: %s %s\n", msg.Payload.ID, msg.Payload.Name)
		return nil
	})

	mux := http.NewServeMux()
	mux.Handle("/hooks/events.created", handler)

	// Start the HTTP server.
	srv := &http.Server{Addr: ":8080", Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Publisher -- POSTs to the subscriber's endpoint.
	pub := gofluxhttp.NewPublisher[Event]("http://localhost:8080/hooks", codec, nil)

	if err := pub.Publish(ctx, "events.created", Event{ID: "1", Name: "signup"}); err != nil {
		log.Fatal(err)
	}
}
```

## Request-Reply Example

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/foomo/goencode/json"
	gofluxhttp "github.com/foomo/goflux/transport/http"
)

type OrderRequest struct {
	ItemID string `json:"item_id"`
	Qty    int    `json:"qty"`
}

type OrderResponse struct {
	OrderID string `json:"order_id"`
	Status  string `json:"status"`
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reqCodec := json.NewCodec[OrderRequest]()
	respCodec := json.NewCodec[OrderResponse]()

	// Responder -- handles incoming requests.
	responder := gofluxhttp.NewResponder[OrderRequest, OrderResponse](reqCodec, respCodec)
	go func() {
		_ = responder.Serve(ctx, "orders.create", func(ctx context.Context, req OrderRequest) (OrderResponse, error) {
			return OrderResponse{OrderID: "ord-42", Status: "created"}, nil
		})
	}()

	// Mount the responder's mux on an HTTP server.
	srv := &http.Server{Addr: ":8081", Handler: responder.Mux()}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	// Requester -- sends a request and receives a response.
	requester := gofluxhttp.NewRequester[OrderRequest, OrderResponse](
		"http://localhost:8081", reqCodec, respCodec, nil,
	)

	resp, err := requester.Request(ctx, "orders.create", OrderRequest{ItemID: "sku-1", Qty: 3})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("order: %s status: %s\n", resp.OrderID, resp.Status)
}
```
