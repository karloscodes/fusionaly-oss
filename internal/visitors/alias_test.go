package visitors_test

import (
	"testing"

	"fusionaly/internal/visitors"
	"github.com/stretchr/testify/assert"
)

func TestVisitorAlias(t *testing.T) {
	t.Run("Generates consistent alias for same signature", func(t *testing.T) {
		signature := "test-signature-123"

		alias1 := visitors.VisitorAlias(signature)
		alias2 := visitors.VisitorAlias(signature)

		assert.Equal(t, alias1, alias2, "Same signature should generate same alias")
		assert.NotEmpty(t, alias1, "Alias should not be empty")
	})

	t.Run("Generates different aliases for different signatures", func(t *testing.T) {
		alias1 := visitors.VisitorAlias("signature1")
		alias2 := visitors.VisitorAlias("signature2")
		alias3 := visitors.VisitorAlias("signature3")

		// While theoretically possible to have collisions, it should be very rare
		// with the number of combinations available
		assert.NotEqual(t, alias1, alias2, "Different signatures should likely generate different aliases")
		assert.NotEqual(t, alias2, alias3, "Different signatures should likely generate different aliases")
	})

	t.Run("Alias format is 'Adjective Animal'", func(t *testing.T) {
		signature := "test-signature"
		alias := visitors.VisitorAlias(signature)

		// Check that it contains a space (Adjective Animal format)
		assert.Contains(t, alias, " ", "Alias should contain a space between adjective and animal")

		// Check that it has two words
		assert.Regexp(t, `^[A-Z][a-z]+ [A-Z][a-z]+$`, alias, "Alias should be in 'Word Word' format")
	})

	t.Run("Generates valid aliases for various signatures", func(t *testing.T) {
		testSignatures := []string{
			"short",
			"a-very-long-signature-with-many-characters-to-test-hash-distribution",
			"123456789",
			"special!@#$%^&*()chars",
			"",
		}

		for _, sig := range testSignatures {
			alias := visitors.VisitorAlias(sig)
			assert.NotEmpty(t, alias, "Alias should not be empty for signature: %s", sig)
			assert.Contains(t, alias, " ", "Alias should contain space for signature: %s", sig)
		}
	})

	t.Run("Hash distribution across adjectives and animals", func(t *testing.T) {
		// Test that different signatures produce various combinations
		// This is a sanity check that the hash function is working
		aliases := make(map[string]bool)

		for i := 0; i < 1000; i++ {
			signature := string(rune(i))
			alias := visitors.VisitorAlias(signature)
			aliases[alias] = true
		}

		// With 1000 different signatures and the available combinations,
		// we should get multiple unique aliases (not all the same)
		assert.Greater(t, len(aliases), 100, "Should generate variety of aliases with different signatures")
	})
}
