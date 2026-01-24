package referrers

import "testing"

func TestFriendlyName(t *testing.T) {
	tests := []struct {
		hostname string
		expected string
	}{
		// Known referrers
		{"google.com", "Google"},
		{"news.ycombinator.com", "Hacker News"},
		{"x.com", "X/Twitter"},
		{"twitter.com", "X/Twitter"},
		{"reddit.com", "Reddit"},
		{"linkedin.com", "LinkedIn"},

		// With www prefix
		{"www.google.com", "Google"},
		{"www.reddit.com", "Reddit"},

		// Subdomains of known referrers
		{"m.facebook.com", "Facebook"},
		{"mobile.twitter.com", "X/Twitter"},

		// Unknown referrers (capitalized)
		{"example.com", "Example.com"},
		{"www.example.com", "Example.com"}, // www. stripped
		{"myblog.io", "Myblog.io"},

		// Case insensitive
		{"GOOGLE.COM", "Google"},
		{"News.Ycombinator.Com", "Hacker News"},
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			got := FriendlyName(tt.hostname)
			if got != tt.expected {
				t.Errorf("FriendlyName(%q) = %q, want %q", tt.hostname, got, tt.expected)
			}
		})
	}
}
