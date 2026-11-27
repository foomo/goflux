package goflux

import (
	"encoding/json"
	"time"
)

// Message is the unit passed to every Handler. Subject carries the routing
// key (e.g. a NATS subject or HTTP path); Payload holds the decoded value;
// Header carries optional metadata; ID carries the message ID the publisher
// set with [WithMessageID]; acker provides acknowledgment controls.
//
// Like Header, ID describes the received message only. Messages published
// from a handler do not inherit it; use middleware.ForwardMessageID to
// forward it explicitly.
type Message[T any] struct {
	Subject string `json:"subject"`
	Payload T      `json:"payload"`
	Header  Header `json:"header,omitempty"`
	ID      string `json:"id,omitempty"`
	acker   Acker
}

// messageJSON is the JSON form of [Message], plus the "nats" key that
// versions before v0.12 used for Subject.
type messageJSON[T any] struct {
	Subject       string `json:"subject"`
	LegacySubject string `json:"nats"`
	Payload       T      `json:"payload"`
	Header        Header `json:"header,omitempty"`
	ID            string `json:"id,omitempty"`
}

// UnmarshalJSON decodes a Message. It also accepts the "nats" key that
// versions before v0.12 wrote for Subject; "subject" wins if both are set.
func (m *Message[T]) UnmarshalJSON(b []byte) error {
	v := messageJSON[T]{Payload: m.Payload, Header: m.Header}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}

	m.Subject = v.Subject
	if m.Subject == "" {
		m.Subject = v.LegacySubject
	}

	m.Payload = v.Payload
	m.Header = v.Header
	m.ID = v.ID

	return nil
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

// InProgress signals that processing is still ongoing, resetting the
// transport's redelivery timer. No-op if the transport does not support it.
func (m Message[T]) InProgress() error {
	if ip, ok := m.acker.(InProgresser); ok {
		return ip.InProgress()
	}

	return nil
}

// Delivery returns the transport's delivery metadata. Returns the zero value
// if the transport does not report it.
func (m Message[T]) Delivery() Delivery {
	if dr, ok := m.acker.(DeliveryReporter); ok {
		return dr.Delivery()
	}

	return Delivery{}
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
