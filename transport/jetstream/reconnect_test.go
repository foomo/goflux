package jetstream_test

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/foomo/goencode/json/v1"
	"github.com/foomo/goflux"
	fluxjetstream "github.com/foomo/goflux/transport/jetstream"
	natsserver "github.com/nats-io/nats-server/v2/server"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// TestSubscriber_serverReplacementConsumerRecreated models a NATS pod restart
// with in-memory JetStream state under NACK (--control-loop): the server is
// replaced at the same address with empty storage, then the stream and the
// durable consumer are recreated with the same configuration. The original
// Subscribe call, never retried, must receive a message published afterwards
// within about one pull expiry.
func TestSubscriber_serverReplacementConsumerRecreated(t *testing.T) {
	srv1 := startServerOnPort(t, -1)

	addr, ok := srv1.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("unexpected listener address %v", srv1.Addr())
	}

	port := addr.Port

	subConn, subDisconnected, subReconnected := connectWithReconnect(t, srv1.ClientURL())
	pubConn, _, pubReconnected := connectWithReconnect(t, srv1.ClientURL())

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	const (
		streamName = "RECONNECT"
		subject    = "reconnect.events"
		durable    = "reconnect_consumer"
	)

	streamCfg := jetstream.StreamConfig{
		Name:     streamName,
		Subjects: []string{subject},
		Storage:  jetstream.MemoryStorage,
	}

	subJS, err := fluxjetstream.NewStream(ctx, subConn, streamCfg)
	if err != nil {
		t.Fatalf("create stream: %s", err)
	}

	consumerCfg := jetstream.ConsumerConfig{
		Durable:   durable,
		AckPolicy: jetstream.AckExplicitPolicy,
	}

	cons, err := subJS.CreateOrUpdateConsumer(ctx, streamName, consumerCfg)
	if err != nil {
		t.Fatalf("create consumer: %s", err)
	}

	pubJS, err := jetstream.New(pubConn)
	if err != nil {
		t.Fatalf("jetstream: %s", err)
	}

	sub := fluxjetstream.NewSubscriber(cons, json.NewCodec[Event]().Decode)

	received := make(chan Event, 8)
	ready := make(chan struct{})

	go func() {
		_ = sub.SubscribeWithReady(ctx, subject, func(_ context.Context, msg goflux.Message[Event]) error {
			received <- msg.Payload
			return nil
		}, func() { close(ready) })
	}()

	waitFor(t, ready, 10*time.Second, "subscription ready")

	publishStored(t, ctx, pubJS, subject, "before")
	expectEvent(t, received, "before", 5*time.Second)

	// Replace the server: same address, empty JetStream storage.
	srv1.Shutdown()
	waitFor(t, subDisconnected, 5*time.Second, "subscriber disconnect")

	srv2 := startServerOnPort(t, port)
	t.Logf("server replaced: %s -> %s", srv1.ID(), srv2.ID())

	waitFor(t, subReconnected, 10*time.Second, "subscriber reconnect")
	waitFor(t, pubReconnected, 10*time.Second, "publisher reconnect")

	// Stand-in for NACK's reconcile: recreate stream and consumer.
	if _, err := fluxjetstream.NewStream(ctx, pubConn, streamCfg); err != nil {
		t.Fatalf("recreate stream: %s", err)
	}

	if _, err := pubJS.CreateOrUpdateConsumer(ctx, streamName, consumerCfg); err != nil {
		t.Fatalf("recreate consumer: %s", err)
	}

	publishStored(t, ctx, pubJS, subject, "after")

	deadline := fluxjetstream.DefaultPullExpiry + 5*time.Second

	select {
	case got := <-received:
		if got.ID != "after" {
			t.Fatalf(`expected event "after", got %q`, got.ID)
		}
	case <-time.After(deadline):
		_, consErr := pubJS.Consumer(ctx, streamName, durable)
		t.Logf("consumer %q lookup on replacement server: err=%v", durable, consErr)

		if s, err := pubJS.Stream(ctx, streamName); err == nil {
			info, _ := s.Info(ctx)
			t.Logf("stream %q: msgs=%d consumers=%d", streamName, info.State.Msgs, info.State.Consumers)
		}

		t.Fatalf(`original subscriber did not receive "after" within %s`, deadline)
	}
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

func publishStored(t *testing.T, ctx context.Context, js jetstream.JetStream, subject, id string) {
	t.Helper()

	ack, err := js.Publish(ctx, subject, []byte(`{"id":"`+id+`"}`))
	if err != nil {
		t.Fatalf("publish %q: %s", id, err)
	}

	t.Logf("published %q: stream=%s seq=%d", id, ack.Stream, ack.Sequence)
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
