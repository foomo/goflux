package http

import "github.com/foomo/goflux"

// PublisherOption configures an HTTP Publisher.
type PublisherOption func(*publisherConfig)

type publisherConfig struct {
	tel *goflux.Telemetry
}

// WithPublisherTelemetry sets the Telemetry instance for the publisher.
// If not provided, a default instance is created from the current OTel globals.
func WithPublisherTelemetry(t *goflux.Telemetry) PublisherOption {
	return func(c *publisherConfig) { c.tel = t }
}

func applyPublisherOpts(opts []PublisherOption) *publisherConfig {
	cfg := &publisherConfig{}
	for _, o := range opts {
		o(cfg)
	}

	if cfg.tel == nil {
		cfg.tel, _ = goflux.NewTelemetry()
	}

	return cfg
}
