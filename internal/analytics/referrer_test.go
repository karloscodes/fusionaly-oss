package analytics_test

import (
	"fusionaly/internal/analytics"
	"testing"

	"fusionaly/internal/events"
)

func TestNormalizeReferrerHostname(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		// Direct traffic
		{"", "Direct / Unknown"},
		{"direct / unknown", "Direct / Unknown"},
		{"(direct)", "Direct / Unknown"},
		{"unknown", "Direct / Unknown"},

		// Known services
		{"www.google.com", "Google"},
		{"google.com", "Google"},
		{"m.facebook.com", "Facebook"},
		{"facebook.com", "Facebook"},
		{"github.com", "GitHub"},
		{"stackoverflow.com", "Stack Overflow"},
		{"youtu.be", "YouTube"},
		{"t.co", "Twitter"},

		// Unknown hostnames should be cleaned
		{"www.example.com", "example.com"},
		{"m.example.com", "example.com"},
		{"mobile.example.com", "example.com"},
		{"amp.example.com", "example.com"},
		{"l.example.com", "example.com"},

		// Should handle case insensitivity
		{"WWW.GOOGLE.COM", "Google"},
		{"EXAMPLE.COM", "example.com"},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := analytics.NormalizeReferrerHostname(test.input)
			if result != test.expected {
				t.Errorf("analytics.NormalizeReferrerHostname(%q) = %q, expected %q", test.input, result, test.expected)
			}
		})
	}
}

func TestIsSelfReferral(t *testing.T) {
	tests := []struct {
		hostname      string
		websiteDomain string
		expected      bool
	}{
		// Direct matches
		{"example.com", "example.com", true},
		{"", "", false},
		{"example.com", "sub.example.com", false},
	}

	for _, test := range tests {
		t.Run(test.hostname+"_vs_"+test.websiteDomain, func(t *testing.T) {
			// Test with subdomain tracking enabled (default behavior)
			result := events.IsSelfReferral(test.hostname, test.websiteDomain)
			if result != test.expected {
				t.Errorf("IsSelfReferral(%q, %q, true) = %v, expected %v", test.hostname, test.websiteDomain, result, test.expected)
			}
		})
	}
}
