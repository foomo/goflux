package channel

import (
	semconvmsg "go.opentelemetry.io/otel/semconv/v1.41.0/messagingconv"
)

// system is the goflux.system attribute value for this transport.
var system = semconvmsg.SystemAttr("go_channel")
