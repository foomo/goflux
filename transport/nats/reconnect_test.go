package nats_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/foomo/goencode/json/v1"
	"github.com/foomo/goflux"
	fluxnats "github.com/foomo/goflux/transport/nats"
	"github.com/foomo/gofuncy"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
)

// TestSubscriber_serverReplacement asserts that a subscription survives the
// server being replaced at the same address: the original subscriber must
// receive a message published after the client reconnects to the new server.
// Subscribe is called exactly once and never retried.
func TestSubscriber_serverReplacement(t *testing.T) {
	srv1 := startServerOnPort(t, -1)

	addr, ok := srv1.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected listener address %v", srv1.Addr())
	}

	port := addr.Port

	subConn, subDisconnected, subReconnected := connectWithReconnect(t, srv1.ClientURL())
	pubConn, _, pubReconnected := connectWithReconnect(t, srv1.ClientURL())

	sub := fluxnats.NewSubscriber(subConn, json.NewCodec[Event]().Decode)
	pub := fluxnats.NewPublisher(pubConn, json.NewCodec[Event]().Encode)

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	const subject = "reconnect.events"

	received := make(chan Event, 8)

	gofuncy.StartWithReady(ctx, func(ctx context.Context, ready gofuncy.ReadyFunc) error {
		return sub.SubscribeWithReady(ctx, subject, func(_ context.Context, msg goflux.Message[Event]) error {
			received <- msg.Payload
			return nil
		}, ready)
	}, gofuncy.WithName("test.subscriber"))

	publishAndFlush(t, ctx, pub, pubConn, subject, Event{ID: "before"})
	expectEvent(t, received, "before", 5*time.Second)

	// Replace the server: same address, fresh state.
	srv1.Shutdown()
	waitFor(t, subDisconnected, 5*time.Second, "subscriber disconnect")

	srv2 := startServerOnPort(t, port)
	t.Logf("server replaced: %s -> %s", srv1.ID(), srv2.ID())

	waitFor(t, subReconnected, 10*time.Second, "subscriber reconnect")
	waitFor(t, pubReconnected, 10*time.Second, "publisher reconnect")

	// ReconnectHandler fires once the replayed SUB is written, not once the
	// server has processed it. A publish from another connection can win that
	// race and core NATS drops it (at-most-once). The round trip closes the
	// window so the test checks that the subscription survives, not that race.
	if err := subConn.Flush(); err != nil {
		t.Fatalf("subscriber flush: %s", err)
	}

	publishAndFlush(t, ctx, pub, pubConn, subject, Event{ID: "after"})
	expectEvent(t, received, "after", 5*time.Second)
}

func startServerOnPort(t *testing.T, port int) *natsserver.Server {
	t.Helper()

	srv, err := natsserver.NewServer(&natsserver.Options{
		Host:      "127.0.0.1",
		Port:      port,
		NoLog:     true,
		JetStream: true,
		StoreDir:  t.TempDir(),
	})
	if err != nil {
		t.Fatalf("nats server: %s", err)
	}

	srv.Start()
	t.Cleanup(srv.Shutdown)

	if !srv.ReadyForConnections(5 * time.Second) {
		t.Fatal("nats server not ready")
	}

	return srv
}

func connectWithReconnect(t *testing.T, url string) (*nats.Conn, <-chan struct{}, <-chan struct{}) {
	t.Helper()

	disconnected := make(chan struct{}, 1)
	reconnected := make(chan struct{}, 1)
	signal := func(ch chan struct{}) {
		select {
		case ch <- struct{}{}:
		default:
		}
	}

	conn, err := nats.Connect(url,
		nats.Timeout(10*time.Second),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(50*time.Millisecond),
		nats.DisconnectErrHandler(func(*nats.Conn, error) { signal(disconnected) }),
		nats.ReconnectHandler(func(*nats.Conn) { signal(reconnected) }),
	)
	if err != nil {
		t.Fatalf("nats connect: %s", err)
	}

	t.Cleanup(conn.Close)

	return conn, disconnected, reconnected
}

func publishAndFlush(t *testing.T, ctx context.Context, pub *fluxnats.Publisher[Event], conn *nats.Conn, subject string, v Event) {
	t.Helper()

	if err := pub.Publish(ctx, subject, v); err != nil {
		t.Fatalf("publish %q: %s", v.ID, err)
	}

	if err := conn.Flush(); err != nil {
		t.Fatalf("flush %q: %s", v.ID, err)
	}
}

func waitFor(t *testing.T, ch <-chan struct{}, timeout time.Duration, what string) {
	t.Helper()

	select {
	case <-ch:
	case <-time.After(timeout):
		t.Fatalf("timed out waiting for %s", what)
	}
}

func expectEvent(t *testing.T, received <-chan Event, id string, timeout time.Duration) {
	t.Helper()

	select {
	case got := <-received:
		if got.ID != id {
			t.Fatalf("expected event %q, got %q", id, got.ID)
		}
	case <-time.After(timeout):
		t.Fatalf("original subscriber did not receive %q within %s", id, timeout)
	}
}
