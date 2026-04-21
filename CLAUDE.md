# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Module

```
github.com/foomo/goflux
```

Multi-module workspace — submodules in `pkg/channel/`, `pkg/nats/`, `pkg/http/` each have their own `go.mod`. Use
`go work` (auto-initialized by `make`).

## Commands

```sh
make check          # tidy + generate + lint + test + audit (full CI flow)
make test           # go test -tags=safe -shuffle=on across all modules
make test.race      # tests with -race
make lint           # golangci-lint across all modules
make lint.fix       # golangci-lint --fix
make tidy           # go mod tidy across all modules + go work sync
make audit          # trivy vulnerability scan
```

Run a single test:

```sh
go test -tags=safe -run TestName ./...
```

Run tests in a submodule:

```sh
cd transport/nats && go test -tags=safe -v ./...
```

Build tag `-tags=safe` is required (configured in `.golangci.yaml`).

## Toolchain (managed by mise)

mise installs: golangci-lint, lefthook, trivy, bun (for docs). Run `mise install` or just `make` (auto-triggers).

## Git Conventions

Enforced by lefthook pre-commit and commit-msg hooks:

- **Branch names**: must start with `feature/` or `fix/`
- **Commit messages**: Conventional Commits format — `type(scope?): subject` (max 50 chars in subject). Types:
  `build|chore|ci|docs|feat|fix|perf|refactor|style|test|sec|wip|revert`
- **Pre-commit**: runs `golangci-lint fmt` on staged `.go` files and `golangci-lint run --new --fast-only`

## Architecture

goflux is a generic, transport-agnostic pub/sub messaging library. Business logic is written against core interfaces;
transports are swapped without touching handler code.

### Core Interfaces (root package)

- `Publisher[T]` — publishes typed messages to a subject
- `Subscriber[T]` — subscribes to a subject, dispatches decoded `Message[T]` to a `Handler[T]`
- `Handler[T]` — `func(ctx context.Context, msg Message[T]) error`
- `Message[T]` — carries `Subject` (routing key) and `Payload T` (always fully decoded, no raw bytes)
- `Topic[T]` — convenience struct embedding both Publisher and Subscriber

### Transports (submodules)

| Package        | Transport           | Notes                                                                                                                          |
|----------------|---------------------|--------------------------------------------------------------------------------------------------------------------------------|
| `pkg/channel/` | In-process channels | `Bus[T]` broker, backpressure by blocking, no codec needed                                                                     |
| `pkg/nats/`    | NATS core           | Wraps `*nats.Conn`, publisher takes `goencode.Encoder[T, []byte]`, subscriber takes `goencode.Decoder[T, []byte]`, no ack/nack |
| `pkg/http/`    | HTTP POST           | Publisher POSTs to `{baseURL}/{subject}`, Subscriber exposes `http.ServeMux` (does not own listener)                           |
| `bridge/`      | goflow stream       | Own `go.mod`, isolates goflow dependency from root module                                                                      |

Network transports take `goencode.Encoder[T, []byte]` for publishers and `goencode.Decoder[T, []byte]` for subscribers.
Both are function types from `github.com/foomo/goencode`, composable via `PipeEncoder`/`PipeDecoder`. A
`goencode.Codec[T, []byte]` provides both as `.Encode`/`.Decode` method values. Requester/Responder still take full
`Codec` params.

### Pipeline Operators (`pipe/` subpackage)

Handler factories that compose `Handler[T]` and `Publisher[T]`:

- `pipe.New[T]` — forward messages through optional filter to a publisher
- `pipe.NewMap[T, U]` — transform T→U before publishing
- `pipe.NewFlatMap[T, U]` — expand T into []U, publish each individually
- Options: `WithFilter`, `WithDeadLetter`, `WithMiddleware` (and `WithMapFilter`, `WithMapDeadLetter`,
  `WithMapMiddleware` for map variants)
- Error semantics: filter rejection returns nil (ack), map/publish errors return to transport
- OTel: span events (`pipe.dead_letter`, `pipe.publish_error`, `pipe.map_error`) and attributes (`pipe.type`,
  `pipe.filtered`, `pipe.items_published`) on existing transport span

### Adapters

- `ToChan[T]` (root package) — bridge subscriber to `<-chan Message[T]`
- `bridge.ToStream[T]` / `bridge.FromStream[T]` (`bridge/` submodule) — bridge to/from `goflow.Stream[Message[T]]`
- `Bind[T]` — fixed-subject publisher wrapper
- `RetryPublisher[T]` — publish retry with backoff

Stream-processing operators (fan-out, fan-in, round-robin, filter, map, distinct, skip, take, throttle, peek) are
provided by `github.com/foomo/goflow` — use `bridge.ToStream` to bridge.

### Middleware (`middleware/` subpackage)

`Middleware[T]` type, `PublisherMiddleware[T]` type, and `Chain[T]` live in the root package. Messaging-specific
middleware lives in `github.com/foomo/goflux/middleware`:

- `middleware.AutoAck[T]()` — ack on nil error, nak on non-nil
- `middleware.RetryAck[T](policy)` — retry-policy-aware ack (nak/nak-with-delay/term based on error)
- `middleware.InjectMessageID[T]()` / `middleware.InjectHeader[T]()` — inject header values into context
- `middleware.ForwardMessageID[T]()` — forward message ID from context through pipe stages

### Telemetry

OTel is built into transports, not added as middleware. Package-level singleton initialized via `sync.Once` against OTel
globals.

- Transports call `RecordPublish()` / `RecordProcess()` directly
- All metrics follow `messaging.*` semconv naming
- `ResetForTest()` resets the singleton for test isolation
- New transports must call `RecordPublish`/`RecordProcess` and declare a `system` var

### Context Propagation

- NATS and HTTP transports automatically propagate OTel trace context via transport headers (W3C Trace Context)
- `InjectContext(ctx, carrier)` / `ExtractContext(ctx, carrier)` — thin wrappers around the global `TextMapPropagator`
- `WithMessageID(ctx, id)` / `MessageID(ctx)` — opt-in business-level message ID, propagated via `X-Message-ID` header
  and attached as `messaging.message.id` span attribute
- New transports crossing process boundaries should call `InjectContext` on publish and `ExtractContext` on subscribe

### Goroutine Management

Uses `github.com/foomo/gofuncy` for goroutine lifecycle management with built-in OTel observability. Production code
uses `gofuncy.Go` (fire-and-forget) and `gofuncy.All` (concurrent iteration). Tests use `gofuncy.Start`/
`gofuncy.StartWithReady` for synchronization. All calls should include `gofuncy.WithName()` for tracing labels.

## Key Design Rules

- `Subscribe` always blocks until ctx is cancelled — run in a goroutine
- Transport constructors do not own connections — caller connects and closes
- `Publisher.Close` on `chan/` is a no-op; caller owns inner publishers
- Non-nil error from handler signals failure; semantics are transport-specific
- `Bus[T]` is internal to `pkg/channel/` — not a cross-cutting interface
