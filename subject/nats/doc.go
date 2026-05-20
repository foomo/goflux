// Package nats builds NATS subject strings using a fluent, type-safe chain.
//
// A NATS subject is a dot-separated path such as "prod.acme.user.profile.updated".
// This package exposes a small builder that composes such paths from named
// segments while enforcing valid token syntax at construction time.
//
// # Chain
//
// The builder has four stages, each represented by its own type:
//
//	[Subject] -> [Domain] -> [Entity] -> [Event]
//
// A [Subject] holds zero or more leading segments (for example an environment
// or tenant prefix). [Subject.Domain] extends it to a [Domain]. [Domain.Entity]
// extends it to an [Entity]. [Entity.Event] terminates the chain with an [Event].
// Every stage exposes String to render the path built so far.
//
// # Wildcards
//
// [Domain.All] and [Entity.All] append the NATS multi-token wildcard ">",
// producing patterns suitable for subscriptions that match every subject
// nested below that level.
//
// # Validation
//
// Each segment is validated when it is added. An empty segment, or one that
// contains ".", "*", ">", or any whitespace, causes a panic. Subjects are
// expected to be assembled at startup from constants, so panicking surfaces
// programmer errors immediately.
//
// # Example
//
//	subject := nats.New("prod", "acme").
//		Domain("user").
//		Entity("profile").
//		Event("updated").
//		String()
//	// subject == "prod.acme.user.profile.updated"
//
//	wildcard := nats.NewDomain("user").All()
//	// wildcard == "user.>"
package nats
