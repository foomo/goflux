package nats

// Entity is the entity segment of a NATS subject. It always has a parent
// [Domain].
type Entity struct {
	domain string
	name   string
}

// String returns the subject up to and including the entity segment.
func (e Entity) String() string {
	return e.domain + "." + e.name
}

// All returns the NATS multi-token wildcard subject that matches every
// subject nested below this entity (the entity path followed by ".>").
func (e Entity) All() string {
	return e.String() + ".>"
}

// Event extends the entity with a terminal event segment and returns the
// resulting [Event]. Event panics if name is not a valid NATS token.
func (e Entity) Event(name string) Event {
	validateSegment(name)
	return Event{entity: e.String(), name: name}
}
