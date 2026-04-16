# Channel Transport

Package `github.com/foomo/goflux/transport/channel`

The channel transport is an in-process pub/sub broker built on typed Go channels. Messages pass as-is with no serialization. It is useful for decoupling components within a single process, for testing, and as a building block for pipelines.

## Interfaces

| Interface | Implemented |
|-----------|-------------|
| `Publisher[T]` | Yes |
| `Subscriber[T]` | Yes |
| `Requester[Req, Resp]` | No |
| `Responder[Req, Resp]` | No |

## Bus

`Bus[T]` is the central broker. Subjects are matched by exact string equality. A Bus is safe for concurrent use.

```go
func NewBus[T any]() *Bus[T]
```

## Publisher

```go
func NewPublisher[T any](bus *Bus[T], opts ...Option) *Publisher[T]
```

`Publish` blocks until every subscriber on the subject has accepted the message or the context is cancelled. No messages are dropped; slow consumers apply backpressure to the publisher goroutine.

`Close` is a no-op. The caller owns the Bus.

## Subscriber

```go
func NewSubscriber[T any](bus *Bus[T], bufSize int, opts ...Option) (*Subscriber[T], error)
```

`bufSize` controls the internal channel buffer. A zero value means unbuffered (strict synchronous handoff). `Subscribe` blocks until the context is cancelled.

`Close` is a no-op.

## Options

| Option | Description |
|--------|-------------|
| `WithTelemetry(t *goflux.Telemetry)` | Sets the OTel telemetry instance. A default is created from OTel globals if not provided. |

## Behavior

- **No codec required** -- messages are passed through typed channels as-is.
- **Backpressure** -- the publisher blocks on each subscriber's channel send. If any subscriber is slow, the publisher goroutine is blocked until all subscribers accept.
- **Headers** -- `goflux.Header` from the context is propagated in-memory on the `Message`.
- **Acker** -- `Message.Acker` is nil. Channel transport is fire-and-forget with no acknowledgement semantics.
- **Telemetry** -- publish and process operations are recorded via `RecordPublish` and `RecordProcess`.

## Example

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/foomo/goflux"
	"github.com/foomo/goflux/transport/channel"
)

type Event struct {
	ID   string
	Name string
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create a bus and wire up publisher + subscriber.
	bus := channel.NewBus[Event]()

	pub := channel.NewPublisher[Event](bus)

	sub, err := channel.NewSubscriber[Event](bus, 16) // buffered channel of 16
	if err != nil {
		log.Fatal(err)
	}

	// Subscribe in a goroutine -- Subscribe blocks until ctx is cancelled.
	go func() {
		_ = sub.Subscribe(ctx, "events.created", func(ctx context.Context, msg goflux.Message[Event]) error {
			fmt.Printf("received: %s %s\n", msg.Payload.ID, msg.Payload.Name)
			return nil
		})
	}()

	// Publish a message.
	if err := pub.Publish(ctx, "events.created", Event{ID: "1", Name: "signup"}); err != nil {
		log.Fatal(err)
	}

	cancel()
}
```
