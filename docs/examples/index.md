---
outline: deep
---

# Examples

Runnable code samples demonstrating common goflux patterns.

- [Basic Pub/Sub](./basic-pubsub) -- In-memory messaging with the channel transport
- [NATS Pipeline](./nats-pipeline) -- NATS transport with filtering and type mapping
- [HTTP Webhook](./http-webhook) -- HTTP POST transport for webhook-style messaging
- [Fan Patterns](./fan-patterns) -- Broadcasting, merging, and load distribution
- [Middleware Chain](./middleware-chain) -- Composing throttle, deduplicate, and concurrency control

::: info
Most examples use the `chan/` transport for simplicity (no external dependencies).
:::
