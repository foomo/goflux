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

// InProgresser extends Acker with a work-in-progress signal that resets the
// transport's redelivery timer (e.g. JetStream AckWait). Long-running
// handlers call it periodically to keep a message from being redelivered
// while it is still being processed.
type InProgresser interface {
	Acker
	InProgress() error
}

// Delivery carries transport-level delivery metadata for a message.
type Delivery struct {
	// NumDelivered is the number of times the message has been delivered,
	// starting at 1 for the first delivery. Zero means unknown.
	NumDelivered uint64
	// Timestamp is when the message was originally stored or sent. Zero
	// means unknown.
	Timestamp time.Time
}

// DeliveryReporter extends Acker with delivery metadata, e.g. the redelivery
// count a retry policy uses to grow its backoff delay.
type DeliveryReporter interface {
	Acker
	Delivery() Delivery
}
