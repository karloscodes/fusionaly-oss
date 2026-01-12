package events

import (
	"encoding/json"
	"net"
	"strings"

	"log/slog"

	"fusionaly/internal/pkg/geoip"
	ua "fusionaly/internal/pkg/user_agent"
)

// getDeviceTypeFromParsedUA extracts device type from parsed user agent
func getDeviceTypeFromParsedUA(ua ua.UserAgent) string {
	if ua.Mobile {
		return "mobile"
	}
	if ua.Tablet {
		return "tablet"
	}
	if ua.Desktop {
		return "desktop"
	}
	// Add check for Bot if needed from ua.Bot
	return UnknownDevice
}

// getBrowserFromParsedUA extracts and normalizes browser name from parsed user agent
func getBrowserFromParsedUA(ua ua.UserAgent) string {
	// Bot filtering should ideally happen before this function is called (as it does in processEventBatch).
	if ua.Bot || ua.Browser == "" {
		return UnknownBrowser
	}

	browserName := strings.ToLower(ua.Browser)

	// The new Matomo-based parser already provides normalized browser names
	// Only need basic mapping for consistency with existing data
	switch browserName {
	case "internet explorer":
		return "ie"
	case "mobile safari":
		return "safari"
	case "chrome mobile", "chrome mobile webview":
		return "chrome"
	case "firefox mobile":
		return "firefox"
	case "opera mini", "opera mobile":
		return "opera"
	case "edge mobile":
		return "edge"
	default:
		// Return the normalized name from the parser directly
		return browserName
	}
}

// NormalizeOperatingSystem normalizes operating system names to standardize them
func NormalizeOperatingSystem(os string) string {
	if os == "" {
		return UnknownOS
	}

	// Convert to lowercase for comparison
	osLower := strings.ToLower(os)

	// Normalize macOS variations
	if strings.Contains(osLower, "mac") || strings.Contains(osLower, "darwin") {
		return "MacOS"
	}

	// Normalize Linux variations
	if strings.Contains(osLower, "linux") || strings.Contains(osLower, "gnu/linux") {
		return "Linux"
	}

	// Normalize iOS variations
	if strings.Contains(osLower, "ios") || strings.Contains(osLower, "iphone os") {
		return "iOS"
	}

	// Normalize Android
	if strings.Contains(osLower, "android") {
		return "Android"
	}

	// Normalize Windows
	if strings.Contains(osLower, "windows") {
		return "Windows"
	}

	// For other operating systems, capitalize the first letter and return as is
	if len(os) > 0 {
		return strings.ToUpper(os[:1]) + strings.ToLower(os[1:])
	}

	return os
}

// getOSFromParsedUA extracts and normalizes OS from parsed user agent
func getOSFromParsedUA(ua ua.UserAgent) string {
	if ua.OS != "" {
		return NormalizeOperatingSystem(ua.OS)
	}
	return UnknownOS
}

// GetCountryFromIP resolves an IP address to a lowercase ISO country code or UnknownCountry.
func GetCountryFromIP(ipAddress string) string {
	// Get logger from context
	logger := slog.Default()
	logger.Debug("Attempting to get country from IP",
		slog.String("ip_address", ipAddress))

	geoDB := geoip.GetGeoDB()
	if geoDB == nil {
		logger.Error("GeoIP database is nil - not initialized properly")
		return UnknownCountry
	}

	ip := net.ParseIP(ipAddress)
	if ip == nil {
		logger.Error("Failed to parse IP address",
			slog.String("ip_address", ipAddress))
		return UnknownCountry
	}

	logger.Debug("Looking up country for IP",
		slog.String("ip_address", ipAddress),
		slog.String("parsed_ip", ip.String()))

	record, err := geoDB.Country(ip)
	if err != nil {
		logger.Error("Error looking up country for IP",
			slog.String("ip_address", ipAddress),
			slog.Any("error", err))
		return UnknownCountry
	}

	if record.Country.IsoCode == "" || record.Country.IsoCode == "--" {
		logger.Debug("Country code not found or invalid",
			slog.String("ip_address", ipAddress),
			slog.String("iso_code", record.Country.IsoCode))
		return UnknownCountry
	}

	logger.Debug("Successfully resolved IP to country",
		slog.String("ip_address", ipAddress),
		slog.String("country_code", record.Country.IsoCode),
		slog.String("country_name", record.Country.Names["en"]))

	return strings.ToLower(record.Country.IsoCode)
}

// ExtractCustomEventKey extracts a key from custom event metadata JSON
func ExtractCustomEventKey(metadata string) string {
	if metadata == "" {
		return "unknown_key"
	}
	var metadataMap map[string]interface{}
	if err := json.Unmarshal([]byte(metadata), &metadataMap); err == nil {
		for _, key := range []string{"key", "event_key", "eventKey"} {
			if val, ok := metadataMap[key].(string); ok {
				return val
			}
		}
	}
	if len(metadata) > 50 {
		return metadata[:50]
	}
	return metadata
}
