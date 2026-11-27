# Migrating to v0.12

v0.12 contains breaking changes. goflux is pre-1.0, so they ship in a minor release.

## Received message ID moved to `Message.ID`

Subscribers no longer put the received message ID into the handler context. They set `msg.ID` instead, the same way `msg.Header` carries received headers. Publishers still read the outgoing ID from `goflux.WithMessageID(ctx)`.

Before, every publish from a handler inherited the received ID. On JetStream the ID is also the `Nats-Msg-Id`, so a handler publishing into the same stream was silently deduplicated.

This change compiles without errors: `goflux.MessageID(ctx)` in a handler now returns `""`.

```go
// Before
id := goflux.MessageID(ctx)

// After
id := msg.ID
```

To publish with the received ID, forward it explicitly:

```go
handler := pipe.New[Event](pub,
    pipe.WithMiddleware(middleware.ForwardMessageID[Event]()),
)
```

Request-reply responders record the request ID on the process span only. See [Message ID](/integrations/opentelemetry#message-id).

## `channel.NewSubscriber` returns no error

The lag gauge is now registered per subscription, so the constructor cannot fail.

```go
// Before
sub, err := channel.NewSubscriber(bus, 16)

// After
sub := channel.NewSubscriber(bus, 16)
```

## `Telemetry.RegisterLag` returns an unregister function

```go
// Before
gauge, err := tel.RegisterLag(subject, lagFn)

// After
unregister := tel.RegisterLag(subject, lagFn)
defer unregister()
```

The gauge honors `WithDestinationTemplate` and sums the lag of all registrations with the same destination.

## `gofluxconv.NewConsumerLag` creates the instrument only

```go
// Before
lag, err := gofluxconv.NewConsumerLag(meter, subject, lagFn)

// After
lag, err := gofluxconv.NewConsumerLag(meter)
_, err = meter.RegisterCallback(func(_ context.Context, obs metric.Observer) error {
    lag.Observe(obs, lagFn(), lag.AttrDestinationName(subject))
    return nil
}, lag.Inst())
```

## `Message` JSON key `nats` renamed to `subject`

`Message.Subject` was serialized under the misnamed key `"nats"`. It is now `"subject"`, and `Message` gained an `"id"` key for `Message.ID`. Transports send only the payload, so this only affects code that stores or sends whole `Message` values as JSON.

Decoding still accepts the old `"nats"` key, so stored messages load unchanged. Older versions cannot read the new `"subject"` key, so upgrade readers before writers.
