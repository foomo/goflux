package jetstream_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/foomo/goencode/json/v1"
	"github.com/foomo/goflux"
	fluxjetstream "github.com/foomo/goflux/transport/jetstream"
	"github.com/foomo/gofuncy"
	"github.com/nats-io/nats.go/jetstream"
)

func TestSubscriber_DeliveryAndInProgress(t *testing.T) {
	t.Parallel()

	srv, conn := newServer()
	defer srv.Shutdown()
	defer conn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	js, err := fluxjetstream.NewStream(ctx, conn, jetstream.StreamConfig{
		Name:     "IMPORTS",
		Subjects: []string{"imports.>"},
	})
	if err != nil {
		t.Fatal(err)
	}

	cons, err := js.CreateConsumer(ctx, "IMPORTS", jetstream.ConsumerConfig{
		Durable:   "importer",
		AckPolicy: jetstream.AckExplicitPolicy,
	})
	if err != nil {
		t.Fatal(err)
	}

	sub := fluxjetstream.NewSubscriber(cons, json.NewCodec[Event]().Decode)

	deliveries := make(chan goflux.Delivery, 2)
	inProgressErrs := make(chan error, 2)

	gofuncy.Start(ctx, func(ctx context.Context) error {
		return sub.Subscribe(ctx, "imports.run", func(_ context.Context, msg goflux.Message[Event]) error {
			inProgressErrs <- msg.InProgress()

			d := msg.Delivery()
			deliveries <- d

			if d.NumDelivered == 1 {
				return errors.New("fail first delivery") // transport naks → redelivery
			}

			return nil
		})
	})

	if _, err := js.Publish(ctx, "imports.run", []byte(`{"id":"1","name":"import"}`)); err != nil {
		t.Fatal(err)
	}

	for want := uint64(1); want <= 2; want++ {
		select {
		case d := <-deliveries:
			if d.NumDelivered != want {
				t.Fatalf("expected NumDelivered %d, got %d", want, d.NumDelivered)
			}

			if d.Timestamp.IsZero() {
				t.Fatal("expected non-zero Timestamp")
			}
		case <-ctx.Done():
			t.Fatalf("timed out waiting for delivery %d", want)
		}

		if err := <-inProgressErrs; err != nil {
			t.Fatalf("InProgress: %v", err)
		}
	}
}
