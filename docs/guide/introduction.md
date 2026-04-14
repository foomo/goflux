# Introduction

## What is goflux?

goflux is a Go library that provides generic, type-safe abstractions over common messaging patterns. It is part of the [foomo](https://github.com/foomo) ecosystem.

Business logic is written against a small set of core interfaces -- `Publisher[T]`, `Subscriber[T]`, `Consumer[T]`, `Requester[Req, Resp]`, and `Responder[Req, Resp]`. Transport-specific configuration stays in the transport layer, and transports can be swapped without touching handler code.

```go
// The Publisher interface -- one method, fully generic.
type Publisher[T any] interface {
    Publish(ctx context.Context, subject string, v T) error
    Close() error
}
```

Every message is `Message[T]` -- fully decoded at the transport boundary. No raw bytes leak into handlers.

## Design Philosophy

- **Interfaces over implementations.** Core types are small interfaces. Transports implement them; your code depends only on the interfaces.
- **Generics for type safety.** `Message[T]` carries a decoded payload. The compiler catches type mismatches, not runtime panics.
- **Composition over configuration.** Pipelines, middleware, fan-out, and fan-in are plain functions that compose with each other. There is no framework to configure.
- **Caller owns connections.** Transport constructors accept an existing connection (e.g. `*nats.Conn`). The caller connects and closes; the transport never manages lifecycle behind your back.
- **Telemetry by default.** OpenTelemetry tracing and metrics are built into every transport. No opt-in required, no middleware to wire.

## Architecture

goflux is organized into five layers. Each layer depends only on the one above it:

```
+------------------------------------------------------------+
| Core Interfaces                                            |
| Publisher, Subscriber, Consumer, Requester, Responder,     |
| Handler, Message, Header, Acker                            |
+------------------------------------------------------------+
| Transports                                                 |
| channel (in-process), nats (NATS core),                    |
| jetstream (NATS JetStream), http (HTTP POST)               |
+------------------------------------------------------------+
| Middleware                                                  |
| Process, Peek, Distinct, Skip, Take, Throttle, AutoAck    |
+------------------------------------------------------------+
| Pipeline Operators                                         |
| Pipe, PipeMap, FanOut, FanIn, RoundRobin                   |
+------------------------------------------------------------+
| Telemetry                                                  |
| OTel tracing + metrics, context propagation, message ID    |
+------------------------------------------------------------+
```

Your code lives at the top -- it depends on core interfaces and optionally uses middleware and pipeline operators. The transport layer is the only part that changes when you switch from in-process channels to NATS, JetStream, or HTTP.

## Supported Patterns

| Pattern | Description | Transports |
|---------|-------------|------------|
| Fire-and-forget | Publish and move on, no delivery guarantee | channel, nats, http |
| At-least-once | Messages are acked/naked; redelivered on failure | jetstream |
| Pull-based | Consumer fetches batches at its own pace | jetstream |
| Request-reply | Send a request, wait for a typed response | nats, http |
| Queue groups | Load-balance messages across subscriber instances | nats, jetstream |
| Fan-out / Fan-in | Broadcast to N publishers or merge N subscribers | all (pipeline operators) |

## What's Next

- [Getting Started](./getting-started.md) -- install goflux and run your first example
- [Core Concepts](./core-concepts.md) -- learn the fundamental types and design rules
