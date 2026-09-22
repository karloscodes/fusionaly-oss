package referrers

import "strings"

// Common referrer hostnames mapped to friendly display names
var knownReferrers = map[string]string{
	// Search engines
	"google.com":     "Google",
	"google.co.uk":   "Google",
	"google.de":      "Google",
	"google.fr":      "Google",
	"google.es":      "Google",
	"google.it":      "Google",
	"google.ca":      "Google",
	"google.com.au":  "Google",
	"google.co.jp":   "Google",
	"google.com.br":  "Google",
	"bing.com":       "Bing",
	"duckduckgo.com": "DuckDuckGo",
	"yahoo.com":      "Yahoo",
	"baidu.com":      "Baidu",
	"yandex.ru":      "Yandex",
	"ecosia.org":     "Ecosia",
	"kagi.com":       "Kagi",

	// Social media
	"x.com":           "X/Twitter",
	"twitter.com":     "X/Twitter",
	"t.co":            "X/Twitter",
	"facebook.com":    "Facebook",
	"fb.com":          "Facebook",
	"l.facebook.com":  "Facebook",
	"lm.facebook.com": "Facebook",
	"instagram.com":   "Instagram",
	"l.instagram.com": "Instagram",
	"linkedin.com":    "LinkedIn",
	"lnkd.in":         "LinkedIn",
	"tiktok.com":      "TikTok",
	"pinterest.com":   "Pinterest",
	"reddit.com":      "Reddit",
	"old.reddit.com":  "Reddit",
	"threads.net":     "Threads",
	"bsky.app":        "Bluesky",
	"mastodon.social": "Mastodon",
	"youtube.com":     "YouTube",
	"youtu.be":        "YouTube",
	"snapchat.com":    "Snapchat",
	"discord.com":     "Discord",
	"discordapp.com":  "Discord",
	"whatsapp.com":    "WhatsApp",
	"telegram.org":    "Telegram",
	"t.me":            "Telegram",
	"slack.com":       "Slack",

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
	"nytimes.com":        "NY Times",
	"washingtonpost.com": "Washington Post",
	"theguardian.com":    "The Guardian",
	"bbc.com":            "BBC",
	"bbc.co.uk":          "BBC",
	"cnn.com":            "CNN",
	"reuters.com":        "Reuters",
	"bloomberg.com":      "Bloomberg",
	"forbes.com":         "Forbes",
	"wsj.com":            "WSJ",
	"ft.com":             "Financial Times",

	// Email providers (for newsletter clicks)
	"mail.google.com":    "Gmail",
	"outlook.live.com":   "Outlook",
	"outlook.office.com": "Outlook",
	"mail.yahoo.com":     "Yahoo Mail",
	"protonmail.com":     "Proton Mail",
	"mail.proton.me":     "Proton Mail",

	// Mobile apps send their package name as the referrer
	"com.google.android.googlequicksearchbox": "Google",
	"com.google.android.gm":                   "Gmail",
	"com.google.android.youtube":              "YouTube",
	"com.facebook.katana":                     "Facebook",
	"com.facebook.facebook":                   "Facebook",
	"com.twitter.android":                     "X/Twitter",
	"com.linkedin.android":                    "LinkedIn",
	"com.reddit.frontpage":                    "Reddit",
	"com.reddit.redditswe":                    "Reddit",
	"com.instagram.android":                   "Instagram",
	"com.medium.reader":                       "Medium",
	"com.duckduckgo.mobile.android":           "DuckDuckGo",
	"com.zhiliaoapp.musically":                "TikTok",
	"com.whatsapp":                            "WhatsApp",
	"discord.gg":                              "Discord",

	// Link shorteners
	"bit.ly":      "Bitly",
	"tinyurl.com": "TinyURL",
	"goo.gl":      "Google Links",
	"ow.ly":       "Hootsuite",
}

// countryDomainBrands have a site per country (google.de, amazon.co.uk).
var countryDomainBrands = map[string]string{
	"google": "Google",
	"amazon": "Amazon",
}

// FriendlyName returns a human-friendly name for a referrer hostname, or the
// hostname itself (lowercase, without "www.") when it is not a known one.
func FriendlyName(hostname string) string {
	if name, ok := Lookup(hostname); ok {
		return name
	}
	return strings.TrimPrefix(strings.ToLower(hostname), "www.")
}

// Lookup returns the friendly name of a known referrer. It tries the full
// hostname, then each parent domain (a.m.youtube.com, m.youtube.com,
// youtube.com), so the longest known match wins and the result never
// depends on map order. It matches whole labels only: evilgoogle.com is not
// Google.
func Lookup(hostname string) (string, bool) {
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
	for host != "" {
		if name, ok := knownReferrers[host]; ok {
			return name, true
		}
		if name, ok := countryDomain(host); ok {
			return name, true
		}
		dot := strings.IndexByte(host, '.')
		if dot < 0 {
			break
		}
		host = host[dot+1:]
	}
	return "", false
}

// countryDomain matches google.de, google.co.in, amazon.com.br: a known brand
// followed by one or two short country labels.
func countryDomain(host string) (string, bool) {
	brand, rest, found := strings.Cut(host, ".")
	name, known := countryDomainBrands[brand]
	if !found || !known {
		return "", false
	}
	labels := strings.Split(rest, ".")
	if len(labels) > 2 {
		return "", false
	}
	for _, label := range labels {
		if label == "" || len(label) > 3 {
			return "", false
		}
	}
	return name, true
}
