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

::: tip Stream Processing
For stream-processing operators like concurrency limiting, deduplication, throttling, skip/take, and peek, use [goflow](https://github.com/foomo/goflow) via `ToStream`. See [Pipeline Operators](/pipeline/) for the bridge functions.
:::

### AutoAck

```go
func AutoAck[T any]() Middleware[T]
```

Acknowledges messages automatically based on the handler result: `nil` error triggers `Ack()`, non-nil error triggers `Nak()`. Messages without an acker (fire-and-forget transports like channels and core NATS) are passed through as-is.

```go
autoAcked := middleware.AutoAck[Event]()(handler)
```

### InjectMessageID

```go
func InjectMessageID[T any]() Middleware[T]
```

Reads the message ID from the message's header (`X-Message-ID`) and injects it into the context via `WithMessageID`. Push-based transports do this automatically — this middleware is for handler chains that bypass built-in injection.

```go
withID := middleware.InjectMessageID[Event]()(handler)
// handler can now use goflux.MessageID(ctx)
```

### InjectHeader

```go
func InjectHeader[T any]() Middleware[T]
```

Injects the message's full `Header` into the context via `WithHeader`. Downstream code can read it with `HeaderFromContext`. Push-based transports do this automatically.

```go
withHeader := middleware.InjectHeader[Event]()(handler)
// handler can now use goflux.HeaderFromContext(ctx)
```

### ForwardMessageID

```go
func ForwardMessageID[T any]() Middleware[T]
```

Preserves the message ID from the incoming context through the handler chain. Use this with pipe's `WithMiddleware` to forward message IDs across pipe stages.

Unlike `InjectMessageID` (which reads the ID from message headers set by transports), `ForwardMessageID` reads from context -- suitable for in-process handler chains where the ID is already in context.

```go
handler := pipe.New[Event](pub,
    pipe.WithMiddleware(
        middleware.ForwardMessageID[Event](),
    ),
)
```

## Composing with Chain

Use `Chain` to stack multiple middleware into a single wrapper:

```go
handler := goflux.Chain[Event](
    middleware.AutoAck[Event](),
    middleware.InjectMessageID[Event](),
)(func(ctx context.Context, msg goflux.Message[Event]) error {
    return processEvent(ctx, msg.Payload)
})
```

Execution order: `AutoAck` runs first (outermost), then `InjectMessageID`, then the inner handler. Errors propagate back through each layer in reverse order.

## Writing Custom Middleware

A custom middleware is any function that matches `func(Handler[T]) Handler[T]`. Wrap the next handler and add your logic before or after calling it:

```go
func LogDuration[T any]() goflux.Middleware[T] {
    return func(next goflux.Handler[T]) goflux.Handler[T] {
        return func(ctx context.Context, msg goflux.Message[T]) error {
            start := time.Now()
            err := next(ctx, msg)
            slog.InfoContext(ctx, "handler completed",
                slog.String("nats", msg.Subject),
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
    middleware.AutoAck[Event](),
)(myHandler)
```

::: tip
Keep middleware focused on a single concern. Compose multiple small middleware with `Chain` rather than building one large wrapper.
:::
