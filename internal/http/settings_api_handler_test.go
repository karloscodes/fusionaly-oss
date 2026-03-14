package http

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateIPList(t *testing.T) {
	t.Run("accepts empty string", func(t *testing.T) {
		valid, msg := validateIPList("")
		assert.True(t, valid)
		assert.Empty(t, msg)
	})

	t.Run("accepts single IP", func(t *testing.T) {
		valid, msg := validateIPList("192.168.1.1")
		assert.True(t, valid)
		assert.Empty(t, msg)
	})

	t.Run("accepts multiple IPs", func(t *testing.T) {
		valid, msg := validateIPList("192.168.1.1, 10.0.0.1, 172.16.0.1")
		assert.True(t, valid)
		assert.Empty(t, msg)
	})

	t.Run("accepts CIDR /24 range", func(t *testing.T) {
		valid, msg := validateIPList("85.23.45.0/24")
		assert.True(t, valid)
		assert.Empty(t, msg)
	})

	t.Run("accepts CIDR /16 range", func(t *testing.T) {
		valid, msg := validateIPList("10.0.0.0/8")
		assert.True(t, valid)
		assert.Empty(t, msg)
	})

	t.Run("accepts mixed IPs and CIDR ranges", func(t *testing.T) {
		valid, msg := validateIPList("192.168.1.1, 85.23.45.0/24, 10.0.0.5")
		assert.True(t, valid)
		assert.Empty(t, msg)
	})

	t.Run("rejects invalid IP", func(t *testing.T) {
		valid, msg := validateIPList("not-an-ip")
		assert.False(t, valid)
		assert.Contains(t, msg, "Invalid IP address format")
	})

	t.Run("rejects invalid CIDR mask", func(t *testing.T) {
		valid, msg := validateIPList("10.0.0.0/33")
		assert.False(t, valid)
		assert.Contains(t, msg, "Invalid IP range format")
	})

	t.Run("rejects garbage CIDR", func(t *testing.T) {
		valid, msg := validateIPList("foo/24")
		assert.False(t, valid)
		assert.Contains(t, msg, "Invalid IP range format")
	})

	t.Run("skips empty entries between commas", func(t *testing.T) {
		valid, msg := validateIPList("192.168.1.1,, 10.0.0.1")
		assert.True(t, valid)
		assert.Empty(t, msg)
	})
}
