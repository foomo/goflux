package goflux_test

import (
	"encoding/json"
	"testing"

	"github.com/foomo/goflux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessage_UnmarshalJSON(t *testing.T) {
	t.Parallel()

	for name, tc := range map[string]struct {
		in   string
		want string
	}{
		"subject key":     {in: `{"subject":"orders","payload":"p"}`, want: "orders"},
		"legacy nats key": {in: `{"nats":"orders","payload":"p"}`, want: "orders"},
		"subject wins":    {in: `{"subject":"orders","nats":"old","payload":"p"}`, want: "orders"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var msg goflux.Message[string]
			require.NoError(t, json.Unmarshal([]byte(tc.in), &msg))
			assert.Equal(t, tc.want, msg.Subject)
			assert.Equal(t, "p", msg.Payload)
		})
	}
}

func TestMessage_JSONRoundTrip(t *testing.T) {
	t.Parallel()

	in := goflux.NewMessageWithHeader("orders", "p", goflux.Header{"X-Tenant": {"acme"}})
	in.ID = "id-1"

	b, err := json.Marshal(in)
	require.NoError(t, err)
	assert.JSONEq(t, `{"subject":"orders","payload":"p","header":{"X-Tenant":["acme"]},"id":"id-1"}`, string(b))

	var out goflux.Message[string]
	require.NoError(t, json.Unmarshal(b, &out))
	assert.Equal(t, in, out)
}
