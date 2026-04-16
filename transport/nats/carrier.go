package nats

import (
	"strings"

	"github.com/foomo/goflux"
)

const gofluxHeaderPrefix = "X-Goflux-"

// extractGofluxHeaders extracts X-Goflux-* headers into a goflux.Header,
// stripping the prefix. Returns nil if no goflux headers are present.
func extractGofluxHeaders(h map[string][]string) goflux.Header {
	var out goflux.Header

	for k, vs := range h {
		if strings.HasPrefix(k, gofluxHeaderPrefix) {
			if out == nil {
				out = make(goflux.Header)
			}

			out[strings.TrimPrefix(k, gofluxHeaderPrefix)] = append([]string(nil), vs...)
		}
	}

	return out
}

// natsHeaderCarrier implements propagation.TextMapCarrier for NATS message
// headers. We cannot use propagation.HeaderCarrier (which is http.Header)
// because it canonicalizes keys via textproto.CanonicalMIMEHeaderKey —
// e.g. "traceparent" becomes "Traceparent". NATS preserves raw key casing,
// so the W3C TraceContext propagator's lowercase keys don't survive the
// round-trip through http.Header.
type natsHeaderCarrier struct {
	Headers map[string][]string
}

func (c natsHeaderCarrier) Get(key string) string {
	values := c.Headers[key]
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

func (c natsHeaderCarrier) Set(key string, value string) {
	c.Headers[key] = []string{value}
}

func (c natsHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c.Headers))
	for k := range c.Headers {
		keys = append(keys, k)
	}

	return keys
}
