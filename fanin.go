package goflux

import (
	"context"

	"github.com/foomo/gofuncy"
)

// fanInSubscriber merges multiple subscribers into one.
type fanInSubscriber[T any] struct {
	subscribers []Subscriber[T]
}

// FanIn returns a Subscriber[T] that subscribes to the same subject on all
// provided subscribers and dispatches every message to a single handler.
// Subscribe blocks until all inner subscriptions complete (i.e. all contexts
// are cancelled or all return an error).
//
// Close is a no-op — the caller owns the inner subscribers and is responsible
// for closing them.
func FanIn[T any](subscribers ...Subscriber[T]) Subscriber[T] {
	return &fanInSubscriber[T]{subscribers: subscribers}
}

func (f *fanInSubscriber[T]) Subscribe(ctx context.Context, subject string, handler Handler[T]) error {
	return gofuncy.All(ctx, f.subscribers, func(ctx context.Context, sub Subscriber[T]) error {
		return sub.Subscribe(ctx, subject, handler)
	}, gofuncy.WithName("goflux.fanin"))
}

func (f *fanInSubscriber[T]) Close() error { return nil }
