---
layout: home

hero:
  name: goflux
  text: Type-Safe Pub/Sub for Go
  tagline: Generic, transport-agnostic messaging with composable pipelines, middleware, and built-in observability.
  image:
    src: /logo.png
    alt: goflux
  actions:
    - theme: brand
      text: Get Started
      link: /guide/getting-started
    - theme: alt
      text: API Reference
      link: /api/

features:
  - title: Type-Safe Generics
    details: Publisher[T], Subscriber[T], Message[T] with compile-time type safety. No interface{} casts.
  - title: Transport-Agnostic
    details: Write handlers once, swap between channels, NATS, and HTTP without changing business logic.
  - title: Pipeline Operators
    details: Pipe, PipeMap, Filter, and DeadLetter for composable message routing and transformation.
  - title: Middleware
    details: Chain, Process, Peek, Distinct, Skip, Take, and Throttle as composable handler decorators.
  - title: Distribution Patterns
    details: FanOut, FanIn, and RoundRobin for broadcasting, merging, and load distribution.
  - title: OpenTelemetry Built-In
    details: Tracing and metrics baked into every transport. No middleware setup required.
---
