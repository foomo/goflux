package nats_test

import (
	"testing"

	"github.com/foomo/goflux/subject/nats"
)

func TestNewDomain_invalidNamePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid domain name")
		}
	}()

	nats.NewDomain("")
}
