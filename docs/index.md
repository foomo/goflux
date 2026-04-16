---
layout: home

hero:
  name: goflux
  text: Messaging Patterns for Go
  tagline: Generic, transport-agnostic messaging with acknowledgments, request-reply, pull consumers, composable pipelines, and built-in observability.
  image:
    src: /logo.png
    alt: goflux
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: API Reference
      link: /reference/

features:
  - title: Type-Safe Generics
    details: Publisher[T], Subscriber[T], Requester[Req, Resp] with compile-time type safety. No interface{} casts.
  - title: Transport-Agnostic
    details: Write handlers once, swap between channels, NATS, JetStream, and HTTP without changing business logic.
  - title: Acknowledgment Control
    details: Auto-ack by default, explicit Ack/Nak/NakWithDelay/Term per message. Fire-and-forget or at-least-once delivery.
  - title: Request-Reply
    details: Type-safe Requester and Responder interfaces for synchronous messaging over NATS and HTTP.
  - title: Pipeline Operators
    details: Pipe, PipeMap for message routing and transformation. ToStream/FromStream bridge to goflow for fan-out, fan-in, and stream processing.
  - title: OpenTelemetry Built-In
    details: Tracing and metrics baked into every transport. Span links for async, parent-child for sync. No middleware setup required.
---
