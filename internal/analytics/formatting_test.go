package analytics

import (
	"testing"

	"fusionaly/internal/events"
)

func TestFormatReferrerStats(t *testing.T) {
	tests := []struct {
		name     string
		input    []MetricCountResult
		expected []MetricCountResult
	}{
		{
			name: "Convert direct or unknown referrer",
			input: []MetricCountResult{
				{Name: events.DirectOrUnknownReferrer, Count: 15},
				{Name: "Twitter", Count: 6},
				{Name: "tinylaun.ch", Count: 1},
			},
			expected: []MetricCountResult{
				{Name: "Direct / Unknown", Count: 15},
				{Name: "Twitter", Count: 6},
				{Name: "tinylaun.ch", Count: 1},
			},
		},
		{
			name:     "Empty input",
			input:    []MetricCountResult{},
			expected: []MetricCountResult{},
		},
		{
			name: "No conversion needed",
			input: []MetricCountResult{
				{Name: "google.com", Count: 10},
				{Name: "Twitter", Count: 5},
			},
			expected: []MetricCountResult{
				{Name: "google.com", Count: 10},
				{Name: "Twitter", Count: 5},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatReferrerStats(tt.input)

			if len(result) != len(tt.expected) {
				t.Errorf("Expected %d items, got %d", len(tt.expected), len(result))
				return
			}

			for i, item := range result {
				if item.Name != tt.expected[i].Name || item.Count != tt.expected[i].Count {
					t.Errorf("Expected %+v, got %+v", tt.expected[i], item)
				}
			}
		})
	}
}
