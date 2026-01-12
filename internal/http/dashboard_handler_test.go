package http

import (
	"testing"

	"fusionaly/internal/analytics"
	"fusionaly/internal/events"
)

func Test_convertReferrerStats(t *testing.T) {
	tests := []struct {
		name     string
		input    []analytics.MetricCountResult
		expected []analytics.MetricCountResult
	}{
		{
			name: "Convert direct or unknown referrer",
			input: []analytics.MetricCountResult{
				{Name: events.DirectOrUnknownReferrer, Count: 15},
				{Name: "Twitter", Count: 6},
				{Name: "tinylaun.ch", Count: 1},
			},
			expected: []analytics.MetricCountResult{
				{Name: "Direct / Unknown", Count: 15},
				{Name: "Twitter", Count: 6},
				{Name: "tinylaun.ch", Count: 1},
			},
		},
		{
			name:     "Empty input",
			input:    []analytics.MetricCountResult{},
			expected: []analytics.MetricCountResult{},
		},
		{
			name: "No conversion needed",
			input: []analytics.MetricCountResult{
				{Name: "google.com", Count: 10},
				{Name: "Twitter", Count: 5},
			},
			expected: []analytics.MetricCountResult{
				{Name: "google.com", Count: 10},
				{Name: "Twitter", Count: 5},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertReferrerStats(tt.input)

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
