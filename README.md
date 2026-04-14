[![Build Status](https://github.com/foomo/goflux/actions/workflows/test.yml/badge.svg?branch=main&event=push)](https://github.com/foomo/goflux/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/foomo/goflux)](https://goreportcard.com/report/github.com/foomo/goflux)
[![GoDoc](https://godoc.org/github.com/foomo/goflux?status.svg)](https://godoc.org/github.com/foomo/goflux)

<p align="center">
  <img alt="goflux" src="docs/public/logo.png" width="400" height="400"/>
</p>

# goflux

> Generic, transport-agnostic messaging patterns for Go.

Write business logic against core interfaces. Swap transports without touching handler code.

## Architecture

| Layer | What it provides |
|-------|-----------------|
| **Core Interfaces** | `Publisher[T]`, `Subscriber[T]`, `Consumer[T]`, `Requester[Req, Resp]`, `Responder[Req, Resp]`, `Message[T]`, `Handler[T]` |
| **Transports** | Channel (in-process), NATS, JetStream, HTTP — each implements the core interfaces |
| **Middleware** | `Chain`, `Process`, `Peek`, `Distinct`, `Skip`, `Take`, `Throttle`, `AutoAck` |
| **Pipeline Operators** | `Pipe`, `PipeMap`, `FanOut`, `FanIn`, `RoundRobin`, `BoundPublisher` |
| **Telemetry** | OpenTelemetry tracing and metrics built into every transport |

## Supported Patterns

- **Fire & Forget** — publish with no delivery guarantee (channels, NATS core)
- **At-Least-Once** — ack/nak with auto-ack or manual control (JetStream)
- **Pull Consumer** — fetch messages on demand with explicit ack (JetStream)
- **Request-Reply** — typed request/response (NATS, HTTP)
- **Queue Groups** — competing consumers (NATS)
- **Fan-Out / Fan-In** — broadcast, merge, round-robin (any transport)

## Transport Feature Matrix

| Interface | Channel | NATS | JetStream | HTTP |
|-----------|:-------:|:----:|:---------:|:----:|
| `Publisher[T]` | yes | yes | yes | yes |
| `Subscriber[T]` | yes | yes | yes | yes |
| `Consumer[T]` | - | - | yes | - |
| `Requester[Req, Resp]` | - | yes | - | yes |
| `Responder[Req, Resp]` | - | yes | - | yes |

## Installation

```bash
go get github.com/foomo/goflux
```

## Quick Start

```go
package main

import (
  "context"
  "fmt"

  "github.com/foomo/goflux"
  "github.com/foomo/goflux/transport/channel"
)

func main() {
  ctx, cancel := context.WithCancel(context.Background())
  defer cancel()

  bus := channel.NewBus[string]()
  pub := channel.NewPublisher(bus)
  sub, _ := channel.NewSubscriber(bus, 1)

  go sub.Subscribe(ctx, "greetings", func(_ context.Context, msg goflux.Message[string]) error {
    fmt.Println(msg.Subject, msg.Payload)
    cancel()
    return nil
  })

  _ = pub.Publish(ctx, "greetings", "Hello, goflux!")
  <-ctx.Done()
}
```

Swap to NATS by changing the import and constructor — the handler stays the same. See the [Getting Started](https://foomo.github.io/goflux/guide/getting-started) guide.

## Documentation

Full documentation: [https://foomo.github.io/goflux/](https://foomo.github.io/goflux/)

## Contributing

```bash
make check   # tidy + generate + lint + test + audit (full CI flow)
```

See [CONTRIBUTING.md](docs/CONTRIBUTING.md) for details.

![Contributors](https://contributors-table.vercel.app/image?repo=foomo/goflux&width=50&columns=15)

## License

Distributed under MIT License, see [LICENSE](LICENSE) for details.

_Made with ♥ [foomo](https://www.foomo.org) by [bestbytes](https://www.bestbytes.com)_
