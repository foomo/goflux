# Middleware

Middleware wraps a `Handler[T]` to add cross-cutting behaviour such as rate-limiting, deduplication, or observability side-effects.

## Middleware[T] Type

```go
type Middleware[T any] func(Handler[T]) Handler[T]
```

A middleware receives the next handler in the chain and returns a new handler that typically calls the original after (or instead of) running its own logic.

## Chain

`Chain` composes multiple middleware left-to-right. The first middleware in the list becomes the outermost wrapper.

```go
func Chain[T any](mws ...Middleware[T]) Middleware[T]
```

`Chain(a, b)(h)` is equivalent to `a(b(h))` -- `a` runs first, then `b`, then `h`.

## Built-in Middleware

### Process

```go
func Process[T any](n int) Middleware[T]
```

Concurrency limiter backed by a channel semaphore. Allows at most `n` handler invocations to run simultaneously. When all slots are occupied, subsequent calls block until a slot frees up or the context is cancelled.

```go
limited := goflux.Process[Event](5)(handler)
```

### Peek

```go
func Peek[T any](fn func(context.Context, Message[T])) Middleware[T]
```

Side-effect tap. Calls `fn` for every message before forwarding to the next handler. Errors from `fn` are intentionally ignored -- use a regular middleware if error handling is needed.

```go
logged := goflux.Peek[Event](func(ctx context.Context, msg goflux.Message[Event]) {
    slog.InfoContext(ctx, "received", slog.String("subject", msg.Subject))
})(handler)
```

### Distinct

```go
func Distinct[T any](key func(Message[T]) string) Middleware[T]
```

Deduplicates messages by a caller-supplied key function. The first message for each key passes through; duplicates are silently dropped. The internal seen set is unbounded -- use a custom middleware with TTL-based eviction if memory is a concern.

```go
deduped := goflux.Distinct[Event](func(m goflux.Message[Event]) string {
    return m.Payload.ID
})(handler)
```

### Skip

```go
func Skip[T any](n int) Middleware[T]
```

Drops the first `n` messages, then passes all subsequent messages to the next handler. The counter is atomic and safe for concurrent use.

```go
skipFirst := goflux.Skip[Event](10)(handler)
```

### Take

```go
func Take[T any](n int) Middleware[T]
```

Passes the first `n` messages through and silently drops all subsequent messages. The counter is atomic and safe for concurrent use.

```go
firstTen := goflux.Take[Event](10)(handler)
```

### Throttle

```go
func Throttle[T any](d time.Duration) Middleware[T]
```

Rate-limits handler invocations to at most one per duration `d`. The first message passes immediately; subsequent messages block until the internal ticker fires. Context cancellation is respected while waiting.

```go
throttled := goflux.Throttle[Event](100 * time.Millisecond)(handler)
```

### AutoAck

```go
func AutoAck[T any]() Middleware[T]
```

Acknowledges messages automatically based on the handler result: `nil` error triggers `Ack()`, non-nil error triggers `Nak()`. Messages without an acker (fire-and-forget transports like channels and core NATS) are passed through as-is.

```go
autoAcked := goflux.AutoAck[Event]()(handler)
```

## Composing with Chain

Use `Chain` to stack multiple middleware into a single wrapper:

```go
handler := goflux.Chain[Event](
    goflux.Process[Event](10),
    goflux.Throttle[Event](100*time.Millisecond),
    goflux.Distinct[Event](func(m goflux.Message[Event]) string {
        return m.Payload.ID
    }),
)(func(ctx context.Context, msg goflux.Message[Event]) error {
    return processEvent(ctx, msg.Payload)
})
```

Execution order: `Process` runs first (acquires semaphore), then `Throttle` (rate-limits), then `Distinct` (deduplicates), then the inner handler. Errors propagate back through each layer in reverse order.

## Writing Custom Middleware

A custom middleware is any function that matches `func(Handler[T]) Handler[T]`. Wrap the next handler and add your logic before or after calling it:

```go
func LogDuration[T any]() goflux.Middleware[T] {
    return func(next goflux.Handler[T]) goflux.Handler[T] {
        return func(ctx context.Context, msg goflux.Message[T]) error {
            start := time.Now()
            err := next(ctx, msg)
            slog.InfoContext(ctx, "handler completed",
                slog.String("subject", msg.Subject),
                slog.Duration("duration", time.Since(start)),
                slog.Any("error", err),
            )
            return err
        }
    }
}
```

Use it like any built-in middleware:

```go
handler := goflux.Chain[Event](
    LogDuration[Event](),
    goflux.Process[Event](10),
)(myHandler)
```

::: tip
Keep middleware focused on a single concern. Compose multiple small middleware with `Chain` rather than building one large wrapper.
:::
