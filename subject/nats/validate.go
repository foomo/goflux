package nats

import (
	"fmt"
	"strings"
)

// validateSegment panics if s is not a valid NATS subject token. A token must
// be non-empty and free of ".", "*", ">", and whitespace.
func validateSegment(s string) {
	if s == "" {
		panic("nats: segment must not be empty")
	}

	if strings.ContainsAny(s, ".*> \t\n\r") {
		panic(fmt.Sprintf("nats: segment %q contains invalid character (one of: . * > or whitespace)", s))
	}
}
