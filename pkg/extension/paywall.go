package extension

// PaywallInfo contains information for displaying upgrade prompts
type PaywallInfo struct {
	Feature     string
	Title       string
	Description string
	UpgradeURL  string
	Price       string
}

// LensPaywall returns paywall info for the Lens feature
func LensPaywall() PaywallInfo {
	return PaywallInfo{
		Feature:     "lens",
		Title:       "Lens is a Pro Feature",
		Description: "Save queries, create custom views, and organize your analytics insights.",
		UpgradeURL:  "https://fusionaly.com/#pricing",
		Price:       "$100 one-time",
	}
}

// InsightsPaywall returns paywall info for the AI Insights feature
func InsightsPaywall() PaywallInfo {
	return PaywallInfo{
		Feature:     "insights",
		Title:       "AI Insights is a Pro Feature",
		Description: "Ask questions about your analytics in plain English. Get instant answers powered by AI.",
		UpgradeURL:  "https://fusionaly.com/#pricing",
		Price:       "$100 one-time",
	}
}

// AIDigestPaywall returns paywall info for the AI Weekly Digest feature
func AIDigestPaywall() PaywallInfo {
	return PaywallInfo{
		Feature:     "ai_digest",
		Title:       "AI Weekly Digest is a Pro Feature",
		Description: "Get weekly AI-generated summaries of your analytics delivered to your inbox.",
		UpgradeURL:  "https://fusionaly.com/#pricing",
		Price:       "$100 one-time",
	}
}
