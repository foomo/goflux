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

// Domain creates a Domain from this prefix.
func (p Prefix) Domain(name string) Domain {
	validateSegment(name)
	return Domain{prefix: p.joined, name: name}
}

// Domain represents the domain segment of a subject.
type Domain struct {
	prefix string
	name   string
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

// Entity creates an Entity from this domain.
func (d Domain) Entity(name string) Entity {
	validateSegment(name)
	return Entity{domain: d.String(), name: name}
}

// Entity represents the entity segment of a subject.
type Entity struct {
	domain string
	name   string
}

// String returns the full subject up to and including the entity.
func (e Entity) String() string {
	return e.domain + "." + e.name
}

// All returns a NATS ">" wildcard subject matching everything under this entity.
func (e Entity) All() string {
	return e.String() + ".>"
}

// Event creates an Event from this entity.
func (e Entity) Event(name string) Event {
	validateSegment(name)
	return Event{entity: e.String(), name: name}
}

// Event represents the terminal event segment of a subject.
type Event struct {
	entity string
	name   string
}

// String returns the complete subject string.
func (ev Event) String() string {
	return ev.entity + "." + ev.name
}
