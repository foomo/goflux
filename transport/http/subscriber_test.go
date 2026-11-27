package http_test

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/foomo/goencode/json/v1"
	"github.com/foomo/goflux"
	"github.com/foomo/goflux/transport/http"
)

// Compile-time check: the subscriber supports readiness signalling.
var _ goflux.ReadySubscriber[Event] = (*http.Subscriber[Event])(nil)

// TestSubscriber_messageID asserts that the handler receives the publisher's
// message ID as msg.ID but not in its context, so its publishes do not
// inherit it.
func TestSubscriber_messageID(t *testing.T) {
	t.Parallel()

	ids := make(chan [2]string, 1)
	sub := http.NewSubscriber(json.NewCodec[Event]().Decode)

	s := httptest.NewServer(sub.Handler("events", func(ctx context.Context, msg goflux.Message[Event]) error {
		ids <- [2]string{msg.ID, goflux.MessageID(ctx)}

		return nil
	}))
	t.Cleanup(s.Close)

	pub := http.NewPublisher(s.URL, json.NewCodec[Event]().Encode, s.Client())
	if err := pub.Publish(goflux.WithMessageID(t.Context(), "id-1"), "events", Event{ID: "1"}); err != nil {
		t.Fatal(err)
	}

	if got := <-ids; got != [2]string{"id-1", ""} {
		t.Fatalf("expected msg.ID id-1 and no ctx ID, got %v", got)
	}
}
