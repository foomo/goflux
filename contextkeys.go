package goflux

import "context"

type ctxKey int

const (
	ctxKeyMessageID ctxKey = iota
	ctxKeyHeader
)

// MessageIDHeader is the HTTP header name used to propagate a message ID
// across the HTTP transport.
const MessageIDHeader = "X-Message-ID"

// WithMessageID returns a copy of ctx with the message ID for outgoing
// publishes. The ID is purely opt-in: if set, transports send it with every
// message published with ctx, the subscriber exposes it as [Message.ID], and
// RecordPublish / RecordProcess attach it as the messaging.message.id span
// attribute. Subscribers never put a received ID into the handler context, so
// a handler's publishes carry no ID unless the handler sets one (for example
// via middleware.ForwardMessageID).
func WithMessageID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyMessageID, id)
}

// MessageID returns the outgoing message ID stored in ctx by [WithMessageID],
// or "" if none is set. Read the ID of a received message from [Message.ID].
func MessageID(ctx context.Context) string {
	v, _ := ctx.Value(ctxKeyMessageID).(string)
	return v
}

// WithHeader returns a copy of ctx with the given header attached.
// Transports read this header during Publish and merge it into the
// outgoing transport headers.
func WithHeader(ctx context.Context, h Header) context.Context {
	return context.WithValue(ctx, ctxKeyHeader, h)
}

// HeaderFromContext returns the header stored in ctx, or nil if none is set.
func HeaderFromContext(ctx context.Context) Header {
	v, _ := ctx.Value(ctxKeyHeader).(Header)
	return v
}
