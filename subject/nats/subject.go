package nats

import "strings"

// Subject represents zero or more leading segments (e.g., env, tenant) prepended to all subjects.
type Subject struct {
	joined string
}

// New creates a reusable subject from zero or more segments.
// Zero segments are valid — Domain becomes the first token in the nats.
func New(segments ...string) Subject {
	for _, s := range segments {
		validateSegment(s)
	}

	return Subject{joined: strings.Join(segments, ".")}
}

// String returns the dot-joined prefix segments.
func (p Subject) String() string {
	return p.joined
}
