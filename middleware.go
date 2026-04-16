package goflux

// Middleware wraps a Handler[T] to add cross-cutting behaviour such as
// logging, rate-limiting, or circuit-breaking.
type Middleware[T any] func(Handler[T]) Handler[T]

// PublisherMiddleware wraps a Publisher[T] to add cross-cutting behaviour
// such as retry, rate-limiting, or circuit-breaking on the publish path.
type PublisherMiddleware[T any] func(Publisher[T]) Publisher[T]

// Chain composes middlewares left-to-right: the first middleware in the list
// is the outermost wrapper. Chain(a, b)(h) is equivalent to a(b(h)).
func Chain[T any](mws ...Middleware[T]) Middleware[T] {
	return func(next Handler[T]) Handler[T] {
		for i := len(mws) - 1; i >= 0; i-- {
			next = mws[i](next)
		}

		return next
	}
}
