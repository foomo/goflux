package jetstream

import (
	"time"

	"github.com/foomo/goflux"
	"github.com/nats-io/nats.go/jetstream"
)

// jsAcker wraps a jetstream.Msg to implement goflux.Acker, goflux.DelayedNaker,
// goflux.Terminator, goflux.InProgresser and goflux.DeliveryReporter.
type jsAcker struct {
	msg jetstream.Msg
}

func (a *jsAcker) Ack() error                         { return a.msg.Ack() }
func (a *jsAcker) Nak() error                         { return a.msg.Nak() }
func (a *jsAcker) NakWithDelay(d time.Duration) error { return a.msg.NakWithDelay(d) }
func (a *jsAcker) Term() error                        { return a.msg.Term() }
func (a *jsAcker) InProgress() error                  { return a.msg.InProgress() }

// Delivery parses the delivery metadata from the message's reply subject.
// Returns the zero value if the metadata cannot be parsed.
func (a *jsAcker) Delivery() goflux.Delivery {
	md, err := a.msg.Metadata()
	if err != nil {
		return goflux.Delivery{}
	}

	return goflux.Delivery{
		NumDelivered: md.NumDelivered,
		Timestamp:    md.Timestamp,
	}
}
