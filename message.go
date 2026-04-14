package goflux

import "time"

// Message is the unit passed to every Handler. Subject carries the routing
// key (e.g. a NATS subject or HTTP path); Payload holds the decoded value;
// Header carries optional metadata; acker provides acknowledgment controls.
type Message[T any] struct {
	Subject string `json:"subject"`
	Payload T      `json:"payload"`
	Header  Header `json:"header,omitempty"`
	acker   Acker
}

// NewMessage creates a new Message.
func NewMessage[T any](subject string, payload T) Message[T] {
	return Message[T]{
		Subject: subject,
		Payload: payload,
	}
}

// NewMessageWithHeader creates a new Message with the given header.
func NewMessageWithHeader[T any](subject string, payload T, header Header) Message[T] {
	return Message[T]{
		Subject: subject,
		Payload: payload,
		Header:  header,
	}
}

// WithAcker returns a copy of the message with the given acker attached.
// This is intended for transport implementations, not application code.
func (m Message[T]) WithAcker(a Acker) Message[T] {
	m.acker = a

	return m
}

// HasAcker reports whether the message carries acknowledgment controls.
func (m Message[T]) HasAcker() bool {
	return m.acker != nil
}

// Ack acknowledges successful processing. No-op if the transport does not
// support acknowledgments.
func (m Message[T]) Ack() error {
	if m.acker == nil {
		return nil
	}

	return m.acker.Ack()
}

// Nak signals processing failure; the message should be redelivered.
// No-op if the transport does not support acknowledgments.
func (m Message[T]) Nak() error {
	if m.acker == nil {
		return nil
	}

	return m.acker.Nak()
}

// NakWithDelay signals processing failure with a redelivery delay hint.
// Falls back to Nak if the transport does not support delayed redelivery.
func (m Message[T]) NakWithDelay(d time.Duration) error {
	if m.acker == nil {
		return nil
	}

	if da, ok := m.acker.(DelayedNaker); ok {
		return da.NakWithDelay(d)
	}

	return m.acker.Nak()
}

// Term terminates processing — the message will not be redelivered.
// Falls back to Ack if the transport does not support terminal rejection,
// to prevent infinite redelivery loops.
func (m Message[T]) Term() error {
	if m.acker == nil {
		return nil
	}

	if t, ok := m.acker.(Terminator); ok {
		return t.Term()
	}

	return m.acker.Ack()
}
