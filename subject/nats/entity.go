package nats

// Entity represents the entity segment of a nats.
type Entity struct {
	domain string
	name   string
}

// String returns the full nats up to and including the entity.
func (e Entity) String() string {
	return e.domain + "." + e.name
}

// All returns a NATS ">" wildcard nats matching everything under this entity.
func (e Entity) All() string {
	return e.String() + ".>"
}

// Event creates an Event from this entity.
func (e Entity) Event(name string) Event {
	validateSegment(name)
	return Event{entity: e.String(), name: name}
}
