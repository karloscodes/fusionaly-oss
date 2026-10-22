package visitors

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// BuildUniqueVisitorId creates a privacy-first unique visitor identifier.
// Pass the DailySalt: it changes at midnight UTC, so visitors cannot be
// tracked across days. IP addresses are never stored - only used in hashing.
func BuildUniqueVisitorId(website, ipAddress, userAgent, salt string) string {
	data := fmt.Sprintf("%s.%s.%s.%s", salt, website, ipAddress, userAgent)

	// Create a SHA-256 hash (IP address is never stored, only hashed)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
