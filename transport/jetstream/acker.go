package jetstream

import (
	"time"

	"github.com/nats-io/nats.go/jetstream"
)

// jsAcker wraps a jetstream.Msg to implement goflux.Acker, goflux.DelayedNaker,
// and goflux.Terminator.
type jsAcker struct {
	msg jetstream.Msg
}

func (a *jsAcker) Ack() error                         { return a.msg.Ack() }
func (a *jsAcker) Nak() error                         { return a.msg.Nak() }
func (a *jsAcker) NakWithDelay(d time.Duration) error { return a.msg.NakWithDelay(d) }
func (a *jsAcker) Term() error                        { return a.msg.Term() }
