package config

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidatePrivateKey(t *testing.T) {
	valid := func(key string) *Config {
		return &Config{Environment: Production, DatabaseType: SQLiteDatabase, PrivateKey: key}
	}

	t.Run("accepts a key of 32 characters or more", func(t *testing.T) {
		err := valid(strings.Repeat("a", 32)).validate()

		assert.NoError(t, err)
	})

	t.Run("rejects a shorter key with a clear message", func(t *testing.T) {
		err := valid("short").validate()

		assert.ErrorContains(t, err, "FUSIONALY_PRIVATE_KEY must have at least 32 characters")
	})
}
