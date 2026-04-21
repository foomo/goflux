package goflux

// Topic bundles a Publisher and Subscriber sharing the same Codec. Use it
// when a service needs to both produce and consume the same message type.
type Topic[T any] struct {
	Publisher[T]
	Subscriber[T]
}

// BoundTopic bundles a BoundPublisher and BoundSubscriber sharing the same
// fixed nats. Use it when a service needs to both produce and consume the
// same message type on a known nats.
type BoundTopic[T any] struct {
	BoundPublisher[T]
	BoundSubscriber[T]
}

// BindTopic wraps a Publisher and Subscriber with a fixed nats.
func BindTopic[T any](pub Publisher[T], sub Subscriber[T], subject string) *BoundTopic[T] {
	return &BoundTopic[T]{
		BoundPublisher:  BindPublisher(pub, subject),
		BoundSubscriber: BindSubscriber(sub, subject),
	}
}
