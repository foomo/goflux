[![Build Status](https://github.com/foomo/goflux/actions/workflows/test.yml/badge.svg?branch=main&event=push)](https://github.com/foomo/goflux/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/foomo/goflux)](https://goreportcard.com/report/github.com/foomo/goflux)
[![GoDoc](https://godoc.org/github.com/foomo/goflux?status.svg)](https://godoc.org/github.com/foomo/goflux)

<p align="center">
  <img alt="goflux" src="docs/public/logo.png" width="400" height="400"/>
</p>

# goflux

> Type-safe, transport-agnostic pub/sub messaging for Go.

## Features

- Generic `Publisher[T]`, `Subscriber[T]`, `Handler[T]` interfaces
- Pluggable transports: in-process channels, NATS, HTTP
- Composable pipelines with filtering, mapping, and dead-letter handling
- Middleware: throttle, deduplicate, limit concurrency, and more
- OpenTelemetry tracing and metrics built into every transport

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
  _chan "github.com/foomo/goflux/pkg/chan"
)

func main() {
  ctx, cancel := context.WithCancel(context.Background())
  defer cancel()

  bus := _chan.NewBus[string]()
  pub := _chan.NewPublisher(bus)
  sub, _ := _chan.NewSubscriber(bus, 1)

  go sub.Subscribe(ctx, "greetings", func(_ context.Context, msg goflux.Message[string]) error {
    fmt.Println(msg.Subject, msg.Payload)
    cancel()
    return nil
  })

  pub.Publish(ctx, "greetings", "Hello, goflux!")
  <-ctx.Done()
}
```

## Available Transports

| Package | Transport | Use Case |
|---------|-----------|----------|
| `goflux/chan` | Go channels | Testing, in-process |
| `goflux/nats` | NATS core | Microservices |
| `goflux/http` | HTTP POST | Webhooks |

## Documentation

Full documentation available at [https://foomo.github.io/goflux/](https://foomo.github.io/goflux/)

## How to Contribute

Contributions are welcome! Please read the [contributing guide](docs/CONTRIBUTING.md).

![Contributors](https://contributors-table.vercel.app/image?repo=foomo/goflux&width=50&columns=15)

## License

Distributed under MIT License, please see the [license](LICENSE) file within the code for more details.

_Made with ♥ [foomo](https://www.foomo.org) by [bestbytes](https://www.bestbytes.com)_
