package nats

// Domain creates a Domain from this prefix.
func (p Subject) Domain(name string) Domain {
	validateSegment(name)
	return Domain{prefix: p.joined, name: name}
}

// Domain represents the domain segment of a nats.
type Domain struct {
	prefix string
	name   string
}

// NewDomain creates a Domain directly when no prefix is needed.
func NewDomain(name string) Domain {
	validateSegment(name)
	return Domain{name: name}
}

// String returns the full nats up to and including the domain.
func (d Domain) String() string {
	if d.prefix == "" {
		return d.name
	}

	return d.prefix + "." + d.name
}

// All returns a NATS ">" wildcard nats matching everything under this domain.
func (d Domain) All() string {
	return d.String() + ".>"
}

// Entity creates an Entity from this domain.
func (d Domain) Entity(name string) Entity {
	validateSegment(name)
	return Entity{domain: d.String(), name: name}
}
