package middleware

import (
	"context"
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

// RetryPolicy inspects a handler error and decides how to acknowledge the
// failed message.
type RetryPolicy func(err error) RetryDecision

// NewRetryPolicy returns a [RetryPolicy] that terminates non-retryable errors
// (see [goflux.ErrNonRetryable]) and naks all other errors with the given delay.
func NewRetryPolicy(delay time.Duration) RetryPolicy {
	return func(err error) RetryDecision {
		if goflux.IsNonRetryable(err) {
			return RetryDecision{Action: RetryTerm}
		}

		return RetryDecision{Action: RetryNakWithDelay, Delay: delay}
	}
}

// RetryAck returns a Middleware that acknowledges messages based on handler
// outcome and a [RetryPolicy]:
//   - nil error: [goflux.Message.Ack]
//   - non-nil error + [RetryTerm]: [goflux.Message.Term]
//   - non-nil error + [RetryNakWithDelay]: [goflux.Message.NakWithDelay]
//   - non-nil error + [RetryNak]: [goflux.Message.Nak]
//
// Messages without an acker (fire-and-forget transports) are passed through.
func RetryAck[T any](policy RetryPolicy) goflux.Middleware[T] {
	return func(next goflux.Handler[T]) goflux.Handler[T] {
		return func(ctx context.Context, msg goflux.Message[T]) error {
			err := next(ctx, msg)
			if !msg.HasAcker() {
				return err
			}

			if err == nil {
				return msg.Ack()
			}

			decision := policy(err)

			switch decision.Action {
			case RetryTerm:
				_ = msg.Term()
			case RetryNakWithDelay:
				_ = msg.NakWithDelay(decision.Delay)
			default:
				_ = msg.Nak()
			}

			return err
		}
	}
}
