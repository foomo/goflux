// Package pipe provides handler factories that compose [goflux.Handler] and
// [goflux.Publisher] into pipeline stages: forwarding, mapping, and
// flat-mapping messages from one subject onto another, with optional
// filtering, dead-lettering, and middleware.
package pipe
