package semconv

import "go.opentelemetry.io/otel/attribute"

// Attribute keys for goflux telemetry.
const (
	// MessageIDKey is the attribute key for the business-level message ID.
	MessageIDKey = attribute.Key("goflux.message.id")
	// DestinationNameKey is the attribute key for the destination (subject) name.
	DestinationNameKey = attribute.Key("goflux.destination.name")
	// AckActionKey is the attribute key for the acknowledgment action (ack, nak, nak_with_delay, term).
	AckActionKey = attribute.Key("goflux.ack.action")
	// AckErrorKey is the attribute key indicating whether the ack operation itself failed.
	AckErrorKey = attribute.Key("goflux.ack.error")
)

// MessageID returns an attribute with the business-level message ID.
func MessageID(v string) attribute.KeyValue {
	return MessageIDKey.String(v)
}

// DestinationName returns an attribute with the destination (subject) name.
func DestinationName(v string) attribute.KeyValue {
	return DestinationNameKey.String(v)
}

// AckAction returns an attribute with the acknowledgment action.
func AckAction(v string) attribute.KeyValue {
	return AckActionKey.String(v)
}

// AckError returns an attribute indicating whether the ack operation itself failed.
func AckError(v bool) attribute.KeyValue {
	return AckErrorKey.Bool(v)
}
