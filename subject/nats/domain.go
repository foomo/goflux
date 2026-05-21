package nats

// Domain is the domain segment of a NATS subject, optionally preceded by a
// [Subject] prefix.
type Domain struct {
	prefix string
	name   string
}

// NewDomain returns a [Domain] without a prefix. It is a shortcut for
// [New]().[Subject.Domain](name).
//
// NewDomain panics if name is not a valid NATS token.
func NewDomain(name string) Domain {
	validateSegment(name)
	return Domain{name: name}
}

// String returns the subject up to and including the domain segment.
func (d Domain) String() string {
	if d.prefix == "" {
		return d.name
	}

	return d.prefix + "." + d.name
}

// All returns the NATS multi-token wildcard subject that matches every
// subject nested below this domain (the domain path followed by ".>").
func (d Domain) All() string {
	return d.String() + ".>"
}

// Entity extends the domain with an entity segment and returns the resulting
// [Entity]. Entity panics if name is not a valid NATS token.
func (d Domain) Entity(name string) Entity {
	validateSegment(name)
	return Entity{domain: d.String(), name: name}
}
