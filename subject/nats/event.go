package nats

// Event is the terminal segment of the builder chain. It always has a parent
// [Entity].
type Event struct {
	entity string
	name   string
}

// String returns the complete NATS subject.
func (ev Event) String() string {
	return ev.entity + "." + ev.name
}
