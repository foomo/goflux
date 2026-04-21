package nats_test

import (
	"testing"

	"github.com/foomo/goflux/subject/nats"
)

func TestEntity(t *testing.T) {
	t.Run("full chain", func(t *testing.T) {
		e := nats.New("prod", "acme").Domain("user").Entity("profile")
		if got := e.String(); got != "prod.acme.user.profile" {
			t.Errorf("got %q, want %q", got, "prod.acme.user.profile")
		}
	})

	t.Run("no prefix", func(t *testing.T) {
		e := nats.NewDomain("user").Entity("profile")
		if got := e.String(); got != "user.profile" {
			t.Errorf("got %q, want %q", got, "user.profile")
		}
	})

	t.Run("All wildcard", func(t *testing.T) {
		e := nats.New("prod", "acme").Domain("user").Entity("profile")
		if got := e.All(); got != "prod.acme.user.profile.>" {
			t.Errorf("got %q, want %q", got, "prod.acme.user.profile.>")
		}
	})

	t.Run("All wildcard no prefix", func(t *testing.T) {
		e := nats.NewDomain("order").Entity("item")
		if got := e.All(); got != "order.item.>" {
			t.Errorf("got %q, want %q", got, "order.item.>")
		}
	})

	t.Run("invalid name panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for invalid entity name")
			}
		}()

		nats.NewDomain("user").Entity("")
	})
}
