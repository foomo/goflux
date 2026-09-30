// Package http implements the goflux [goflux.Publisher] and
// [goflux.Subscriber] interfaces over plain HTTP POST requests. The
// publisher posts encoded messages to {baseURL}/{subject}; the subscriber
// exposes an *http.ServeMux that the caller registers with its own
// http.Server, since this package does not own a listener.
package http
