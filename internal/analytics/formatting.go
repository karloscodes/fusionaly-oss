package analytics

import (
	"strings"

	"fusionaly/internal/events"

	"github.com/pariz/gountries"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// FormatCountryStats converts country codes to human-readable names.
func FormatCountryStats(items []MetricCountResult) []MetricCountResult {
	caser := cases.Upper(language.AmericanEnglish)
	countries := gountries.New()

	if len(items) == 0 {
		return []MetricCountResult{}
	}

	result := make([]MetricCountResult, len(items))
	for i, item := range items {
		if item.Name == events.UnknownCountry {
			result[i] = MetricCountResult{Name: "Unknown", Count: item.Count}
		} else {
			countryName, err := countries.FindCountryByAlpha(item.Name)
			if err != nil {
				result[i] = MetricCountResult{Name: caser.String(item.Name), Count: item.Count}
			} else {
				result[i] = MetricCountResult{Name: countryName.Name.Common, Count: item.Count}
			}
		}
	}
	return result
}

// FormatDeviceStats title-cases device type names.
func FormatDeviceStats(items []MetricCountResult) []MetricCountResult {
	caser := cases.Title(language.AmericanEnglish)

	if len(items) == 0 {
		return []MetricCountResult{}
	}

	result := make([]MetricCountResult, len(items))
	for i, item := range items {
		name := item.Name
		if name == events.UnknownDevice {
			name = "Unknown"
		}
		result[i] = MetricCountResult{Name: caser.String(name), Count: item.Count}
	}
	return result
}

// FormatReferrerStats converts internal referrer constants to human-readable names.
func FormatReferrerStats(items []MetricCountResult) []MetricCountResult {
	if len(items) == 0 {
		return []MetricCountResult{}
	}

	result := make([]MetricCountResult, len(items))
	for i, item := range items {
		name := item.Name
		if name == events.DirectOrUnknownReferrer {
			name = "Direct / Unknown"
		}
		result[i] = MetricCountResult{Name: name, Count: item.Count}
	}
	return result
}

// FormatOSStats normalizes OS names with correct capitalization.
func FormatOSStats(items []MetricCountResult) []MetricCountResult {
	caser := cases.Title(language.AmericanEnglish)

	if len(items) == 0 {
		return []MetricCountResult{}
	}

	result := make([]MetricCountResult, len(items))
	for i, item := range items {
		name := item.Name

		if name == events.UnknownOS {
			name = "Unknown"
		} else {
			nameLower := strings.ToLower(strings.TrimSpace(name))

			switch nameLower {
			case "ios", "iphone os":
				name = "iOS"
			case "ipados":
				name = "iPadOS"
			case "macos", "mac os", "mac os x", "darwin":
				name = "macOS"
			default:
				name = caser.String(name)
			}
		}
		result[i] = MetricCountResult{Name: name, Count: item.Count}
	}
	return result
}

// FormatBrowserStats title-cases browser names.
func FormatBrowserStats(items []MetricCountResult) []MetricCountResult {
	caser := cases.Title(language.AmericanEnglish)

	if len(items) == 0 {
		return []MetricCountResult{}
	}

	result := make([]MetricCountResult, len(items))
	for i, item := range items {
		name := item.Name
		if name == events.UnknownBrowser {
			name = "Unknown"
		}
		result[i] = MetricCountResult{Name: caser.String(name), Count: item.Count}
	}
	return result
}
