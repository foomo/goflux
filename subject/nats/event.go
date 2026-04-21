package nats

// Event represents the terminal event segment of a nats.
type Event struct {
	entity string
	name   string
}

// String returns the complete nats string.
func (ev Event) String() string {
	return ev.entity + "." + ev.name
}
