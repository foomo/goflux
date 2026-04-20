# Pipe Package Redesign

**Date:** 2026-04-20
**Status:** Approved

## Summary

Redesign the pipe package from first-draft to production-ready. Pipe is a linear handler factory: `subscribe → middleware → filter → [map/flatmap] → publish`. Errors always return to transport. Observability via span events. Bridge adapters extracted to own module.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Pipeline model | Linear (sub → transform → filter → publish) | Matches use case, simple |
| Error semantics | Errors return to transport handler caller | Transport owns retry/nak/DLQ. Pipe doesn't swallow. |
| Filter rejection | Returns nil (ack, intentional skip) | Filtered messages are successfully ignored |
| Observability | Span events + attributes on existing span | Lightweight, no child span overhead |
| API pattern | Option-driven handler factory | Idiomatic Go, extensible |
| Context forwarding | Header + trace auto, message ID via middleware | Infrastructure auto, business opt-in |
| Bound variants | Removed entirely | Users compose with `Bind()` |
| Stream bridges | Own `bridge/` module with `go.mod` | Isolates goflow dependency |
| `ToChan` | Root package | stdlib only, no external deps |
| Middleware in pipe | `WithMiddleware` option | Composable, extensible |

## Pipe Package API

### Functions

```go
// pipe/pipe.go

// New creates a handler that forwards messages to a publisher.
func New[T any](pub goflux.Publisher[T], opts ...Option[T]) goflux.Handler[T]

// NewMap creates a handler that transforms messages before publishing.
func NewMap[T, U any](pub goflux.Publisher[U], mapFn MapFunc[T, U], opts ...MapOption[T, U]) goflux.Handler[T]

// NewFlatMap creates a handler that expands messages into multiple items before publishing.
func NewFlatMap[T, U any](pub goflux.Publisher[U], fn FlatMapFunc[T, U], opts ...MapOption[T, U]) goflux.Handler[T]
```

### Types

```go
// pipe/option.go

type Filter[T any] func(ctx context.Context, msg goflux.Message[T]) bool
type MapFunc[T, U any] func(ctx context.Context, msg goflux.Message[T]) (U, error)
type FlatMapFunc[T, U any] func(ctx context.Context, msg goflux.Message[T]) ([]U, error)
type DeadLetterFunc[T any] func(ctx context.Context, msg goflux.Message[T], err error)

type Option[T any] func(*config[T])
type MapOption[T, U any] func(*mapConfig[T, U])

func WithFilter[T any](fn Filter[T]) Option[T]
func WithDeadLetter[T any](fn DeadLetterFunc[T]) Option[T]
func WithMiddleware[T any](mw ...goflux.Middleware[T]) Option[T]

func WithMapFilter[T, U any](fn Filter[T]) MapOption[T, U]
func WithMapDeadLetter[T, U any](fn DeadLetterFunc[T]) MapOption[T, U]
func WithMapMiddleware[T, U any](mw ...goflux.Middleware[T]) MapOption[T, U]
```

### Execution Flow

```
incoming message
  → middleware chain (enrich context, log, validate, etc.)
    → filter (true = continue, false = return nil / ack)
      → [map/flatmap if applicable] (error → dead-letter observer + return error)
        → publish with forwarded header + trace context
          → publish error → dead-letter observer + return error
```

### Error Semantics

| Stage | On failure | Return value |
|-------|-----------|--------------|
| Filter | Rejects message | `nil` — intentional skip, transport acks |
| MapFunc | Transform fails | `error` — transport decides retry/nak. Dead-letter observer called. |
| FlatMapFunc | Transform fails | `error` — transport decides retry/nak. Dead-letter observer called. |
| Publish | Publish fails | `error` — transport decides retry/nak. Dead-letter observer called. |
| FlatMap publish (item N of M) | Fails mid-batch | `error` — items 1..N-1 already published. Transport retries whole handler. Downstream must be idempotent. |

Dead-letter func is an **observer** — receives `(ctx, msg, err)` for logging/alerting. Never swallows errors.

## Observability

Pipe adds to the existing transport span. No child spans created.

### Span Attributes (every message)

| Attribute | Value | When |
|-----------|-------|------|
| `pipe.type` | `"forward"` / `"map"` / `"flatmap"` | Always |
| `pipe.filtered` | `true` | Filter rejected message |
| `pipe.items_published` | `int` | FlatMap only |

### Span Events (notable outcomes)

| Event | Attributes | When |
|-------|-----------|------|
| `pipe.dead_letter` | `pipe.error` | Dead-letter observer called |
| `pipe.publish_error` | `pipe.error` | Publish failed |
| `pipe.map_error` | `pipe.error` | Map/FlatMap func failed |

## Context & Header Forwarding

| What | Behavior | Rationale |
|------|----------|-----------|
| Header | Auto-forwarded: `msg.Header` → `WithHeader(ctx)` → publish | Infrastructure plumbing |
| Trace context | Flows naturally — no new spans, publish inherits span context | Transparent |
| Message ID | Opt-in via `middleware.ForwardMessageID[T]()` | Business-level concern |
| Acker | Never forwarded | Incoming message ack stays with incoming message |
| Subject | Not inherited | Publish target set by caller via publisher or `Bind()` |

## Bridge Package

New `bridge/` subpackage with own `go.mod`. Isolates goflow dependency.

### API

```go
// bridge/tostream.go
func ToStream[T any](ctx context.Context, sub goflux.Subscriber[T], subject string) goflow.Stream[goflux.Message[T]]

// bridge/fromstream.go
func FromStream[T any](ctx context.Context, stream goflow.Stream[goflux.Message[T]], pub goflux.Publisher[T], subject string) error
```

### Error Handling

- `ToStream` — stream ends on ctx cancel.
- `FromStream` — returns first publish error. Caller decides retry.

## Root Package: ToChan

Moved from pipe to root. stdlib only.

```go
// tochan.go
func ToChan[T any](ctx context.Context, sub goflux.Subscriber[T], subject string, bufSize int) <-chan goflux.Message[T]
```

Closes channel on ctx cancel. Subscribe errors logged.

## New Middleware

### `middleware.ForwardMessageID[T]()`

```go
func ForwardMessageID[T any]() goflux.Middleware[T] {
    return func(next goflux.Handler[T]) goflux.Handler[T] {
        return func(ctx context.Context, msg goflux.Message[T]) error {
            if id := goflux.MessageID(ctx); id != "" {
                ctx = goflux.WithMessageID(ctx, id)
            }
            return next(ctx, msg)
        }
    }
}
```

## Deletions

| Removed | Reason |
|---------|--------|
| All `Bound*` functions (root + pipe) | Use `Bind()` for fixed-subject |
| `pipe.ToStream` / `pipe.FromStream` | → `bridge/` |
| `pipe.ToChan` | → root package |
| Root `Pipe` / `PipeMap` wrappers | Replaced by `pipe.New` / `pipe.NewMap` |
| Root `ToStream` / `FromStream` | → `bridge/` |
| Root `ToChan` (current location) | Stays in root, just cleaned up |

## Testing Strategy

- Unit tests per pipe function with channel transport (fast, in-process)
- Test filter rejection returns nil
- Test map/flatmap error propagation
- Test FlatMap partial publish failure
- Test middleware ordering (middleware sees original message, enriched context flows to pipe)
- Test dead-letter observer receives correct error + message
- Test span events/attributes via OTel test exporter
- Bridge tests with goflow stream assertions
