// Package goflux provides generic, transport-agnostic pub/sub messaging
// patterns for Go.
//
// Business logic is written against the core interfaces — [Publisher],
// [Subscriber], [Handler], [Message] — and transports are swapped without
// touching handler code. Concrete transports live in submodules under
// transport/ (in-process channels, NATS core, JetStream, HTTP); stream
// bridging to github.com/foomo/goflow lives in the bridge/ submodule.
package goflux
