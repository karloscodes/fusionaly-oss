package v1

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeIPVariants(t *testing.T) {
	t.Helper()

	tests := []struct {
		name string
		raw  string
		want string
	}{
		{name: "plain ipv4", raw: "79.144.65.173", want: "79.144.65.173"},
		{name: "ipv4 with spaces", raw: " 79.144.65.173 ", want: "79.144.65.173"},
		{name: "quoted ipv4", raw: "\"79.144.65.173\"", want: "79.144.65.173"},
		{name: "ipv4 with port", raw: "79.144.65.173:443", want: "79.144.65.173"},
		{name: "quoted forwarded ipv4", raw: "\"79.144.65.173:1234\"", want: "79.144.65.173"},
		{name: "ipv6 literal", raw: "2001:db8::1", want: "2001:db8::1"},
		{name: "ipv6 in brackets", raw: "[2001:db8::1]", want: "2001:db8::1"},
		{name: "ipv6 with port", raw: "[2001:db8::1]:8443", want: "2001:db8::1"},
		{name: "ipv6 with zone", raw: "fe80::1%eth0", want: "fe80::1"},
		{name: "ipv4 mapped ipv6", raw: "::ffff:203.0.113.9", want: "203.0.113.9"},
		{name: "invalid value", raw: "not-an-ip", want: ""},
		{name: "empty", raw: "   ", want: ""},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got, parsed := normalizeIP(tc.raw)
			assert.Equal(t, tc.want, got)

			if tc.want == "" {
				assert.Nil(t, parsed)
				return
			}

			require.NotNil(t, parsed)
			assert.Equal(t, tc.want, parsed.String())
		})
	}
}

func TestSelectPreferredIP(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   string
	}{
		{
			name:   "prefers public ipv4 over ipv6",
			values: []string{"2001:db8::1", "203.0.113.20"},
			want:   "203.0.113.20",
		},
		{
			name:   "skips private addresses",
			values: []string{"192.168.1.10", "10.0.0.5", "::1", "198.51.100.7"},
			want:   "198.51.100.7",
		},
		{
			name:   "returns ipv6 fallback when no ipv4",
			values: []string{"2001:db8::2"},
			want:   "2001:db8::2",
		},
		{
			name:   "returns empty when no valid candidates",
			values: []string{"", "   ", "not-an-ip"},
			want:   "",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, selectPreferredIP(tc.values))
		})
	}
}

func TestIsPrivateIPWithMappedIPv4(t *testing.T) {
	private := net.ParseIP("::ffff:192.168.1.5")
	require.NotNil(t, private)
	assert.True(t, isPrivateIP(private))

	public := net.ParseIP("::ffff:8.8.8.8")
	require.NotNil(t, public)
	assert.False(t, isPrivateIP(public))
}
