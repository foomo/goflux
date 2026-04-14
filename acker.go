package goflux

import "time"

// Acker is the minimal acknowledgment interface. Transports that support
// at-least-once delivery implement this on their message wrapper.
type Acker interface {
	Ack() error
	Nak() error
}

// DelayedNaker extends Acker with delayed negative acknowledgment,
// causing the message to be redelivered after the given delay.
type DelayedNaker interface {
	Acker
	NakWithDelay(d time.Duration) error
}

// Terminator extends Acker with terminal rejection — the message
// will not be redelivered. Use for dead-letter patterns where
// handler errors are non-retryable.
type Terminator interface {
	Acker
	Term() error
}
