# Patterns

Most patterns have a **Diagram** tab next to the code. The diagrams are animations. Blue elements are messages going in. Green elements are handled messages, acks and replies. Purple elements are transformed messages. A red element is an error, a nak or a dropped message.

| Pattern | Core Interface(s) | Transport(s) |
|---------|-------------------|--------------|
| [Fire and Forget](./fire-and-forget.md) | `Publisher` + `Subscriber`, acker=nil | channel, nats |
| [At-Least-Once](./at-least-once.md) (auto-ack) | `Publisher` + `Subscriber`, auto-ack | jetstream |
| [At-Least-Once](./at-least-once.md) (manual ack) | `Publisher` + `Subscriber` + `WithManualAck`, middleware ack | jetstream |
| [Pull Consumer](./pull-consumer.md) | `Subscriber` + consumer config + middleware | jetstream |
| Manual Ack/Nak/Term | `Subscriber` + `WithManualAck()` | jetstream |
| Exactly-Once (publish dedup) | `Publisher` + `goflux.WithMessageID` (sent as `Nats-Msg-Id`) | jetstream |
| [Request-Reply](./request-reply.md) | `Requester` + `Responder` | nats, http |
| [Queue Groups](./queue-groups.md) | `Subscriber` + `WithQueueGroup` | nats |
| [Fan-out](./fan-out-fan-in.md) | `bridge.ToStream` + goflow `Tee` / `FanOut` | any |
| [Fan-in](./fan-out-fan-in.md) | `bridge.ToStream` + goflow `FanIn` | any |
| [Stream Processing](./stream-processor.md) | `bridge.ToStream` + goflow operators | any |
| [Headers](./headers.md) | `Message.Header` + header middleware | any |
