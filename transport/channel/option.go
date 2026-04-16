package channel

import "github.com/foomo/goflux"

// Option configures a channel Publisher or Subscriber.
type Option func(*config)

type config struct {
	tel *goflux.Telemetry
}

// WithTelemetry sets the Telemetry instance for the transport. If not provided,
// a default instance is created from the current OTel globals.
func WithTelemetry(t *goflux.Telemetry) Option {
	return func(c *config) { c.tel = t }
}

func applyOpts(opts []Option) *config {
	cfg := &config{}
	for _, o := range opts {
		o(cfg)
	}

	cfg.tel = goflux.DefaultTelemetry(cfg.tel)

	return cfg
}
