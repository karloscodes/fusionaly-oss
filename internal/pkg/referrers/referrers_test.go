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

		// Unknown referrers (kept lowercase)
		{"example.com", "example.com"},
		{"www.example.com", "example.com"}, // www. stripped
		{"myblog.io", "myblog.io"},

		// The longest known match wins, the same way every time
		{"x.mail.google.com", "Gmail"},
		{"producthunt.com", "Product Hunt"},
		{"old.reddit.com", "Reddit"},

		// Country domains and mobile apps
		{"google.co.in", "Google"},
		{"www.google.de", "Google"},
		{"amazon.co.uk", "Amazon"},
		{"google.evil.example.com", "google.evil.example.com"},
		{"com.google.android.youtube", "YouTube"},
		{"com.google.android.googlequicksearchbox", "Google"},
		{"discord.gg", "Discord"},

		// AI assistants
		{"chatgpt.com", "ChatGPT"},
		{"chat.openai.com", "ChatGPT"},
		{"www.perplexity.ai", "Perplexity"},
		{"claude.ai", "Claude"},
		{"gemini.google.com", "Gemini"},
		{"copilot.microsoft.com", "Copilot"},
		{"chat.deepseek.com", "DeepSeek"},

		// Newsletters
		{"beehiiv.com", "Beehiiv"},
		{"newsletter.beehiiv.com", "Beehiiv"},
		{"buttondown.com", "Buttondown"},
		{"us21.campaign-archive.com", "Mailchimp"},

		// More search and social
		{"search.brave.com", "Brave Search"},
		{"startpage.com", "Startpage"},
		{"news.google.com", "Google News"},

		// utm_source words name the same source as its hosts
		{"twitter", "X/Twitter"},
		{"fb", "Facebook"},
		{"linkedin", "LinkedIn"},
		{"hackernews", "Hacker News"},
		{"chatgpt", "ChatGPT"},
		{"newsletter", "newsletter"},
		{"example.hn", "example.hn"}, // a country domain, not Hacker News
		{"shop.x", "shop.x"},

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

func TestIsPaymentProvider(t *testing.T) {
	for _, host := range []string{"checkout.stripe.com", "buy.stripe.com", "www.paypal.com", "paypal.com", "checkout.paddle.com", "myshop.lemonsqueezy.com"} {
		if !IsPaymentProvider(host) {
			t.Errorf("IsPaymentProvider(%q) = false, want true", host)
		}
	}
	for _, host := range []string{"stripe.dev", "news.ycombinator.com", "notpaypal.com"} {
		if IsPaymentProvider(host) {
			t.Errorf("IsPaymentProvider(%q) = true, want false", host)
		}
	}
}

func TestCategoryOf(t *testing.T) {
	t.Run("with a known referrer", func(t *testing.T) {
		tests := map[string]Category{
			"www.google.com":         Search,
			"google.co.in":           Search,
			"duckduckgo.com":         Search,
			"chatgpt.com":            AI,
			"perplexity.ai":          AI,
			"t.co":                   Social,
			"m.facebook.com":         Social,
			"youtube.com":            Social,
			"mail.google.com":        Email,
			"newsletter.beehiiv.com": Email,
			"news.ycombinator.com":   Referral,
			"nytimes.com":            Referral,
			"bit.ly":                 Referral,
			"amazon.co.uk":           Referral,
			"com.google.android.gm":  Email,
			"com.reddit.frontpage":   Social,
			"twitter":                Social,
			"chatgpt":                AI,
		}

		for host, want := range tests {
			got := CategoryOf(host)

			if got != want {
				t.Errorf("CategoryOf(%q) = %q, want %q", host, got, want)
			}
		}
	})

	t.Run("with an unknown referrer", func(t *testing.T) {
		got := CategoryOf("myblog.io")

		if got != Referral {
			t.Errorf("CategoryOf(myblog.io) = %q, want %q", got, Referral)
		}
	})
}
