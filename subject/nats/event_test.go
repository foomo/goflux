package nats_test

import (
	"testing"

	"github.com/foomo/goflux/subject/nats"
)

func TestEvent(t *testing.T) {
	t.Run("full chain with prefix", func(t *testing.T) {
		ev := nats.New("prod", "acme").Domain("user").Entity("profile").Event("updated")
		if got := ev.String(); got != "prod.acme.user.profile.updated" {
			t.Errorf("got %q, want %q", got, "prod.acme.user.profile.updated")
		}
	})

	t.Run("no prefix", func(t *testing.T) {
		ev := nats.NewDomain("order").Entity("item").Event("created")
		if got := ev.String(); got != "order.item.created" {
			t.Errorf("got %q, want %q", got, "order.item.created")
		}
	})

	t.Run("invalid name panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for invalid event name")
			}
		}()

		nats.NewDomain("user").Entity("profile").Event("")
	})
}
