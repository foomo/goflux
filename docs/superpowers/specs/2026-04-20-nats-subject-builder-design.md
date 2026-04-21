# NATS Subject Builder

**Date:** 2026-04-20
**Status:** Approved

## Problem

goflux uses raw strings for NATS subjects everywhere. No structure, no validation, no reusable patterns. Users concatenate strings manually, which is error-prone and gives no compile-time safety.

## Solution

A fluent builder in `subject/nats/` that produces structured NATS subjects with typed intermediates. Each level in the chain returns a distinct type, enabling compile-time enforcement of subject structure and use as typed function arguments.

## Types

```
Prefix → Domain → Entity → Event
```

Each type is an immutable struct holding accumulated segments. Each implements `String() string`.

### Type Definitions

```go
type Prefix struct{ segments []string }
type Domain struct{ prefix string; name string }
type Entity struct{ domain string; name string }
type Event  struct{ entity string; name string }
```

Internal string fields store the already-joined prefix portion for efficient `String()` calls.

## API

### Entry Points

```go
// NewPrefix creates a reusable prefix from one or more segments (e.g., env, tenant).
// Zero segments is valid — Domain becomes the first token in the nats.
func NewPrefix(segments ...string) Prefix

// NewDomain creates a Domain directly when no prefix is needed.
func NewDomain(name string) Domain
```

### Chain Methods

```go
func (p Prefix) Domain(name string) Domain
func (d Domain) Entity(name string) Entity
func (e Entity) Event(name string) Event
```

### Wildcard Methods

```go
// All returns a NATS ">" wildcard nats at the given level.
func (d Domain) All() string   // "prefix.domain.>"
func (e Entity) All() string   // "prefix.domain.entity.>"
```

### String Methods

```go
func (p Prefix) String() string  // "seg1.seg2"
func (d Domain) String() string  // "prefix.domain"
func (e Entity) String() string  // "prefix.domain.entity"
func (ev Event) String() string  // "prefix.domain.entity.event"
```

## Validation

Segments are validated at construction time. Invalid segments cause a **panic** — these are programming errors, not runtime input.

Invalid segment conditions:
- Empty string
- Contains `.` (NATS token separator)
- Contains `*` (NATS single-token wildcard)
- Contains `>` (NATS multi-token wildcard)
- Contains whitespace

## Usage Examples

### Full subject with prefix

```go
prefix := subject.NewPrefix("prod", "acme")
s := prefix.Domain("user").Entity("profile").Event("updated")
s.String() // "prod.acme.user.profile.updated"
```

### No prefix

```go
s := subject.NewDomain("order").Entity("item").Event("created")
s.String() // "order.item.created"
```

### Wildcards for subscriptions

```go
prefix := subject.NewPrefix("prod", "acme")
prefix.Domain("user").All()                   // "prod.acme.user.>"
prefix.Domain("user").Entity("profile").All() // "prod.acme.user.profile.>"
```

### Typed arguments for compile-time safety

```go
// Function requires at least a Domain-level nats
func SubscribeUserEvents(ctx context.Context, domain subject.Domain) error {
    return subscriber.Subscribe(ctx, domain.All(), handler)
}

// Caller must provide a properly constructed Domain
prefix := subject.NewPrefix("prod", "acme")
SubscribeUserEvents(ctx, prefix.Domain("user"))
```

### Partial subjects

```go
prefix := subject.NewPrefix("staging", "tenant-42")
prefix.String()                        // "staging.tenant-42"
prefix.Domain("order").String()        // "staging.tenant-42.order"
prefix.Domain("order").Entity("item").String() // "staging.tenant-42.order.item"
```

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Immutable structs | Yes | No mutation bugs, safe for concurrent use |
| Panic on invalid segments | Yes | Programming error, not user input |
| Wildcards on Domain/Entity only | Yes | `Prefix.All()` rarely useful; `Event` is terminal |
| No `goflux` core dependency | Yes | Pure utility, no coupling |
| Package in `subject/nats/` | Yes | Transport-specific, not cross-cutting |
| `String()` on all types | Yes | Partial subjects valid for NATS wildcards and subscriptions |

## Testing Strategy

- Table-driven tests for each chain combination
- Panic tests for invalid segments (empty, dots, wildcards, whitespace)
- Verify `String()` output at each level
- Verify wildcard output at Domain and Entity levels
- Test `NewPrefix` with zero, one, and multiple segments
- Test `NewDomain` as prefix-free entry point

## Future Considerations

- Other transports (HTTP, JetStream) may get their own `subject/` packages following same pattern
- JetStream may reuse NATS subject builder since JetStream subjects follow NATS conventions
