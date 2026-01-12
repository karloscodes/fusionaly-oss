package events

import "strings"

// IsSelfReferral checks if a hostname matches the website domain.
//
// Parameters:
//   - hostname: The referrer hostname to check (e.g., "www.example.com")
//   - websiteDomain: The website's domain (e.g., "example.com")
//
// Returns true if the hostname should be considered a self-referral.
// Only exact domain matches are considered self-referrals (privacy-first approach).
func IsSelfReferral(hostname, websiteDomain string) bool {
	if hostname == "" || websiteDomain == "" {
		return false
	}

	// Convert both to lowercase for comparison
	lowerHostname := strings.ToLower(hostname)
	lowerDomain := strings.ToLower(websiteDomain)

	// Direct match
	return lowerHostname == lowerDomain
}
