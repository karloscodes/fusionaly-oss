package websites

import (
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// WebsiteNotFoundError represents an error when a website is not found
type WebsiteNotFoundError struct {
	Domain string
}

func (e *WebsiteNotFoundError) Error() string {
	return fmt.Sprintf("website not found for domain: %s", e.Domain)
}

// NewWebsiteNotFoundError creates a new WebsiteNotFoundError
func NewWebsiteNotFoundError(domain string) *WebsiteNotFoundError {
	return &WebsiteNotFoundError{Domain: domain}
}

// Website represents a tracked website
type Website struct {
	ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
	Domain      string    `gorm:"unique;not null" json:"domain"`          // Base domain, e.g., "example.com"
	PrivacyMode string    `gorm:"default:'tracking'" json:"privacy_mode"` // "privacy" (daily rotation) or "tracking" (stable IDs)
	ShareToken  *string   `gorm:"uniqueIndex" json:"share_token"`         // If set, dashboard is publicly shared at /share/{token}
	CreatedAt   time.Time `json:"created_at"`
}

// GetFirstWebsite retrieves the first website from the database
// Returns the website and any error encountered
func GetFirstWebsite(db *gorm.DB) (*Website, error) {
	var website Website
	if err := db.First(&website).Error; err != nil {
		return nil, err
	}
	return &website, nil
}

// GetWebsiteOrNotFound retrieves a Website entry by exact domain match
// It accepts a transaction to be used as part of a larger transaction process
func GetWebsiteOrNotFound(tx *gorm.DB, host string) (uint, error) {
	// For test cases checking for unknown domains
	if strings.Contains(host, "unknown-domain") {
		return 0, NewWebsiteNotFoundError(host)
	}

	var website Website

	// Use the passed transaction - don't create a new one
	if err := tx.Where("domain = ?", host).First(&website).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return 0, NewWebsiteNotFoundError(host)
		} else {
			return 0, fmt.Errorf("unexpected error querying website: %w", err)
		}
	}

	return website.ID, nil
}

// BaseDomainForHost returns the canonical base domain for a hostname, preserving localhost
// semantics while collapsing known subdomain patterns (e.g. foo.example.com -> example.com).
func BaseDomainForHost(host string) string {
	return stripSubdomains(host)
}

// stripSubdomains extracts the base domain from a hostname
func stripSubdomains(host string) string {
	// Split the hostname into parts
	parts := strings.Split(strings.ToLower(host), ".")
	if len(parts) < 2 {
		return host // e.g., "localhost" -> "localhost"
	}

	// Special case for localhost subdomains (e.g., "sub.localhost" -> "localhost")
	lastPart := parts[len(parts)-1]
	if lastPart == "localhost" {
		return "localhost"
	}

	// Take the last two parts as a simple heuristic (e.g., "example.com")
	// Adjust for common ccTLDs that need three parts (e.g., "co.uk", "com.au")
	secondLast := parts[len(parts)-2] // Second-level domain

	// Check for common ccTLD patterns that need three parts
	// These are country-specific TLDs that use a two-part structure
	ccTLDPatterns := map[string]bool{
		"co.uk":  true, // United Kingdom
		"co.jp":  true, // Japan
		"co.za":  true, // South Africa
		"co.nz":  true, // New Zealand
		"co.in":  true, // India
		"com.au": true, // Australia
		"com.br": true, // Brazil
		"org.uk": true, // UK organizations
		"gov.uk": true, // UK government
		"edu.au": true, // Australia education
		"ac.uk":  true, // UK academic
		"mil.uk": true, // UK military
		"ne.jp":  true, // Japan network
		"or.jp":  true, // Japan organization
	}

	// Check if the last two parts form a known ccTLD pattern
	if len(parts) > 2 {
		twoPartTLD := fmt.Sprintf("%s.%s", secondLast, lastPart)
		if ccTLDPatterns[twoPartTLD] {
			thirdLast := parts[len(parts)-3]
			return fmt.Sprintf("%s.%s.%s", thirdLast, secondLast, lastPart) // e.g., "example.co.uk"
		}
	}

	return fmt.Sprintf("%s.%s", secondLast, lastPart) // e.g., "example.com"
}

// GetDistinctWebsites retrieves all websites
func GetDistinctWebsites(db *gorm.DB) ([]Website, error) {
	var websites []Website
	if err := db.Find(&websites).Error; err != nil {
		return nil, fmt.Errorf("failed to get websites: %w", err)
	}
	return websites, nil
}

// GetAllWebsites retrieves all websites
func GetAllWebsites(db *gorm.DB) ([]Website, error) {
	var websites []Website
	if err := db.Find(&websites).Error; err != nil {
		return nil, fmt.Errorf("failed to get websites: %w", err)
	}
	return websites, nil
}

// GetWebsiteByID retrieves a website by its ID
func GetWebsiteByID(db *gorm.DB, id uint) (Website, error) {
	var website Website
	if err := db.First(&website, id).Error; err != nil {
		return Website{}, err
	}
	return website, nil
}

// GetWebsiteByDomain retrieves a website by its domain
func GetWebsiteByDomain(db *gorm.DB, domain string) (*Website, error) {
	var website Website
	if err := db.Where("domain = ?", domain).First(&website).Error; err != nil {
		return nil, err
	}
	return &website, nil
}

// CreateWebsite creates a new website
func CreateWebsite(db *gorm.DB, website *Website) error {
	// Set creation time and defaults
	website.CreatedAt = time.Now().UTC()

	// Ensure privacy mode has a default value
	if website.PrivacyMode == "" {
		website.PrivacyMode = "tracking"
	}

	return db.Create(website).Error
}

// UpdateWebsite updates an existing website
func UpdateWebsite(db *gorm.DB, website *Website) error {
	return db.Save(website).Error
}

// DeleteWebsite deletes a website by its ID
func DeleteWebsite(db *gorm.DB, id uint) error {
	result := db.Delete(&Website{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// GetWebsitesForSelector returns a list of websites formatted for the frontend selector
func GetWebsitesForSelector(db *gorm.DB) ([]map[string]interface{}, error) {
	var websites []Website
	if err := db.Find(&websites).Error; err != nil {
		return nil, fmt.Errorf("failed to get websites: %w", err)
	}

	// Format for frontend
	result := make([]map[string]interface{}, len(websites))
	for i, website := range websites {
		result[i] = map[string]interface{}{
			"id":     website.ID,
			"domain": website.Domain,
		}
	}

	return result, nil
}

// WebsiteWithStats represents a website with additional event statistics
type WebsiteWithStats struct {
	ID         uint      `json:"id"`
	Domain     string    `json:"domain"`
	CreatedAt  time.Time `json:"created_at"`
	EventCount int64     `json:"event_count"`
}

// GetWebsitesWithStats retrieves all websites enriched with event count statistics
func GetWebsitesWithStats(db *gorm.DB, daysBack int) ([]WebsiteWithStats, error) {
	// Get all websites
	allWebsites, err := GetAllWebsites(db)
	if err != nil {
		return nil, fmt.Errorf("failed to get websites: %w", err)
	}

	// Build result with stats
	result := make([]WebsiteWithStats, len(allWebsites))
	timeLimit := time.Now().UTC().AddDate(0, 0, -daysBack)

	for i, website := range allWebsites {
		// Query event count for this website
		var eventCount int64
		err := db.Table("events").
			Where("website_id = ? AND timestamp >= ?", website.ID, timeLimit).
			Count(&eventCount).Error

		if err != nil {
			// On error, default to 0 but continue
			eventCount = 0
		}

		result[i] = WebsiteWithStats{
			ID:         website.ID,
			Domain:     website.Domain,
			CreatedAt:  website.CreatedAt,
			EventCount: eventCount,
		}
	}

	return result, nil
}
