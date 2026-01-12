package visitors

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"
)

// BuildUniqueVisitorId creates a privacy-first unique visitor identifier.
// The signature rotates daily at midnight UTC, ensuring visitors cannot be
// tracked across days. IP addresses are never stored - only used in hashing.
func BuildUniqueVisitorId(website, ipAddress, userAgent, salt string) string {
	// Daily rotating signature - visitors reset at midnight UTC
	today := time.Now().UTC().Format("2006-01-02")
	dailySalt := fmt.Sprintf("%s-%s", today, salt)
	data := fmt.Sprintf("%s.%s.%s.%s", dailySalt, website, ipAddress, userAgent)

	// Create a SHA-256 hash (IP address is never stored, only hashed)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
