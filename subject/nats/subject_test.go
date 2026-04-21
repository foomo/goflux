package nats_test

import (
	"testing"

	"github.com/foomo/goflux/subject/nats"
)

func TestNew(t *testing.T) {
	t.Run("zero segments", func(t *testing.T) {
		p := nats.New()
		if got := p.String(); got != "" {
			t.Errorf("got %q, want %q", got, "")
		}
	})

	t.Run("single segment", func(t *testing.T) {
		p := nats.New("prod")
		if got := p.String(); got != "prod" {
			t.Errorf("got %q, want %q", got, "prod")
		}
	})

	t.Run("multiple segments", func(t *testing.T) {
		p := nats.New("prod", "acme")
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

		nats.New("prod", "bad.segment")
	})
}
