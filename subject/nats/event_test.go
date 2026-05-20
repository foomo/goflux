package nats_test

import (
	"testing"

	"github.com/foomo/goflux/subject/nats"
)

func TestEvent_invalidNamePanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid event name")
		}
	}()

	nats.NewDomain("user").Entity("profile").Event("")
}
