// Package channel implements the goflux [goflux.Publisher] and
// [goflux.Subscriber] interfaces over in-process Go channels. It requires no
// codec, and backpressure is applied by blocking rather than dropping or
// erroring.
package channel
