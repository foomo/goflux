# NATS Subject Builder Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a fluent, type-safe NATS subject builder to `transport/nats/subject/` with typed intermediates, validation, and wildcard support.

**Architecture:** Four immutable struct types (`Prefix`, `Domain`, `Entity`, `Event`) chained via methods. Each accumulates joined string segments internally for efficient `String()`. Validation panics on invalid NATS tokens at construction time.

**Tech Stack:** Go standard library only. No external dependencies.

---

## File Structure

| File | Responsibility |
|------|---------------|
| `transport/nats/subject/subject.go` | Type definitions, constructors, chain methods, String(), wildcards |
| `transport/nats/subject/validate.go` | `validateSegment()` helper — panics on invalid NATS tokens |
| `transport/nats/subject/subject_test.go` | All tests: chain, String(), wildcards, validation panics |

---

### Task 1: Validation Helper

**Files:**
- Create: `transport/nats/subject/validate.go`
- Create: `transport/nats/subject/subject_test.go`

- [ ] **Step 1: Write failing tests for validation**

In `transport/nats/subject/subject_test.go`:

```go
package subject

import (
	"testing"
)

func TestValidateSegment(t *testing.T) {
	t.Run("valid segments", func(t *testing.T) {
		valid := []string{"user", "prod", "tenant-123", "my_domain", "v2"}
		for _, s := range valid {
			t.Run(s, func(t *testing.T) {
				validateSegment(s) // should not panic
			})
		}
	})

	t.Run("empty string panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for empty segment")
			}
		}()
		validateSegment("")
	})

	t.Run("dot panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for segment containing dot")
			}
		}()
		validateSegment("user.profile")
	})

	t.Run("star panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for segment containing *")
			}
		}()
		validateSegment("user*")
	})

	t.Run("chevron panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for segment containing >")
			}
		}()
		validateSegment("all>")
	})

	t.Run("whitespace panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for segment containing whitespace")
			}
		}()
		validateSegment("user profile")
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd transport/nats && go test -tags=safe -run TestValidateSegment ./subject/ -v`
Expected: FAIL — `validateSegment` not defined.

- [ ] **Step 3: Implement validateSegment**

In `transport/nats/subject/validate.go`:

```go
package subject

import (
	"fmt"
	"strings"
)

// validateSegment panics if the segment is not a valid NATS subject token.
func validateSegment(s string) {
	if s == "" {
		panic("subject: segment must not be empty")
	}
	if strings.ContainsAny(s, ".*> \t\n\r") {
		panic(fmt.Sprintf("subject: segment %q contains invalid character (one of: . * > or whitespace)", s))
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd transport/nats && go test -tags=safe -run TestValidateSegment ./subject/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add transport/nats/subject/validate.go transport/nats/subject/subject_test.go
git commit -m "feat(nats/subject): add segment validation helper"
```

---

### Task 2: Prefix Type

**Files:**
- Create: `transport/nats/subject/subject.go`
- Modify: `transport/nats/subject/subject_test.go`

- [ ] **Step 1: Write failing tests for Prefix**

Append to `transport/nats/subject/subject_test.go`:

```go
func TestNewPrefix(t *testing.T) {
	t.Run("zero segments", func(t *testing.T) {
		p := NewPrefix()
		if got := p.String(); got != "" {
			t.Errorf("got %q, want %q", got, "")
		}
	})

	t.Run("single segment", func(t *testing.T) {
		p := NewPrefix("prod")
		if got := p.String(); got != "prod" {
			t.Errorf("got %q, want %q", got, "prod")
		}
	})

	t.Run("multiple segments", func(t *testing.T) {
		p := NewPrefix("prod", "acme")
		if got := p.String(); got != "prod.acme" {
			t.Errorf("got %q, want %q", got, "prod.acme")
		}
	})

	t.Run("invalid segment panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for invalid prefix segment")
			}
		}()
		NewPrefix("prod", "bad.segment")
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd transport/nats && go test -tags=safe -run TestNewPrefix ./subject/ -v`
Expected: FAIL — `NewPrefix` not defined.

- [ ] **Step 3: Implement Prefix**

In `transport/nats/subject/subject.go`:

```go
package subject

import "strings"

// Prefix represents zero or more leading segments (e.g., env, tenant) prepended to all subjects.
type Prefix struct {
	joined string
}

// NewPrefix creates a reusable prefix from zero or more segments.
// Zero segments is valid — Domain becomes the first token in the subject.
func NewPrefix(segments ...string) Prefix {
	for _, s := range segments {
		validateSegment(s)
	}
	return Prefix{joined: strings.Join(segments, ".")}
}

// String returns the dot-joined prefix segments.
func (p Prefix) String() string {
	return p.joined
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd transport/nats && go test -tags=safe -run TestNewPrefix ./subject/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add transport/nats/subject/subject.go transport/nats/subject/subject_test.go
git commit -m "feat(nats/subject): add Prefix type with String()"
```

---

### Task 3: Domain Type

**Files:**
- Modify: `transport/nats/subject/subject.go`
- Modify: `transport/nats/subject/subject_test.go`

- [ ] **Step 1: Write failing tests for Domain**

Append to `transport/nats/subject/subject_test.go`:

```go
func TestDomain(t *testing.T) {
	t.Run("from prefix", func(t *testing.T) {
		d := NewPrefix("prod", "acme").Domain("user")
		if got := d.String(); got != "prod.acme.user" {
			t.Errorf("got %q, want %q", got, "prod.acme.user")
		}
	})

	t.Run("from empty prefix", func(t *testing.T) {
		d := NewPrefix().Domain("user")
		if got := d.String(); got != "user" {
			t.Errorf("got %q, want %q", got, "user")
		}
	})

	t.Run("NewDomain shortcut", func(t *testing.T) {
		d := NewDomain("order")
		if got := d.String(); got != "order" {
			t.Errorf("got %q, want %q", got, "order")
		}
	})

	t.Run("All wildcard from prefix", func(t *testing.T) {
		d := NewPrefix("prod", "acme").Domain("user")
		if got := d.All(); got != "prod.acme.user.>" {
			t.Errorf("got %q, want %q", got, "prod.acme.user.>")
		}
	})

	t.Run("All wildcard no prefix", func(t *testing.T) {
		d := NewDomain("user")
		if got := d.All(); got != "user.>" {
			t.Errorf("got %q, want %q", got, "user.>")
		}
	})

	t.Run("invalid name panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for invalid domain name")
			}
		}()
		NewDomain("")
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd transport/nats && go test -tags=safe -run TestDomain ./subject/ -v`
Expected: FAIL — `Domain` type and methods not defined.

- [ ] **Step 3: Implement Domain**

Append to `transport/nats/subject/subject.go`:

```go
// Domain represents the domain segment of a subject.
type Domain struct {
	prefix string
	name   string
}

// Domain creates a Domain from this prefix.
func (p Prefix) Domain(name string) Domain {
	validateSegment(name)
	return Domain{prefix: p.joined, name: name}
}

// NewDomain creates a Domain directly when no prefix is needed.
func NewDomain(name string) Domain {
	validateSegment(name)
	return Domain{name: name}
}

// String returns the full subject up to and including the domain.
func (d Domain) String() string {
	if d.prefix == "" {
		return d.name
	}
	return d.prefix + "." + d.name
}

// All returns a NATS ">" wildcard subject matching everything under this domain.
func (d Domain) All() string {
	return d.String() + ".>"
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd transport/nats && go test -tags=safe -run TestDomain ./subject/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add transport/nats/subject/subject.go transport/nats/subject/subject_test.go
git commit -m "feat(nats/subject): add Domain type with All() wildcard"
```

---

### Task 4: Entity Type

**Files:**
- Modify: `transport/nats/subject/subject.go`
- Modify: `transport/nats/subject/subject_test.go`

- [ ] **Step 1: Write failing tests for Entity**

Append to `transport/nats/subject/subject_test.go`:

```go
func TestEntity(t *testing.T) {
	t.Run("full chain", func(t *testing.T) {
		e := NewPrefix("prod", "acme").Domain("user").Entity("profile")
		if got := e.String(); got != "prod.acme.user.profile" {
			t.Errorf("got %q, want %q", got, "prod.acme.user.profile")
		}
	})

	t.Run("no prefix", func(t *testing.T) {
		e := NewDomain("user").Entity("profile")
		if got := e.String(); got != "user.profile" {
			t.Errorf("got %q, want %q", got, "user.profile")
		}
	})

	t.Run("All wildcard", func(t *testing.T) {
		e := NewPrefix("prod", "acme").Domain("user").Entity("profile")
		if got := e.All(); got != "prod.acme.user.profile.>" {
			t.Errorf("got %q, want %q", got, "prod.acme.user.profile.>")
		}
	})

	t.Run("All wildcard no prefix", func(t *testing.T) {
		e := NewDomain("order").Entity("item")
		if got := e.All(); got != "order.item.>" {
			t.Errorf("got %q, want %q", got, "order.item.>")
		}
	})

	t.Run("invalid name panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for invalid entity name")
			}
		}()
		NewDomain("user").Entity("")
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd transport/nats && go test -tags=safe -run TestEntity ./subject/ -v`
Expected: FAIL — `Entity` type and methods not defined.

- [ ] **Step 3: Implement Entity**

Append to `transport/nats/subject/subject.go`:

```go
// Entity represents the entity segment of a subject.
type Entity struct {
	domain string
	name   string
}

// Entity creates an Entity from this domain.
func (d Domain) Entity(name string) Entity {
	validateSegment(name)
	return Entity{domain: d.String(), name: name}
}

// String returns the full subject up to and including the entity.
func (e Entity) String() string {
	return e.domain + "." + e.name
}

// All returns a NATS ">" wildcard subject matching everything under this entity.
func (e Entity) All() string {
	return e.String() + ".>"
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd transport/nats && go test -tags=safe -run TestEntity ./subject/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add transport/nats/subject/subject.go transport/nats/subject/subject_test.go
git commit -m "feat(nats/subject): add Entity type with All() wildcard"
```

---

### Task 5: Event Type

**Files:**
- Modify: `transport/nats/subject/subject.go`
- Modify: `transport/nats/subject/subject_test.go`

- [ ] **Step 1: Write failing tests for Event**

Append to `transport/nats/subject/subject_test.go`:

```go
func TestEvent(t *testing.T) {
	t.Run("full chain with prefix", func(t *testing.T) {
		ev := NewPrefix("prod", "acme").Domain("user").Entity("profile").Event("updated")
		if got := ev.String(); got != "prod.acme.user.profile.updated" {
			t.Errorf("got %q, want %q", got, "prod.acme.user.profile.updated")
		}
	})

	t.Run("no prefix", func(t *testing.T) {
		ev := NewDomain("order").Entity("item").Event("created")
		if got := ev.String(); got != "order.item.created" {
			t.Errorf("got %q, want %q", got, "order.item.created")
		}
	})

	t.Run("invalid name panics", func(t *testing.T) {
		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic for invalid event name")
			}
		}()
		NewDomain("user").Entity("profile").Event("")
	})
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd transport/nats && go test -tags=safe -run TestEvent ./subject/ -v`
Expected: FAIL — `Event` type and methods not defined.

- [ ] **Step 3: Implement Event**

Append to `transport/nats/subject/subject.go`:

```go
// Event represents the terminal event segment of a subject.
type Event struct {
	entity string
	name   string
}

// Event creates an Event from this entity.
func (e Entity) Event(name string) Event {
	validateSegment(name)
	return Event{entity: e.String(), name: name}
}

// String returns the complete subject string.
func (ev Event) String() string {
	return ev.entity + "." + ev.name
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd transport/nats && go test -tags=safe -run TestEvent ./subject/ -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add transport/nats/subject/subject.go transport/nats/subject/subject_test.go
git commit -m "feat(nats/subject): add Event type (terminal segment)"
```

---

### Task 6: Lint and Final Verification

**Files:**
- All files in `transport/nats/subject/`

- [ ] **Step 1: Run full test suite**

Run: `cd transport/nats && go test -tags=safe -shuffle=on ./subject/ -v`
Expected: All tests PASS.

- [ ] **Step 2: Run linter**

Run: `cd /Users/franklin/Workingcopies/github.com/foomo/goflux && make lint`
Expected: No lint errors.

- [ ] **Step 3: Run tidy**

Run: `cd /Users/franklin/Workingcopies/github.com/foomo/goflux && make tidy`
Expected: No changes needed (subject package has no external deps).

- [ ] **Step 4: Commit if any fixes needed**

Only if lint/tidy produced changes:
```bash
git add -A
git commit -m "chore: fix lint issues in nats/subject package"
```
