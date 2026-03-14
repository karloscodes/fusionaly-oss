package settings

import (
	"log/slog"
	"testing"
	"time"

	"github.com/karloscodes/cartridge/cache"
	"github.com/stretchr/testify/assert"
)

func setupTestIPCache(ips []string) {
	excludedIPsCache = cache.NewCache[string, []string](slog.Default(), 5*time.Minute, func(key string) ([]string, error) {
		return ips, nil
	})
}

func TestIsIPExcluded(t *testing.T) {
	t.Run("matches single IP exactly", func(t *testing.T) {
		setupTestIPCache([]string{"192.168.1.1", "10.0.0.5"})

		excluded, err := IsIPExcluded("192.168.1.1")
		assert.NoError(t, err)
		assert.True(t, excluded)

		excluded, err = IsIPExcluded("192.168.1.2")
		assert.NoError(t, err)
		assert.False(t, excluded)
	})

	t.Run("matches IP within CIDR /24 range", func(t *testing.T) {
		setupTestIPCache([]string{"85.23.45.0/24"})

		excluded, err := IsIPExcluded("85.23.45.1")
		assert.NoError(t, err)
		assert.True(t, excluded)

		excluded, err = IsIPExcluded("85.23.45.255")
		assert.NoError(t, err)
		assert.True(t, excluded)

		excluded, err = IsIPExcluded("85.23.46.1")
		assert.NoError(t, err)
		assert.False(t, excluded)
	})

	t.Run("matches IP within CIDR /16 range", func(t *testing.T) {
		setupTestIPCache([]string{"192.168.0.0/16"})

		excluded, err := IsIPExcluded("192.168.50.100")
		assert.NoError(t, err)
		assert.True(t, excluded)

		excluded, err = IsIPExcluded("192.169.0.1")
		assert.NoError(t, err)
		assert.False(t, excluded)
	})

	t.Run("matches with mixed single IPs and CIDR ranges", func(t *testing.T) {
		setupTestIPCache([]string{"10.0.0.1", "192.168.0.0/16"})

		excluded, err := IsIPExcluded("10.0.0.1")
		assert.NoError(t, err)
		assert.True(t, excluded)

		excluded, err = IsIPExcluded("192.168.50.100")
		assert.NoError(t, err)
		assert.True(t, excluded)

		excluded, err = IsIPExcluded("172.16.0.1")
		assert.NoError(t, err)
		assert.False(t, excluded)
	})

	t.Run("skips empty entries", func(t *testing.T) {
		setupTestIPCache([]string{"", "10.0.0.1", ""})

		excluded, err := IsIPExcluded("10.0.0.1")
		assert.NoError(t, err)
		assert.True(t, excluded)
	})

	t.Run("skips invalid CIDR gracefully", func(t *testing.T) {
		setupTestIPCache([]string{"invalid/33", "10.0.0.1"})

		excluded, err := IsIPExcluded("10.0.0.1")
		assert.NoError(t, err)
		assert.True(t, excluded)
	})

	t.Run("returns false when cache is nil", func(t *testing.T) {
		excludedIPsCache = nil

		excluded, err := IsIPExcluded("10.0.0.1")
		assert.NoError(t, err)
		assert.False(t, excluded)
	})
}
