package nats_test

import (
	"testing"

	"github.com/foomo/goflux/subject/nats"
)

func TestDomain(t *testing.T) {
	t.Run("from prefix", func(t *testing.T) {
		d := nats.New("prod", "acme").Domain("user")
		if got := d.String(); got != "prod.acme.user" {
			t.Errorf("got %q, want %q", got, "prod.acme.user")
		}
	})

	t.Run("from empty prefix", func(t *testing.T) {
		d := nats.New().Domain("user")
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
		d := nats.New("prod", "acme").Domain("user")
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
