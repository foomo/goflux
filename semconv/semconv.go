package semconv

import "go.opentelemetry.io/otel/attribute"

// Attribute keys for goflux telemetry.
const (
	// AckActionKey is the attribute key for the acknowledgment action (ack, nak, nak_with_delay, term).
	AckActionKey = attribute.Key("goflux.ack.action")
	// AckErrorKey is the attribute key indicating whether the ack operation itself failed.
	AckErrorKey = attribute.Key("goflux.ack.error")
	// ReplyBodySizeKey is the attribute key for the reply/response body size, in
	// bytes. The standard messaging.message.body.size key describes the inbound
	// request, so the reply leg needs its own key to avoid overwriting it.
	ReplyBodySizeKey = attribute.Key("goflux.reply.body.size")
	// ReplySubjectKey is the attribute key for the subject the reply was received on.
	ReplySubjectKey = attribute.Key("goflux.reply.subject")
)

// AckAction returns an attribute with the acknowledgment action.
func AckAction(v string) attribute.KeyValue {
	return AckActionKey.String(v)
}

// AckError returns an attribute indicating whether the ack operation itself failed.
func AckError(v bool) attribute.KeyValue {
	return AckErrorKey.Bool(v)
}

// ReplyBodySize returns an attribute with the reply/response body size in bytes.
func ReplyBodySize(v int) attribute.KeyValue {
	return ReplyBodySizeKey.Int(v)
}

// ReplySubject returns an attribute with the subject the reply was received on.
func ReplySubject(v string) attribute.KeyValue {
	return ReplySubjectKey.String(v)
}
