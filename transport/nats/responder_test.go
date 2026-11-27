package nats_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/foomo/goencode/json/v1"
	"github.com/foomo/goflux"
	fluxnats "github.com/foomo/goflux/transport/nats"
	"github.com/foomo/gofuncy"
)

func ExampleNewResponder() {
	srv, conn := newServer()
	defer srv.Shutdown()
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	resp := fluxnats.NewResponder[Event, Event](
		conn,
		json.NewCodec[Event](),
		json.NewCodec[Event](),
	)

	// Start the responder in the background.
	ready := make(chan struct{})

	go func() {
		_ = resp.ServeWithReady(ctx, "greet", func(_ context.Context, req Event) (Event, error) {
			return Event{ID: req.ID, Name: "hello " + req.Name}, nil
		}, func() { close(ready) })
	}()

	// Wait until the server has registered the subscription. Requesting before
	// then yields "no responders available".
	<-ready

	// Send a request via a separate Requester.
	requester := fluxnats.NewRequester[Event, Event](
		conn,
		json.NewCodec[Event](),
		json.NewCodec[Event](),
	)

	reqCtx, reqCancel := context.WithTimeout(ctx, 5*time.Second)
	defer reqCancel()

	reply, err := requester.Request(reqCtx, "greet", Event{ID: "1", Name: "world"})
	if err != nil {
		panic(err)
	}

	fmt.Println(reply.ID, reply.Name)
	// Output: 1 hello world
}

// Compile-time interface compliance check.
var _ goflux.Responder[Event, Event] = (*fluxnats.Responder[Event, Event])(nil)

// TestResponder_messageIDNotInContext asserts that the responder does not put
// the request's message ID into the handler context, so publishes from the
// handler do not inherit it.
func TestResponder_messageIDNotInContext(t *testing.T) {
	t.Parallel()

	srv, conn := newServer()
	t.Cleanup(srv.Shutdown)
	t.Cleanup(conn.Close)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	resp := fluxnats.NewResponder[Event, Event](conn, json.NewCodec[Event](), json.NewCodec[Event]())

	done := make(chan struct{})

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		defer close(done)

		return resp.ServeWithReady(ctx, "greet", func(ctx context.Context, req Event) (Event, error) {
			return Event{ID: req.ID, Name: goflux.MessageID(ctx)}, nil
		}, ready)
	}, gofuncy.WithName("test.responder"))

	requester := fluxnats.NewRequester[Event, Event](conn, json.NewCodec[Event](), json.NewCodec[Event]())

	reqCtx, reqCancel := context.WithTimeout(goflux.WithMessageID(ctx, "id-1"), 5*time.Second)
	defer reqCancel()

	reply, err := requester.Request(reqCtx, "greet", Event{ID: "1"})
	if err != nil {
		t.Fatal(err)
	}

	if reply.Name != "" {
		t.Fatalf("expected no message ID in handler ctx, got %q", reply.Name)
	}

	reqCancel()
	cancel()
	<-done
}
