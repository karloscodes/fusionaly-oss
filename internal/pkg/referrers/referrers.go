package referrers

import "strings"

// Category is the kind of site a referrer is. The dashboard groups sources
// into channels by it.
type Category string

const (
	Search   Category = "Search"
	Social   Category = "Social"
	AI       Category = "AI"
	Email    Category = "Email"
	Referral Category = "Referral" // communities, news, blogs, and unknown sites
)

type referrer struct {
	Name     string
	Category Category
}

// Common referrer hostnames mapped to friendly display names and categories
var knownReferrers = map[string]referrer{
	// Search engines
	"google.com":       {"Google", Search},
	"google.co.uk":     {"Google", Search},
	"google.de":        {"Google", Search},
	"google.fr":        {"Google", Search},
	"google.es":        {"Google", Search},
	"google.it":        {"Google", Search},
	"google.ca":        {"Google", Search},
	"google.com.au":    {"Google", Search},
	"google.co.jp":     {"Google", Search},
	"google.com.br":    {"Google", Search},
	"bing.com":         {"Bing", Search},
	"duckduckgo.com":   {"DuckDuckGo", Search},
	"yahoo.com":        {"Yahoo", Search},
	"baidu.com":        {"Baidu", Search},
	"yandex.ru":        {"Yandex", Search},
	"ecosia.org":       {"Ecosia", Search},
	"kagi.com":         {"Kagi", Search},
	"search.brave.com": {"Brave Search", Search},
	"startpage.com":    {"Startpage", Search},
	"qwant.com":        {"Qwant", Search},
	"yandex.com":       {"Yandex", Search},
	"naver.com":        {"Naver", Search},
	"seznam.cz":        {"Seznam", Search},
	"news.google.com":  {"Google News", Referral},

	// AI assistants
	"chatgpt.com":           {"ChatGPT", AI},
	"chat.openai.com":       {"ChatGPT", AI},
	"perplexity.ai":         {"Perplexity", AI},
	"claude.ai":             {"Claude", AI},
	"gemini.google.com":     {"Gemini", AI},
	"bard.google.com":       {"Gemini", AI},
	"copilot.microsoft.com": {"Copilot", AI},
	"chat.deepseek.com":     {"DeepSeek", AI},
	"grok.com":              {"Grok", AI},
	"meta.ai":               {"Meta AI", AI},
	"chat.mistral.ai":       {"Mistral", AI},
	"you.com":               {"You.com", AI},
	"phind.com":             {"Phind", AI},
	"poe.com":               {"Poe", AI},

	// Social media
	"x.com":           {"X/Twitter", Social},
	"twitter.com":     {"X/Twitter", Social},
	"t.co":            {"X/Twitter", Social},
	"facebook.com":    {"Facebook", Social},
	"fb.com":          {"Facebook", Social},
	"l.facebook.com":  {"Facebook", Social},
	"lm.facebook.com": {"Facebook", Social},
	"instagram.com":   {"Instagram", Social},
	"l.instagram.com": {"Instagram", Social},
	"linkedin.com":    {"LinkedIn", Social},
	"lnkd.in":         {"LinkedIn", Social},
	"tiktok.com":      {"TikTok", Social},
	"pinterest.com":   {"Pinterest", Social},
	"reddit.com":      {"Reddit", Social},
	"old.reddit.com":  {"Reddit", Social},
	"threads.net":     {"Threads", Social},
	"bsky.app":        {"Bluesky", Social},
	"mastodon.social": {"Mastodon", Social},
	"youtube.com":     {"YouTube", Social},
	"youtu.be":        {"YouTube", Social},
	"snapchat.com":    {"Snapchat", Social},
	"discord.com":     {"Discord", Social},
	"discordapp.com":  {"Discord", Social},
	"whatsapp.com":    {"WhatsApp", Social},
	"telegram.org":    {"Telegram", Social},
	"t.me":            {"Telegram", Social},
	"slack.com":       {"Slack", Social},
	"tumblr.com":      {"Tumblr", Social},
	"vk.com":          {"VK", Social},
	"weibo.com":       {"Weibo", Social},
	"xing.com":        {"XING", Social},
	"flipboard.com":   {"Flipboard", Social},

	// Tech communities
	"news.ycombinator.com": {"Hacker News", Referral},
	"hn.algolia.com":       {"Hacker News", Referral},
	"lobste.rs":            {"Lobsters", Referral},
	"producthunt.com":      {"Product Hunt", Referral},
	"indiehackers.com":     {"Indie Hackers", Referral},
	"dev.to":               {"DEV Community", Referral},
	"hashnode.com":         {"Hashnode", Referral},
	"medium.com":           {"Medium", Referral},
	"substack.com":         {"Substack", Referral},
	"hackernoon.com":       {"HackerNoon", Referral},
	"slashdot.org":         {"Slashdot", Referral},
	"techcrunch.com":       {"TechCrunch", Referral},
	"theverge.com":         {"The Verge", Referral},
	"arstechnica.com":      {"Ars Technica", Referral},
	"wired.com":            {"Wired", Referral},
	"github.com":           {"GitHub", Referral},
	"gitlab.com":           {"GitLab", Referral},
	"stackoverflow.com":    {"Stack Overflow", Referral},
	"quora.com":            {"Quora", Referral},

	// News
	"nytimes.com":        {"NY Times", Referral},
	"washingtonpost.com": {"Washington Post", Referral},
	"theguardian.com":    {"The Guardian", Referral},
	"bbc.com":            {"BBC", Referral},
	"bbc.co.uk":          {"BBC", Referral},
	"cnn.com":            {"CNN", Referral},
	"reuters.com":        {"Reuters", Referral},
	"bloomberg.com":      {"Bloomberg", Referral},
	"forbes.com":         {"Forbes", Referral},
	"wsj.com":            {"WSJ", Referral},
	"ft.com":             {"Financial Times", Referral},

	// Email providers (for newsletter clicks)
	"mail.google.com":    {"Gmail", Email},
	"outlook.live.com":   {"Outlook", Email},
	"outlook.office.com": {"Outlook", Email},
	"mail.yahoo.com":     {"Yahoo Mail", Email},
	"protonmail.com":     {"Proton Mail", Email},
	"mail.proton.me":     {"Proton Mail", Email},

	// Newsletter platforms
	"beehiiv.com":          {"Beehiiv", Email},
	"buttondown.com":       {"Buttondown", Email},
	"buttondown.email":     {"Buttondown", Email},
	"kit.com":              {"Kit", Email},
	"convertkit.com":       {"Kit", Email},
	"list-manage.com":      {"Mailchimp", Email},
	"campaign-archive.com": {"Mailchimp", Email},

	// Mobile apps send their package name as the referrer
	"com.google.android.googlequicksearchbox": {"Google", Search},
	"com.google.android.gm":                   {"Gmail", Email},
	"com.google.android.youtube":              {"YouTube", Social},
	"com.facebook.katana":                     {"Facebook", Social},
	"com.facebook.facebook":                   {"Facebook", Social},
	"com.twitter.android":                     {"X/Twitter", Social},
	"com.linkedin.android":                    {"LinkedIn", Social},
	"com.reddit.frontpage":                    {"Reddit", Social},
	"com.reddit.redditswe":                    {"Reddit", Social},
	"com.instagram.android":                   {"Instagram", Social},
	"com.medium.reader":                       {"Medium", Referral},
	"com.duckduckgo.mobile.android":           {"DuckDuckGo", Search},
	"com.zhiliaoapp.musically":                {"TikTok", Social},
	"com.whatsapp":                            {"WhatsApp", Social},
	"discord.gg":                              {"Discord", Social},

	// Link shorteners
	"bit.ly":      {"Bitly", Referral},
	"tinyurl.com": {"TinyURL", Referral},
	"goo.gl":      {"Google Links", Referral},
	"ow.ly":       {"Hootsuite", Referral},

	// Common utm_source words, so a tagged link and its host name the
	// same source
	"google":      {"Google", Search},
	"bing":        {"Bing", Search},
	"twitter":     {"X/Twitter", Social},
	"x":           {"X/Twitter", Social},
	"facebook":    {"Facebook", Social},
	"fb":          {"Facebook", Social},
	"instagram":   {"Instagram", Social},
	"ig":          {"Instagram", Social},
	"linkedin":    {"LinkedIn", Social},
	"reddit":      {"Reddit", Social},
	"youtube":     {"YouTube", Social},
	"tiktok":      {"TikTok", Social},
	"bluesky":     {"Bluesky", Social},
	"mastodon":    {"Mastodon", Social},
	"hackernews":  {"Hacker News", Referral},
	"hn":          {"Hacker News", Referral},
	"producthunt": {"Product Hunt", Referral},
	"github":      {"GitHub", Referral},
	"chatgpt":     {"ChatGPT", AI},
	"perplexity":  {"Perplexity", AI},
}

// paymentProviders are checkout pages a visitor returns from. A return is
// the same visit going on, never a new source.
var paymentProviders = []string{"stripe.com", "paypal.com", "paddle.com", "lemonsqueezy.com"}

// IsPaymentProvider reports whether a referrer hostname is a payment
// provider's page, or one of its subdomains.
func IsPaymentProvider(hostname string) bool {
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
	for _, provider := range paymentProviders {
		if host == provider || strings.HasSuffix(host, "."+provider) {
			return true
		}
	}
	return false
}

// countryDomainBrands have a site per country (google.de, amazon.co.uk).
var countryDomainBrands = map[string]referrer{
	"google": {"Google", Search},
	"amazon": {"Amazon", Referral},
}

// FriendlyName returns a human-friendly name for a referrer hostname, or the
// hostname itself (lowercase, without "www.") when it is not a known one.
func FriendlyName(hostname string) string {
	if name, ok := Lookup(hostname); ok {
		return name
	}
	return strings.TrimPrefix(strings.ToLower(hostname), "www.")
}

// Lookup returns the friendly name of a known referrer.
func Lookup(hostname string) (string, bool) {
	ref, ok := find(hostname)
	return ref.Name, ok
}

// CategoryOf returns the category of a referrer hostname or utm_source.
// An unknown one is a Referral.
func CategoryOf(hostname string) Category {
	if ref, ok := find(hostname); ok {
		return ref.Category
	}
	return Referral
}

// find returns a known referrer. It tries the full hostname, then each
// parent domain (a.m.youtube.com, m.youtube.com, youtube.com), so the
// longest known match wins and the result never depends on map order. It
// matches whole labels only: evilgoogle.com is not Google.
func find(hostname string) (referrer, bool) {
	host := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(hostname)), ".")
	whole := host
	for host != "" {
		// A bare word (a utm_source such as "hn") matches only the whole
		// value, never the last label of a domain (example.hn).
		if !strings.Contains(host, ".") && host != whole {
			break
		}
		if ref, ok := knownReferrers[host]; ok {
			return ref, true
		}
		if ref, ok := countryDomain(host); ok {
			return ref, true
		}
		dot := strings.IndexByte(host, '.')
		if dot < 0 {
			break
		}
		host = host[dot+1:]
	}
	return referrer{}, false
}

// countryDomain matches google.de, google.co.in, amazon.com.br: a known brand
// followed by one or two short country labels.
func countryDomain(host string) (referrer, bool) {
	brand, rest, found := strings.Cut(host, ".")
	ref, known := countryDomainBrands[brand]
	if !found || !known {
		return referrer{}, false
	}
	labels := strings.Split(rest, ".")
	if len(labels) > 2 {
		return referrer{}, false
	}
	for _, label := range labels {
		if label == "" || len(label) > 3 {
			return referrer{}, false
		}
	}
	return ref, true
}
