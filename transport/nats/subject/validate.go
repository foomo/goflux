package subject

import (
	"fmt"
	"strings"
)

// validateSegment panics if the segment is not a valid NATS subject token.
func validateSegment(s string) {
	if s == "" {
		panic("subject: segment must not be empty")
	}

	if strings.ContainsAny(s, ".*> \t\n\r") {
		panic(fmt.Sprintf("subject: segment %q contains invalid character (one of: . * > or whitespace)", s))
	}
}
