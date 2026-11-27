package goflux_test

import (
	"testing"

	"github.com/foomo/goflux"
)

func TestHeader_Get(t *testing.T) {
	t.Parallel()

	h := goflux.Header{"X-Tenant-Id": {"acme"}, "X-Exact": {"lower"}, "x-exact": {"other"}}

	tests := map[string]string{
		"X-Tenant-Id": "acme",  // exact
		"X-Tenant-ID": "acme",  // HTTP canonicalized the key
		"x-tenant-id": "acme",  // HTTP/2 lowercased the key
		"X-Exact":     "lower", // exact match wins over case-insensitive
		"x-exact":     "other",
		"X-Missing":   "",
	}

	for key, want := range tests {
		if got := h.Get(key); got != want {
			t.Errorf("Get(%q) = %q, want %q", key, got, want)
		}
	}
}
