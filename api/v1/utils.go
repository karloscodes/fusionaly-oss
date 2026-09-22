package v1

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/gofiber/fiber/v2"

	"fusionaly/internal/pkg/clientip"
)

// getClientIP returns the visitor's address. See clientip for why it is the
// rightmost X-Forwarded-For entry that no trusted proxy wrote.
func getClientIP(c *fiber.Ctx) string {
	return clientip.FromRequest(c)
}

// generateETag creates a strong ETag from content using SHA-256
func generateETag(content []byte) string {
	hash := sha256.Sum256(content)
	return `"` + hex.EncodeToString(hash[:]) + `"` // Quoted for strong ETag
}
