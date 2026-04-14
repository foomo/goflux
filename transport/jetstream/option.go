package jetstream

import (
	"github.com/foomo/goflux"
)

// Option configures a JetStream Publisher, Subscriber, or Consumer.
type Option func(*config)

type config struct {
	tel       *goflux.Telemetry
	manualAck bool
}

// WithTelemetry sets the Telemetry instance for the transport. If not provided,
// a default instance is created from the current OTel globals.
func WithTelemetry(t *goflux.Telemetry) Option {
	return func(c *config) { c.tel = t }
}

// WithManualAck disables auto-ack on the subscriber. When set, the handler is
// responsible for calling msg.Ack(), msg.Nak(), msg.NakWithDelay(), or
// msg.Term() on every message. Without this option the subscriber auto-acks
// on nil handler error and auto-naks on non-nil error.
func WithManualAck() Option {
	return func(c *config) { c.manualAck = true }
}

func applyOpts(opts []Option) *config {
	cfg := &config{}
	for _, o := range opts {
		o(cfg)
	}

	if cfg.tel == nil {
		cfg.tel, _ = goflux.NewTelemetry()
	}

	return cfg
}
