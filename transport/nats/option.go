package nats

import "github.com/foomo/goflux"

// Option configures a NATS Publisher or Subscriber.
type Option func(*config)

type config struct {
	tel        *goflux.Telemetry
	queueGroup string
}

// WithTelemetry sets the Telemetry instance for the transport. If not provided,
// a default instance is created from the current OTel globals.
func WithTelemetry(t *goflux.Telemetry) Option {
	return func(c *config) { c.tel = t }
}

// WithQueueGroup sets the queue group for subscriber. When set, the subscriber
// joins the named queue group, turning the subscription into a competing
// consumer — each message is delivered to only one member of the group.
func WithQueueGroup(name string) Option {
	return func(c *config) { c.queueGroup = name }
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
