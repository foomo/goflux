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

// WithMessageID returns a copy of ctx with the given message ID attached.
// The ID is purely opt-in: if set, transports propagate it via headers and
// RecordPublish / RecordProcess attach it as the goflux.message.id span
// attribute.
func WithMessageID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyMessageID, id)
}

// MessageID returns the message ID stored in ctx, or "" if none is set.
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
