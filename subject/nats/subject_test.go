package nats_test

import (
	"testing"

	"github.com/foomo/goflux/subject/nats"
)

func TestNewPrefix(t *testing.T) {
	t.Run("zero segments", func(t *testing.T) {
		p := nats.NewPrefix()
		if got := p.String(); got != "" {
			t.Errorf("got %q, want %q", got, "")
		}
	})

	t.Run("single segment", func(t *testing.T) {
		p := nats.NewPrefix("prod")
		if got := p.String(); got != "prod" {
			t.Errorf("got %q, want %q", got, "prod")
		}
	})

	t.Run("multiple segments", func(t *testing.T) {
		p := nats.NewPrefix("prod", "acme")
		if got := p.String(); got != "prod.acme" {
			t.Errorf("got %q, want %q", got, "prod.acme")
		}
	})

	t.Run("invalid segment panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for invalid prefix segment")
			}
		}()

		nats.NewPrefix("prod", "bad.segment")
	})
}

func TestDomain(t *testing.T) {
	t.Run("from prefix", func(t *testing.T) {
		d := nats.NewPrefix("prod", "acme").Domain("user")
		if got := d.String(); got != "prod.acme.user" {
			t.Errorf("got %q, want %q", got, "prod.acme.user")
		}
	})

	t.Run("from empty prefix", func(t *testing.T) {
		d := nats.NewPrefix().Domain("user")
		if got := d.String(); got != "user" {
			t.Errorf("got %q, want %q", got, "user")
		}
	})

	t.Run("NewDomain shortcut", func(t *testing.T) {
		d := nats.NewDomain("order")
		if got := d.String(); got != "order" {
			t.Errorf("got %q, want %q", got, "order")
		}
	})

	t.Run("All wildcard from prefix", func(t *testing.T) {
		d := nats.NewPrefix("prod", "acme").Domain("user")
		if got := d.All(); got != "prod.acme.user.>" {
			t.Errorf("got %q, want %q", got, "prod.acme.user.>")
		}
	})

	t.Run("All wildcard no prefix", func(t *testing.T) {
		d := nats.NewDomain("user")
		if got := d.All(); got != "user.>" {
			t.Errorf("got %q, want %q", got, "user.>")
		}
	})

	t.Run("invalid name panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for invalid domain name")
			}
		}()

		nats.NewDomain("")
	})
}

func TestEntity(t *testing.T) {
	t.Run("full chain", func(t *testing.T) {
		e := nats.NewPrefix("prod", "acme").Domain("user").Entity("profile")
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
		e := nats.NewPrefix("prod", "acme").Domain("user").Entity("profile")
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

func TestEvent(t *testing.T) {
	t.Run("full chain with prefix", func(t *testing.T) {
		ev := nats.NewPrefix("prod", "acme").Domain("user").Entity("profile").Event("updated")
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
