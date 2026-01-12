package visitors_test

import (
	"testing"
	"time"

	"fusionaly/internal/visitors"
	"github.com/stretchr/testify/assert"
)

func TestBuildUniqueVisitorId(t *testing.T) {
	website := "example.com"
	ipAddress := "192.168.1.1"
	userAgent := "Mozilla/5.0"
	salt := "test-salt"

	t.Run("generates consistent ID for same inputs within same day", func(t *testing.T) {
		id1 := visitors.BuildUniqueVisitorId(website, ipAddress, userAgent, salt)
		id2 := visitors.BuildUniqueVisitorId(website, ipAddress, userAgent, salt)

		assert.Equal(t, id1, id2, "Same inputs should generate same ID")
		assert.NotEmpty(t, id1, "ID should not be empty")
		assert.Len(t, id1, 64, "SHA-256 hash should be 64 characters (hex encoded)")
	})

	t.Run("generates different IDs for different IPs", func(t *testing.T) {
		id1 := visitors.BuildUniqueVisitorId(website, ipAddress, userAgent, salt)
		id2 := visitors.BuildUniqueVisitorId(website, "192.168.1.2", userAgent, salt)

		assert.NotEqual(t, id1, id2, "Different IP should generate different ID")
	})

	t.Run("generates different IDs for different user agents", func(t *testing.T) {
		id1 := visitors.BuildUniqueVisitorId(website, ipAddress, userAgent, salt)
		id2 := visitors.BuildUniqueVisitorId(website, ipAddress, "Different Agent", salt)

		assert.NotEqual(t, id1, id2, "Different user agent should generate different ID")
	})

	t.Run("generates different IDs for different websites", func(t *testing.T) {
		id1 := visitors.BuildUniqueVisitorId(website, ipAddress, userAgent, salt)
		id2 := visitors.BuildUniqueVisitorId("different.com", ipAddress, userAgent, salt)

		assert.NotEqual(t, id1, id2, "Different website should generate different ID")
	})

	t.Run("generates different IDs for different salts", func(t *testing.T) {
		id1 := visitors.BuildUniqueVisitorId(website, ipAddress, userAgent, "salt1")
		id2 := visitors.BuildUniqueVisitorId(website, ipAddress, userAgent, "salt2")

		assert.NotEqual(t, id1, id2, "Different salts should generate different IDs")
	})

	t.Run("IDs are stable within the same UTC day", func(t *testing.T) {
		// Generate ID twice with small delay - should be same within same day
		id1 := visitors.BuildUniqueVisitorId(website, ipAddress, userAgent, salt)
		time.Sleep(10 * time.Millisecond)
		id2 := visitors.BuildUniqueVisitorId(website, ipAddress, userAgent, salt)

		assert.Equal(t, id1, id2, "IDs should be same within the same UTC day")
		assert.NotEmpty(t, id1, "ID should not be empty")
	})
}
