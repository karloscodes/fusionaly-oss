package analytics_test

import (
	"fusionaly/internal/analytics"
	"fusionaly/internal/events"
	"testing"
)

func TestNormalizeReferrerHostnameDirectOrUnknown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "DirectOrUnknownReferrer constant",
			input:    events.DirectOrUnknownReferrer,
			expected: "Direct / Unknown",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "Direct / Unknown",
		},
		{
			name:     "Direct pattern",
			input:    "(direct)",
			expected: "Direct / Unknown",
		},
		{
			name:     "Unknown pattern",
			input:    "unknown",
			expected: "Direct / Unknown",
		},
		{
			name:     "Regular domain",
			input:    "google.com",
			expected: "Google",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := analytics.NormalizeReferrerHostname(tt.input)
			if result != tt.expected {
				t.Errorf("analytics.NormalizeReferrerHostname(%q) = %q, expected %q", tt.input, result, tt.expected)
			}
		})
	}
}
