package jetstream_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/foomo/goencode/json/v1"
	"github.com/foomo/goflux"
	fluxjetstream "github.com/foomo/goflux/transport/jetstream"
	"github.com/foomo/gofuncy"
	"github.com/nats-io/nats.go/jetstream"
)

// Compile-time interface compliance check.
var _ goflux.Subscriber[Event] = (*fluxjetstream.Subscriber[Event])(nil)

func ExampleNewSubscriber() {
	srv, conn := newServer()
	defer srv.Shutdown()
	defer conn.Close()

	js, err := fluxjetstream.NewStream(context.Background(), conn,
		jetstream.StreamConfig{
			Name:     "EVENTS",
			Subjects: []string{"events.>"},
		})
	if err != nil {
		panic(err)
	}

	cons, err := js.CreateConsumer(context.Background(), "EVENTS", jetstream.ConsumerConfig{
		Durable:   "processor",
		AckPolicy: jetstream.AckExplicitPolicy,
	})
	if err != nil {
		panic(err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	var once sync.Once

	sub := fluxjetstream.NewSubscriber(cons, json.NewCodec[Event]().Decode)

	gofuncy.Start(ctx, func(ctx context.Context) error {
		return sub.Subscribe(ctx, "events.created", func(_ context.Context, msg goflux.Message[Event]) error {
			once.Do(func() {
				fmt.Println(msg.Subject, msg.Payload)
				cancel()
			})

			return nil
		})
	})

	// Publish a raw message via JetStream.
	if _, err := js.Publish(context.Background(), "events.created", []byte(`{"id":"1","name":"foo"}`)); err != nil {
		panic(err)
	}

	<-ctx.Done()

	// Output: events.created {1 foo}
}

// Compile-time check: the subscriber supports readiness signalling.
var _ goflux.ReadySubscriber[Event] = (*fluxjetstream.Subscriber[Event])(nil)

func TestPullExpiry(t *testing.T) {
	t.Parallel()

	errHandler := jetstream.ConsumeErrHandler(func(jetstream.ConsumeContext, error) {})

	for name, tc := range map[string]struct {
		opts []jetstream.PullConsumeOpt
		want time.Duration
	}{
		"goflux default": {
			opts: []jetstream.PullConsumeOpt{jetstream.PullExpiry(fluxjetstream.DefaultPullExpiry), errHandler},
			want: fluxjetstream.DefaultPullExpiry,
		},
		"caller overrides default": {
			opts: []jetstream.PullConsumeOpt{jetstream.PullExpiry(fluxjetstream.DefaultPullExpiry), errHandler, jetstream.PullExpiry(time.Minute)},
			want: time.Minute,
		},
		"fallback without expiry": {
			opts: []jetstream.PullConsumeOpt{errHandler},
			want: jetstream.DefaultExpires,
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if got := fluxjetstream.PullExpiry(tc.opts); got != tc.want {
				t.Fatalf("expected %s, got %s", tc.want, got)
			}
		})
	}
}
