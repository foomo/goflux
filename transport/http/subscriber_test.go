package http_test

import (
	"github.com/foomo/goflux"
	"github.com/foomo/goflux/transport/http"
)

// Compile-time check: the subscriber supports readiness signalling.
var _ goflux.ReadySubscriber[Event] = (*http.Subscriber[Event])(nil)
