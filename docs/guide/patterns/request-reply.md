# Request-Reply

Request-reply is a synchronous messaging pattern: the caller sends a typed request and blocks until it receives a typed response. goflux models this with two interfaces:

```go
type Requester[Req, Resp any] interface {
	Request(ctx context.Context, subject string, req Req) (Resp, error)
	Close() error
}

type Responder[Req, Resp any] interface {
	Serve(ctx context.Context, subject string, handler RequestHandler[Req, Resp]) error
	Close() error
}

type RequestHandler[Req, Resp any] func(ctx context.Context, req Req) (Resp, error)
```

Both NATS and HTTP transports implement these interfaces.

## NATS Request-Reply

NATS has native request-reply support. The requester publishes to a subject with a unique reply inbox; the responder subscribes, processes the request, and responds on that inbox.

### Requester

```go
package main

import (
	"context"
	"fmt"
	"log"

	json "github.com/foomo/goencode/json/v1"
	gofluxnats "github.com/foomo/goflux/transport/nats"
	"github.com/nats-io/nats.go"
)

type OrderReq struct {
	Item     string
	Quantity int
}

type OrderResp struct {
	ID    string
	Total float64
}

func main() {
	ctx := context.Background()

	conn, _ := nats.Connect(nats.DefaultURL)
	defer conn.Drain()

	reqCodec := json.NewCodec[OrderReq]()
	respCodec := json.NewCodec[OrderResp]()

	req := gofluxnats.NewRequester[OrderReq, OrderResp](conn, reqCodec, respCodec)
	defer req.Close()

	resp, err := req.Request(ctx, "orders.create", OrderReq{
		Item:     "widget",
		Quantity: 3,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("order %s created, total: %.2f\n", resp.ID, resp.Total)
}
```

### Responder

`Serve` blocks until `ctx` is cancelled, just like `Subscribe`.

```go
package main

import (
	"context"
	"fmt"

	json "github.com/foomo/goencode/json/v1"
	gofluxnats "github.com/foomo/goflux/transport/nats"
	"github.com/nats-io/nats.go"
)

type OrderReq struct {
	Item     string
	Quantity int
}

type OrderResp struct {
	ID    string
	Total float64
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	conn, _ := nats.Connect(nats.DefaultURL)
	defer conn.Drain()

	reqCodec := json.NewCodec[OrderReq]()
	respCodec := json.NewCodec[OrderResp]()

	res := gofluxnats.NewResponder[OrderReq, OrderResp](conn, reqCodec, respCodec)

	// Blocks until ctx is cancelled.
	_ = res.Serve(ctx, "orders.create", func(ctx context.Context, req OrderReq) (OrderResp, error) {
		id := fmt.Sprintf("ord-%s-%d", req.Item, req.Quantity)
		return OrderResp{
			ID:    id,
			Total: float64(req.Quantity) * 9.99,
		}, nil
	})
}
```

## HTTP Request-Reply

The HTTP transport sends requests as POST to `{baseURL}/{subject}` and deserializes the response body. This is useful for cross-service RPC over HTTP without a full RPC framework.

### Requester

```go
reqCodec := json.NewCodec[OrderReq]()
respCodec := json.NewCodec[OrderResp]()

// Pass nil for *http.Client to use a default client with 10s timeout.
req := gofluxhttp.NewRequester[OrderReq, OrderResp](
	"https://api.internal",
	reqCodec,
	respCodec,
	nil, // *http.Client
)

resp, err := req.Request(ctx, "orders.create", OrderReq{Item: "widget", Quantity: 1})
```

### Responder

The HTTP responder exposes a `*http.ServeMux` via the `Mux()` method. You mount this mux on your HTTP server -- the responder does not own the listener.

```go
reqCodec := json.NewCodec[OrderReq]()
respCodec := json.NewCodec[OrderResp]()

res := gofluxhttp.NewResponder[OrderReq, OrderResp](reqCodec, respCodec)

go func() {
	_ = res.Serve(ctx, "orders.create", func(ctx context.Context, req OrderReq) (OrderResp, error) {
		return OrderResp{ID: "ord-1", Total: 9.99}, nil
	})
}()

// Mount on any HTTP server.
http.ListenAndServe(":8080", res.Mux())
```

## Trace Context Propagation

Both NATS and HTTP transports automatically inject OTel trace context into outgoing requests and extract it on the responder side. No additional configuration is needed.

## When to Use

- **Synchronous RPC** -- when the caller needs the result before proceeding.
- **Service-to-service calls** -- typed request/response over NATS or HTTP without code generation.
- **Query patterns** -- fetching data from another service in a request-scoped context.

For asynchronous, one-way messaging, use [fire-and-forget](./fire-and-forget) or [at-least-once](./at-least-once) instead.
