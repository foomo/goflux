package goflux

import "strings"

// Header carries message metadata (trace context, message ID, custom KV).
// Values are slices like in http.Header, but keys are stored as given and
// not canonicalized. Only [Header.Get] matches keys case-insensitively.
type Header map[string][]string

// Get returns the first value for the given key, or "" if not present. An
// exact key match wins; otherwise the key is matched case-insensitively,
// because some transports change key case (HTTP canonicalizes "X-Tenant-ID"
// to "X-Tenant-Id", HTTP/2 lowercases it). If several keys differ only in
// case, which one the fallback picks is unspecified.
func (h Header) Get(key string) string {
	if values := h[key]; len(values) > 0 {
		return values[0]
	}

	for k, values := range h {
		if len(values) > 0 && strings.EqualFold(k, key) {
			return values[0]
		}
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
