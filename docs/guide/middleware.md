# Middleware

Middleware wraps a `Handler[T]` to add cross-cutting behaviour. goflux provides several built-in middleware functions and a `Chain` combinator for composing them.

## Middleware[T] Type

```go
type Middleware[T any] func(Handler[T]) Handler[T]
```

A middleware takes a handler and returns a new handler that wraps the original with additional logic.

## Chain[T]

`Chain` composes multiple middleware left-to-right. The first middleware in the list is the **outermost** wrapper.

```go
func Chain[T any](mws ...Middleware[T]) Middleware[T]
```

`Chain(a, b)(h)` is equivalent to `a(b(h))`. When a message arrives, it passes through `a` first, then `b`, then `h`.

```go
wrapped := goflux.Chain[OrderEvent](
    goflux.Process[OrderEvent](5),
    goflux.Throttle[OrderEvent](100 * time.Millisecond),
    goflux.Distinct[OrderEvent](func(msg goflux.Message[OrderEvent]) string {
        return msg.Payload.ID
    }),
)(handler)

go func() {
    _ = sub.Subscribe(ctx, "orders", wrapped)
}()
```

## Built-in Middleware

### Process[T]

Limits concurrent handler invocations to `n` using a semaphore. When all slots are occupied, the handler blocks until a slot frees up or the context is cancelled.

```go
func Process[T any](n int) Middleware[T]
```

```go
// Allow at most 5 concurrent handlers.
handler := goflux.Process[OrderEvent](5)(func(ctx context.Context, msg goflux.Message[OrderEvent]) error {
    // expensive work here
    return nil
})
```

### Peek[T]

Calls a function as a side-effect for each message before forwarding to the next handler. Errors from the peek function are intentionally ignored.

```go
func Peek[T any](fn func(context.Context, Message[T])) Middleware[T]
```

```go
// Log every message without affecting the pipeline.
handler := goflux.Peek[OrderEvent](func(ctx context.Context, msg goflux.Message[OrderEvent]) {
    log.Printf("peek: subject=%s id=%s", msg.Subject, msg.Payload.ID)
})(handler)
```

::: tip
Use `Peek` for logging, metrics, or debugging. If you need error handling on the side-effect, write a custom middleware instead.
:::

### Distinct[T]

Deduplicates messages using a key function. The first message for each key passes through; subsequent messages with the same key are silently dropped.

```go
func Distinct[T any](key func(Message[T]) string) Middleware[T]
```

```go
// Deduplicate by order ID.
handler := goflux.Distinct[OrderEvent](func(msg goflux.Message[OrderEvent]) string {
    return msg.Payload.ID
})(handler)
```

::: warning
The seen set is **unbounded** and grows indefinitely. For long-running subscribers processing many unique keys, implement a custom middleware with an LRU cache or TTL-based eviction.
:::

### Skip[T]

Drops the first `n` messages and passes the rest through. The counter is safe for concurrent use (atomic).

```go
func Skip[T any](n int) Middleware[T]
```

```go
// Skip the first 10 messages.
handler := goflux.Skip[OrderEvent](10)(handler)
```

### Take[T]

Passes the first `n` messages through and silently drops all subsequent messages. The counter is safe for concurrent use (atomic).

```go
func Take[T any](n int) Middleware[T]
```

```go
// Process only the first 100 messages.
handler := goflux.Take[OrderEvent](100)(handler)
```

### Throttle[T]

Rate-limits handler invocations to at most one per duration `d`. The first message passes immediately; subsequent messages block until the ticker fires. Context cancellation is respected while waiting.

```go
func Throttle[T any](d time.Duration) Middleware[T]
```

```go
// At most one message per 200ms.
handler := goflux.Throttle[OrderEvent](200 * time.Millisecond)(handler)
```

## Composition Example

Combine multiple middleware to build a processing pipeline:

```go
// Limit to 5 concurrent handlers, throttle to 10 msg/s,
// deduplicate by ID, and skip the first warm-up message.
wrapped := goflux.Chain[OrderEvent](
    goflux.Process[OrderEvent](5),
    goflux.Throttle[OrderEvent](100 * time.Millisecond),
    goflux.Distinct[OrderEvent](func(msg goflux.Message[OrderEvent]) string {
        return msg.Payload.ID
    }),
    goflux.Skip[OrderEvent](1),
)(handler)

go func() {
    _ = sub.Subscribe(ctx, "orders", wrapped)
}()
```

Execution order for each message: `Process` (acquire slot) -> `Throttle` (wait for tick) -> `Distinct` (check seen set) -> `Skip` (check counter) -> `handler`.

## Writing Custom Middleware

A custom middleware follows the same pattern -- take a handler, return a handler:

```go
func LogErrors[T any](logger *log.Logger) goflux.Middleware[T] {
    return func(next goflux.Handler[T]) goflux.Handler[T] {
        return func(ctx context.Context, msg goflux.Message[T]) error {
            err := next(ctx, msg)
            if err != nil {
                logger.Printf("handler error: subject=%s err=%v", msg.Subject, err)
            }
            return err
        }
    }
}

// Use it:
handler := LogErrors[OrderEvent](logger)(handler)
```

The key points:

1. Accept `Handler[T]` as the `next` parameter.
2. Return a new `Handler[T]` that calls `next` at the appropriate point.
3. Add your logic before, after, or around the `next` call.

## What's Next

- [Distribution](./distribution.md) -- fan-out, fan-in, and round-robin patterns
- [Pipelines](./pipelines.md) -- wire subscribers to publishers with filtering
- [Telemetry](./telemetry.md) -- understand the metrics and traces
