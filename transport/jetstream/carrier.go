package jetstream

// jetstreamHeaderCarrier implements propagation.TextMapCarrier for NATS message
// headers. We cannot use propagation.HeaderCarrier (which is http.Header)
// because it canonicalizes keys via textproto.CanonicalMIMEHeaderKey —
// e.g. "traceparent" becomes "Traceparent". NATS preserves raw key casing,
// so the W3C TraceContext propagator's lowercase keys don't survive the
// round-trip through http.Header.
type jetstreamHeaderCarrier struct {
	Headers map[string][]string
}

func (c jetstreamHeaderCarrier) Get(key string) string {
	values := c.Headers[key]
	if len(values) == 0 {
		return ""
	}

	return values[0]
}

func (c jetstreamHeaderCarrier) Set(key string, value string) {
	c.Headers[key] = []string{value}
}

func (c jetstreamHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c.Headers))
	for k := range c.Headers {
		keys = append(keys, k)
	}

	return keys
}
