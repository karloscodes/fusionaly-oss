package referrers

import "strings"

// Common referrer hostnames mapped to friendly display names
var knownReferrers = map[string]string{
	// Search engines
	"google.com":       "Google",
	"google.co.uk":     "Google",
	"google.de":        "Google",
	"google.fr":        "Google",
	"google.es":        "Google",
	"google.it":        "Google",
	"google.ca":        "Google",
	"google.com.au":    "Google",
	"google.co.jp":     "Google",
	"google.com.br":    "Google",
	"bing.com":         "Bing",
	"duckduckgo.com":   "DuckDuckGo",
	"yahoo.com":        "Yahoo",
	"baidu.com":        "Baidu",
	"yandex.ru":        "Yandex",
	"ecosia.org":       "Ecosia",
	"kagi.com":         "Kagi",

	// Social media
	"x.com":            "X/Twitter",
	"twitter.com":      "X/Twitter",
	"t.co":             "X/Twitter",
	"facebook.com":     "Facebook",
	"fb.com":           "Facebook",
	"l.facebook.com":   "Facebook",
	"lm.facebook.com":  "Facebook",
	"instagram.com":    "Instagram",
	"l.instagram.com":  "Instagram",
	"linkedin.com":     "LinkedIn",
	"lnkd.in":          "LinkedIn",
	"tiktok.com":       "TikTok",
	"pinterest.com":    "Pinterest",
	"reddit.com":       "Reddit",
	"old.reddit.com":   "Reddit",
	"threads.net":      "Threads",
	"bsky.app":         "Bluesky",
	"mastodon.social":  "Mastodon",
	"youtube.com":      "YouTube",
	"youtu.be":         "YouTube",
	"snapchat.com":     "Snapchat",
	"discord.com":      "Discord",
	"discordapp.com":   "Discord",
	"whatsapp.com":     "WhatsApp",
	"telegram.org":     "Telegram",
	"t.me":             "Telegram",
	"slack.com":        "Slack",

	// Tech communities
	"news.ycombinator.com": "Hacker News",
	"hn.algolia.com":       "Hacker News",
	"lobste.rs":            "Lobsters",
	"producthunt.com":      "Product Hunt",
	"indiehackers.com":     "Indie Hackers",
	"dev.to":               "DEV Community",
	"hashnode.com":         "Hashnode",
	"medium.com":           "Medium",
	"substack.com":         "Substack",
	"hackernoon.com":       "HackerNoon",
	"slashdot.org":         "Slashdot",
	"techcrunch.com":       "TechCrunch",
	"theverge.com":         "The Verge",
	"arstechnica.com":      "Ars Technica",
	"wired.com":            "Wired",
	"github.com":           "GitHub",
	"gitlab.com":           "GitLab",
	"stackoverflow.com":    "Stack Overflow",
	"quora.com":            "Quora",

	// News
	"nytimes.com":       "NY Times",
	"washingtonpost.com": "Washington Post",
	"theguardian.com":   "The Guardian",
	"bbc.com":           "BBC",
	"bbc.co.uk":         "BBC",
	"cnn.com":           "CNN",
	"reuters.com":       "Reuters",
	"bloomberg.com":     "Bloomberg",
	"forbes.com":        "Forbes",
	"wsj.com":           "WSJ",
	"ft.com":            "Financial Times",

	// Email providers (for newsletter clicks)
	"mail.google.com":    "Gmail",
	"outlook.live.com":   "Outlook",
	"outlook.office.com": "Outlook",
	"mail.yahoo.com":     "Yahoo Mail",
	"protonmail.com":     "Proton Mail",
	"mail.proton.me":     "Proton Mail",

	// Link shorteners
	"bit.ly":      "Bitly",
	"tinyurl.com": "TinyURL",
	"goo.gl":      "Google Links",
	"ow.ly":       "Hootsuite",
}

// FriendlyName returns a human-friendly name for a referrer hostname.
// If the hostname is not in the known list, it returns the hostname
// with common prefixes like "www." removed and first letter capitalized.
func FriendlyName(hostname string) string {
	hostname = strings.ToLower(hostname)

	// Check exact match first
	if name, ok := knownReferrers[hostname]; ok {
		return name
	}

	// Try without www. prefix
	if strings.HasPrefix(hostname, "www.") {
		withoutWWW := hostname[4:]
		if name, ok := knownReferrers[withoutWWW]; ok {
			return name
		}
		hostname = withoutWWW
	}

	// Check if it's a subdomain of a known referrer
	for domain, name := range knownReferrers {
		if strings.HasSuffix(hostname, "."+domain) {
			return name
		}
	}

	// Capitalize first letter for unknown hostnames
	return capitalizeFirst(hostname)
}

// capitalizeFirst capitalizes the first letter of a string
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}
