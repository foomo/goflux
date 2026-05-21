package nats

import "strings"

// Subject is the root of the builder chain. It holds zero or more leading
// segments (for example an environment or tenant) that are prepended to every
// subject derived from it.
type Subject struct {
	joined string
}

// New returns a [Subject] built from the given segments. Calling New with no
// segments yields an empty prefix, so the next [Subject.Domain] becomes the
// first token of the resulting NATS subject.
//
// New panics if any segment is empty or contains ".", "*", ">", or whitespace.
func New(segments ...string) Subject {
	for _, s := range segments {
		validateSegment(s)
	}

	return Subject{joined: strings.Join(segments, ".")}
}

// String returns the dot-joined prefix segments, or the empty string if no
// segments were supplied.
func (p Subject) String() string {
	return p.joined
}

// Domain extends the subject with a domain segment and returns the resulting
// [Domain]. Domain panics if name is not a valid NATS token.
func (p Subject) Domain(name string) Domain {
	validateSegment(name)
	return Domain{prefix: p.joined, name: name}
}
