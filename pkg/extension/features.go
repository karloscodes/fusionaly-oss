// Package extension provides extension points for Fusionaly Pro features.
// The free (OSS) version uses this to check feature availability and show paywalls.
// The Pro version registers features at startup to enable them.
package extension

import (
	"sync"
)

// Feature flags for pro features
var (
	mu              sync.RWMutex
	lensEnabled     bool
	insightsEnabled bool
	aiDigestEnabled bool
	proVersion      bool
)

// EnableLens enables the Lens (saved queries) feature
func EnableLens() {
	mu.Lock()
	defer mu.Unlock()
	lensEnabled = true
}

// IsLensEnabled returns true if Lens feature is enabled
func IsLensEnabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return lensEnabled
}

// EnableInsights enables the AI Insights feature
func EnableInsights() {
	mu.Lock()
	defer mu.Unlock()
	insightsEnabled = true
}

// IsInsightsEnabled returns true if AI Insights feature is enabled
func IsInsightsEnabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return insightsEnabled
}

// EnableAIDigest enables the AI Weekly Digest feature
func EnableAIDigest() {
	mu.Lock()
	defer mu.Unlock()
	aiDigestEnabled = true
}

// IsAIDigestEnabled returns true if AI Weekly Digest is enabled
func IsAIDigestEnabled() bool {
	mu.RLock()
	defer mu.RUnlock()
	return aiDigestEnabled
}

// SetProVersion marks this as the Pro version
func SetProVersion() {
	mu.Lock()
	defer mu.Unlock()
	proVersion = true
}

// IsProVersion returns true if this is the Pro version
func IsProVersion() bool {
	mu.RLock()
	defer mu.RUnlock()
	return proVersion
}

// EnableAllProFeatures enables all pro features at once
func EnableAllProFeatures() {
	mu.Lock()
	defer mu.Unlock()
	lensEnabled = true
	insightsEnabled = true
	aiDigestEnabled = true
	proVersion = true
}
