package nats_test

import (
	"testing"

	"github.com/foomo/goflux/subject/nats"
)

func TestNew_invalidSegmentPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for invalid prefix segment")
		}
	}()

	nats.New("prod", "bad.segment")
}
