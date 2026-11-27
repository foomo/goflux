package jetstream

import (
	"time"

	"github.com/foomo/goflux"
	"github.com/nats-io/nats.go/jetstream"
)

// DefaultPullExpiry is the pull request expiry the Subscriber passes to
// jetstream.Consumer.Consume unless overridden with [WithConsumeOpts]. The
// idle heartbeat defaults to half of it, so a consumer that disappears and is
// recreated (for example by NACK after a server restart) is picked up again
// within about this duration. nats.go's own default is 30 seconds.
const DefaultPullExpiry = 10 * time.Second

// Option configures a JetStream Publisher, Subscriber, or Consumer.
type Option func(*config)

type config struct {
	tel         *goflux.Telemetry
	manualAck   bool
	consumeOpts []jetstream.PullConsumeOpt
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

// WithConsumeOpts passes options to jetstream.Consumer.Consume on the
// subscriber, for example jetstream.PullExpiry, jetstream.PullHeartbeat,
// jetstream.PullMaxMessages or jetstream.ConsumeErrHandler. They are applied
// after the goflux defaults ([DefaultPullExpiry] and an error handler that logs
// via slog), so they override them. Replacing the error handler also removes
// the cause from the error that Subscribe returns when consumption stops
// (the cause is best effort either way).
func WithConsumeOpts(opts ...jetstream.PullConsumeOpt) Option {
	return func(c *config) { c.consumeOpts = append(c.consumeOpts, opts...) }
}

func applyOpts(opts []Option) *config {
	cfg := &config{}
	for _, o := range opts {
		o(cfg)
	}

	cfg.tel = goflux.DefaultTelemetry(cfg.tel)

	return cfg
}
