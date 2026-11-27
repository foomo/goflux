# Transports

Every transport implements the core interfaces. Swap transports without touching handler code.

## Support Matrix

| Interface | Channel | NATS | JetStream | HTTP |
|-----------|---------|------|-----------|------|
| `Publisher[T]` | yes | yes | yes | yes |
| `Subscriber[T]` | yes | yes | yes | yes |
| `Requester[Req, Resp]` | - | yes | - | yes |
| `Responder[Req, Resp]` | - | yes | - | yes |

## Packages

| Transport | Package | API |
|-----------|---------|-----|
| [Channel](./channel.md) | `github.com/foomo/goflux/transport/channel` | [pkg.go.dev](https://pkg.go.dev/github.com/foomo/goflux/transport/channel) |
| [NATS](./nats.md) | `github.com/foomo/goflux/transport/nats` (separate module) | [pkg.go.dev](https://pkg.go.dev/github.com/foomo/goflux/transport/nats) |
| [JetStream](./jetstream.md) | `github.com/foomo/goflux/transport/jetstream` (separate module) | [pkg.go.dev](https://pkg.go.dev/github.com/foomo/goflux/transport/jetstream) |
| [HTTP](./http.md) | `github.com/foomo/goflux/transport/http` | [pkg.go.dev](https://pkg.go.dev/github.com/foomo/goflux/transport/http) |
