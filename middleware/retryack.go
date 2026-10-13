package middleware

import (
	"context"
	"math"
	"time"

	"github.com/foomo/goflux"
)

// RetryAction describes how a failed message should be acknowledged.
type RetryAction int

const (
	// RetryNak triggers immediate redelivery via [goflux.Message.Nak].
	RetryNak RetryAction = iota
	// RetryNakWithDelay triggers delayed redelivery via [goflux.Message.NakWithDelay].
	// The delay is specified in [RetryDecision.Delay].
	RetryNakWithDelay
	// RetryTerm terminates the message via [goflux.Message.Term] — no further
	// redelivery will occur.
	RetryTerm
)

// RetryDecision holds the action and optional delay for a failed message.
type RetryDecision struct {
	Action RetryAction
	Delay  time.Duration
}

// RetryPolicy inspects a handler error and the message's delivery metadata
// and decides how to acknowledge the failed message. delivery is the zero
// value when the transport does not report delivery metadata.
type RetryPolicy func(err error, delivery goflux.Delivery) RetryDecision

// NewRetryPolicy returns a [RetryPolicy] that terminates non-retryable errors
// (see [goflux.ErrNonRetryable]) and naks all other errors with the given delay.
func NewRetryPolicy(delay time.Duration) RetryPolicy {
	return func(err error, _ goflux.Delivery) RetryDecision {
		if goflux.IsNonRetryable(err) {
			return RetryDecision{Action: RetryTerm}
		}

		return RetryDecision{Action: RetryNakWithDelay, Delay: delay}
	}
}

// BackoffRetryPolicyOption configures [NewBackoffRetryPolicy].
type BackoffRetryPolicyOption func(*backoffRetryPolicyConfig)

type backoffRetryPolicyConfig struct {
	factor        float64
	maxDeliveries uint64
}

// WithBackoffFactor sets the multiplier applied to the delay on each
// redelivery. Values below 1 are ignored. Default: 2.
func WithBackoffFactor(factor float64) BackoffRetryPolicyOption {
	return func(c *backoffRetryPolicyConfig) {
		if factor >= 1 {
			c.factor = factor
		}
	}
}

// BackoffRetryPolicyWithMaxDeliveries terminates the message once it has been delivered n
// times. Zero (the default) never terminates retryable errors. Has no effect
// when the transport does not report delivery counts.
func BackoffRetryPolicyWithMaxDeliveries(n uint64) BackoffRetryPolicyOption {
	return func(c *backoffRetryPolicyConfig) { c.maxDeliveries = n }
}

// NewBackoffRetryPolicy returns a [RetryPolicy] whose nak delay grows with the
// message's delivery count: initial * factor^(NumDelivered-1), capped at
// maxDelay. Non-retryable errors (see [goflux.ErrNonRetryable]) are
// terminated. When the transport does not report a delivery count, the first
// delivery is assumed and initial is used.
func NewBackoffRetryPolicy(initial, maxDelay time.Duration, opts ...BackoffRetryPolicyOption) RetryPolicy {
	cfg := &backoffRetryPolicyConfig{factor: 2}
	for _, o := range opts {
		o(cfg)
	}

	return func(err error, delivery goflux.Delivery) RetryDecision {
		if goflux.IsNonRetryable(err) {
			return RetryDecision{Action: RetryTerm}
		}

		if cfg.maxDeliveries > 0 && delivery.NumDelivered >= cfg.maxDeliveries {
			return RetryDecision{Action: RetryTerm}
		}

		attempt := max(delivery.NumDelivered, 1) - 1

		delay := maxDelay
		if d := float64(initial) * math.Pow(cfg.factor, float64(attempt)); d < float64(maxDelay) {
			delay = time.Duration(d)
		}

		return RetryDecision{Action: RetryNakWithDelay, Delay: delay}
	}
}

type (
	retryAckConfig struct {
		tel *goflux.Telemetry
	}
	// RetryAckOption configures [RetryAck].
	RetryAckOption func(*retryAckConfig)
)

// RetryAckWithTelemetry sets the Telemetry instance used to record ack outcomes.
// If not provided, a default instance is created from the current OTel globals.
func RetryAckWithTelemetry(t *goflux.Telemetry) RetryAckOption {
	return func(c *retryAckConfig) { c.tel = t }
}

// RetryAck returns a Middleware that acknowledges messages based on handler
// outcome and a [RetryPolicy]:
//   - nil error: [goflux.Message.Ack]
//   - non-nil error + [RetryTerm]: [goflux.Message.Term]
//   - non-nil error + [RetryNakWithDelay]: [goflux.Message.NakWithDelay]
//   - non-nil error + [RetryNak]: [goflux.Message.Nak]
//
// The policy receives the message's [goflux.Message.Delivery] metadata. Each
// outcome is recorded via [goflux.Telemetry.RecordAckOutcome]. Messages
// without an acker (fire-and-forget transports) are passed through.
func RetryAck[T any](policy RetryPolicy, opts ...RetryAckOption) goflux.Middleware[T] {
	cfg := &retryAckConfig{}
	for _, o := range opts {
		o(cfg)
	}

	tel := goflux.DefaultTelemetry(cfg.tel)

	return func(next goflux.Handler[T]) goflux.Handler[T] {
		return func(ctx context.Context, msg goflux.Message[T]) error {
			err := next(ctx, msg)
			if !msg.HasAcker() {
				return err
			}

			if err == nil {
				ackErr := msg.Ack()
				tel.RecordAckOutcome(ctx, "ack", msg.Subject, ackErr)

				return ackErr
			}

			decision := policy(err, msg.Delivery())

			var (
				action string
				ackErr error
			)

			switch decision.Action {
			case RetryTerm:
				action = "term"
				ackErr = msg.Term()
			case RetryNakWithDelay:
				action = "nak_with_delay"
				ackErr = msg.NakWithDelay(decision.Delay)
			default:
				action = "nak"
				ackErr = msg.Nak()
			}

			tel.RecordAckOutcome(ctx, action, msg.Subject, ackErr)

			return err
		}
	}
}
