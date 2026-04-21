# Stream Processing

Stream processing combines goflux transports with [goflow](https://github.com/foomo/goflow) operators for concurrency, filtering, deduplication, and transformation. Use `ToStream` to bridge a subscriber into a goflow stream, apply operators, then consume or forward with `FromStream`.

## Overview

```go
stream := bridge.ToStream[Event](ctx, sub, "events.>", 16)

// Apply goflow operators for stream processing.
stream.
    Distinct(func(msg goflux.Message[Event]) string { return msg.Payload.ID }).
    Peek(func(_ context.Context, msg goflux.Message[Event]) {
        slog.Info("processing", "id", msg.Payload.ID)
    }).
    Process(10, func(ctx context.Context, msg goflux.Message[Event]) error {
        return handleEvent(ctx, msg.Payload)
    })
```

## Retry-Aware Acknowledgment

For JetStream subscribers with explicit ack, combine the goflow stream with goflux's messaging-specific middleware:

```go
stream := bridge.ToStream[Task](ctx, sub, "tasks.>", 16)

policy := middleware.NewRetryPolicy(5 * time.Second)

stream.Process(5, func(ctx context.Context, msg goflux.Message[Task]) error {
    err := processTask(ctx, msg.Payload)
    if err != nil {
        decision := policy(err)
        switch decision.Action {
        case middleware.RetryNak:
            return msg.Nak()
        case middleware.RetryNakWithDelay:
            return msg.NakWithDelay(decision.Delay)
        case middleware.RetryTerm:
            return msg.Term()
        }
    }
    return msg.Ack()
})
```

For simpler cases, use `AutoAck` middleware with a handler-based approach:

```go
handler := goflux.Chain[Task](
    middleware.AutoAck[Task](),
)(func(ctx context.Context, msg goflux.Message[Task]) error {
    return processTask(ctx, msg.Payload)
})

err := sub.Subscribe(ctx, "tasks.>", handler)
```

## When to Use

- **Stream operators** -- use `bridge.ToStream` + goflow when you need filtering, deduplication, throttling, fan-out/fan-in, or other stream-processing operators
- **Simple handler chains** -- use `Chain` + middleware when you only need ack/nak behavior and the handler model is sufficient
- **Forwarding** -- use `bridge.FromStream` to publish processed stream elements to another transport
