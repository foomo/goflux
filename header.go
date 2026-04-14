package goflux

// Header carries message metadata (trace context, message ID, custom KV).
// Keys are case-sensitive, values are slices — matching http.Header semantics.
type Header map[string][]string

// Get returns the first value for the given key, or "" if not present.
func (h Header) Get(key string) string {
	if values := h[key]; len(values) > 0 {
		return values[0]
	}

	return ""
}

// Set replaces any existing values for the given key.
func (h Header) Set(key, value string) {
	h[key] = []string{value}
}

// Add appends a value for the given key.
func (h Header) Add(key, value string) {
	h[key] = append(h[key], value)
}

// Del removes the given key.
func (h Header) Del(key string) {
	delete(h, key)
}

// Clone returns a deep copy of the header.
func (h Header) Clone() Header {
	if h == nil {
		return nil
	}

	out := make(Header, len(h))
	for k, vs := range h {
		out[k] = append([]string(nil), vs...)
	}

	return out
}
