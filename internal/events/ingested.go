package events

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/sqlite"
	"gorm.io/gorm"

	"fusionaly/internal/config"
	"fusionaly/internal/pkg/referrers"
	"fusionaly/internal/settings"
	"fusionaly/internal/visitors"
	"fusionaly/internal/websites"
)

// IngestedEvent represents an event stored temporarily before processing
type IngestedEvent struct {
	ID               uint   `gorm:"primaryKey"`
	WebsiteID        uint   `gorm:"index"`
	UserSignature    string `gorm:"index"`
	Hostname         string `gorm:"index"`
	Pathname         string `gorm:"index"`
	RawURL           string
	ReferrerHostname string `gorm:"index"`
	ReferrerPathname string
	EventType        EventType `gorm:"index"`
	CustomEventName  string    `gorm:"index"`
	CustomEventMeta  string
	Timestamp        time.Time `gorm:"index"`
	UserAgent        string
	SecChUa          string
	Country          string
	CreatedAt        time.Time `gorm:"index"`
	Processed        int       `gorm:"index"`
}

// CollectEventInput defines the input required to collect an event.
type CollectEventInput struct {
	IPAddress       string
	UserAgent       string
	SecChUa         string
	ReferrerURL     string
	EventType       EventType
	CustomEventName string
	CustomEventMeta string
	Timestamp       time.Time
	RawUrl          string
	// ReceivedAt is when the server received the event; zero means now.
	// The seeder sets it to write history.
	ReceivedAt time.Time
}

// urlData holds parsed URL components
type urlData struct {
	hostname string
	pathname string
	rawURL   string
}

// Size limits for one event. Real page URLs, event names, and metadata are
// far smaller; larger values only bloat the database and the dashboard.
const (
	MaxURLLength       = 4096
	MaxEventNameLength = 200
	MaxMetadataBytes   = 8 << 10
	MaxUserAgentLength = 1024
	MaxSecChUaLength   = 512
)

// ErrEventTooLarge marks an event over a size limit. The client sent a bad
// request; retrying it cannot succeed.
var ErrEventTooLarge = errors.New("event too large")

// CollectEvent stores an event in the IngestedEvent table
func CollectEvent(dbManager cartridge.DBManager, logger *slog.Logger, input *CollectEventInput) error {
	if len(input.RawUrl) > MaxURLLength || len(input.ReferrerURL) > MaxURLLength ||
		len(input.CustomEventName) > MaxEventNameLength || len(input.CustomEventMeta) > MaxMetadataBytes ||
		len(input.UserAgent) > MaxUserAgentLength || len(input.SecChUa) > MaxSecChUaLength {
		return fmt.Errorf("%w: url and referrer ≤ %d, event name ≤ %d, metadata ≤ %d, user agent ≤ %d, Sec-CH-UA ≤ %d bytes",
			ErrEventTooLarge, MaxURLLength, MaxEventNameLength, MaxMetadataBytes, MaxUserAgentLength, MaxSecChUaLength)
	}

	if input.UserAgent == "" {
		input.UserAgent = "Unknown User Agent"
	}

	urlData, err := parseInputURL(input.RawUrl, logger)
	if err != nil {
		logger.Warn("Failed to parse URL", slog.Any("error", err), slog.String("url", urlForLog(input.RawUrl)))
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	cfg := config.GetConfig()
	if urlData.hostname == "localhost" && cfg.Environment == config.Production {
		logger.Debug("Skipping event for localhost in production environment", slog.String("url", urlForLog(input.RawUrl)))
		return nil
	}

	excluded, err := settings.IsIPExcluded(input.IPAddress)
	if err != nil {
		logger.Error("Error checking IP exclusion", slog.Any("error", err))
	} else if excluded {
		logger.Debug("Skipping event for excluded IP", slog.String("ip", input.IPAddress))
		return nil
	}

	received := input.ReceivedAt.UTC()
	if input.ReceivedAt.IsZero() {
		received = time.Now().UTC()
	}
	if sentBeforeReceiveDay(input.Timestamp, received) {
		// Browser storage replays failed events later. On another day the
		// visitor has another signature, so the event would count as new
		// traffic today. Drop it; the client gets no error and does not retry.
		logger.Debug("Skipping event sent before the receive day", slog.Time("timestamp", input.Timestamp))
		return nil
	}
	input.Timestamp = eventTime(input.Timestamp, received)
	country := GetCountryFromIP(input.IPAddress)
	db := dbManager.GetConnection()

	tempEvent, err := prepareTempEvent(db, logger, input, urlData, country)
	if err != nil {
		logger.Error("Failed to prepare temp event", slog.Any("error", err))
		return classifyWriteError(err)
	}

	err = sqlite.PerformWrite(logger, db, func(tx *gorm.DB) error {
		return tx.Create(tempEvent).Error
	})
	if err != nil {
		logger.Error("Failed to store ingested event", slog.Any("error", err))
		return classifyWriteError(fmt.Errorf("failed to store ingested event: %w", err))
	}

	return nil
}

// sentBeforeReceiveDay reports whether the client sent a time before the
// receive day (UTC), the day the visitor signature belongs to.
func sentBeforeReceiveDay(sent, received time.Time) bool {
	return !sent.IsZero() && sent.UTC().Before(received.Truncate(24*time.Hour))
}

// eventTime returns the time to store for an event, in UTC. A missing time,
// or one in the future (clock skew), becomes the receive time.
func eventTime(sent, received time.Time) time.Time {
	sent = sent.UTC()
	if sent.IsZero() || sent.After(received) {
		return received
	}
	return sent
}

// ErrStorageBusy marks a write that lost to transient SQLite contention. The
// HTTP layer turns it into a 503 with Retry-After, so callers can retry.
var ErrStorageBusy = errors.New("event storage busy")

// ErrInvalidURL marks an event whose url is missing, unparsable, or has no
// hostname. The client sent a bad request; retrying it cannot succeed.
var ErrInvalidURL = errors.New("invalid event URL")

// classifyWriteError tags busy errors so callers test them with errors.Is,
// and passes everything else through untouched.
func classifyWriteError(err error) error {
	if isBusyError(err) {
		return fmt.Errorf("%w: %w", ErrStorageBusy, err)
	}
	return err
}

// isBusyError reports whether a write lost to SQLite contention.
//
// It matches the driver's full lock messages, never a bare "locked" or "busy".
// Cartridge's IsBusyError matches those bare words, which also appear in
// tracked domains and URLs, so a website-not-found error for busybee.com read
// as a busy database and answered 503 instead of 400.
//
// The driver's error code would be exact, but sqlite3.Error exists only under
// cgo, and importing it breaks CGO_ENABLED=0 builds. Gorm's own sqlite driver
// skips that import for the same reason.
func isBusyError(err error) bool {
	if err == nil {
		return false
	}

	// sqlite3_errstr text for SQLITE_BUSY and SQLITE_LOCKED, plus the
	// shared-cache variant. Verified against the driver.
	msg := err.Error()
	return strings.Contains(msg, "database is locked") ||
		strings.Contains(msg, "database table is locked") ||
		strings.Contains(msg, "database schema is locked")
}

// parseInputURL parses a URL string into its components
func parseInputURL(urlStr string, logger *slog.Logger) (*urlData, error) {
	// Check if URL is empty
	if urlStr == "" {
		logger.Error("Empty URL provided")
		return nil, fmt.Errorf("%w: empty", ErrInvalidURL)
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		// A *url.Error includes the full URL. Keep only the cause.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		logger.Error("Failed to parse URL", slog.String("url", urlForLog(urlStr)), slog.Any("error", err))
		return nil, fmt.Errorf("%w: %v", ErrInvalidURL, err)
	}

	// Ensure the URL has a hostname
	hostname := parsedURL.Hostname()
	if hostname == "" {
		logger.Error("URL missing hostname", slog.String("url", urlForLog(urlStr)))
		return nil, fmt.Errorf("%w: no hostname", ErrInvalidURL)
	}

	pathname := parsedURL.Path
	if pathname == "" {
		pathname = "/"
	}

	return &urlData{
		hostname: hostname,
		pathname: pathname,
		rawURL:   withSourceParamsOnly(parsedURL),
	}, nil
}

// urlForLog returns a URL that is safe to log: the scheme and host when the
// URL parses, else at most 64 characters without the query and fragment.
// The path and query can hold personal data.
func urlForLog(raw string) string {
	if parsed, err := url.Parse(raw); err == nil && parsed.Host != "" {
		return parsed.Scheme + "://" + parsed.Host
	}
	if i := strings.IndexAny(raw, "?#"); i >= 0 {
		raw = raw[:i]
	}
	if len(raw) > 64 {
		raw = raw[:64]
	}
	return raw
}

// storedQueryParams are the query parameters that processing reads from
// raw_url. Other parameters can hold personal data (?email=, ?token=), so
// ingested_events does not store them.
var storedQueryParams = append([]string{"utm_source", "utm_medium", "utm_campaign", "utm_term", "utm_content"}, sourceQueryParams...)

// withSourceParamsOnly returns the URL with only storedQueryParams in the
// query, and without user info or fragment.
func withSourceParamsOnly(parsedURL *url.URL) string {
	clean := *parsedURL
	clean.User = nil
	clean.Fragment = ""
	clean.RawFragment = ""
	query := parsedURL.Query()
	kept := url.Values{}
	for _, key := range storedQueryParams {
		if values, ok := query[key]; ok {
			kept[key] = values
		}
	}
	clean.RawQuery = kept.Encode()
	clean.ForceQuery = false
	return clean.String()
}

// prepareTempEvent creates an IngestedEvent from input data
func prepareTempEvent(db *gorm.DB, logger *slog.Logger, input *CollectEventInput, urlData *urlData, country string) (*IngestedEvent, error) {
	referrerHostname := DirectOrUnknownReferrer
	referrerPathname := ""
	if input.ReferrerURL != "" {
		referrerData, err := parseInputURL(input.ReferrerURL, logger)
		if err == nil {
			referrerHostname = referrerData.hostname
			referrerPathname = referrerData.pathname
		} else {
			logger.Warn("Failed to parse referrer URL", slog.String("referrer", input.ReferrerURL), slog.Any("error", err))
		}
	}

	// Try to find the website with the complete hostname first
	websiteID, err := websites.GetWebsiteOrNotFound(db, urlData.hostname)

	// In non-production environments, auto-create localhost website for testing
	// This is a "belt and suspenders" approach - even if setup creates the website,
	// this ensures tests work reliably regardless of timing or setup issues
	cfg := config.GetConfig()
	if err != nil && !cfg.IsProduction() && (urlData.hostname == "localhost" || urlData.hostname == "127.0.0.1") {
		logger.Debug("Auto-creating localhost website for testing", slog.String("hostname", urlData.hostname))
		website := &websites.Website{Domain: urlData.hostname}
		if createErr := websites.CreateWebsite(db, website); createErr != nil {
			// If creation failed (maybe already exists), try to find it again
			websiteID, err = websites.GetWebsiteOrNotFound(db, urlData.hostname)
		} else {
			websiteID = website.ID
			err = nil
		}
	}

	baseDomain := websites.BaseDomainForHost(urlData.hostname)

	if err != nil {
		// If not found, try with the stripped subdomain (base domain)
		var websiteNotFoundErr *websites.WebsiteNotFoundError
		if errors.As(err, &websiteNotFoundErr) {
			// Only try the base domain if it's different from the original hostname
			if baseDomain != urlData.hostname {
				// Check if subdomain tracking is enabled for the base domain
				if !settings.IsSubdomainTrackingEnabled(db, baseDomain) {
					// Subdomain tracking is disabled, return error for original hostname
					return nil, websites.NewWebsiteNotFoundError(urlData.hostname)
				}

				// Subdomain tracking is enabled, try to find the base domain
				websiteID, err = websites.GetWebsiteOrNotFound(db, baseDomain)
				if err != nil {
					// If base domain lookup also fails, return error for original hostname
					return nil, websites.NewWebsiteNotFoundError(urlData.hostname)
				}
			} else {
				// Base domain is the same as original hostname, so it's not found
				return nil, err
			}
		} else {
			// Some other error occurred
			return nil, err
		}
	}
	// A referrer from the site itself is internal navigation, not a source:
	// the page's own host (with or without www.), or with subdomain tracking
	// any host under the site's base domain.
	if referrerHostname != DirectOrUnknownReferrer && referrerHostname != "" {
		sameSite := IsSelfReferral(referrerHostname, urlData.hostname) ||
			(websites.BaseDomainForHost(referrerHostname) == baseDomain && settings.IsSubdomainTrackingEnabled(db, baseDomain))
		// A return from a checkout page goes on with the visit that left for
		// it. After the session timeout it would credit the payment provider.
		if sameSite || referrers.IsPaymentProvider(referrerHostname) {
			logger.Debug("Self-referral detected, treating as direct traffic",
				slog.String("referrer", referrerHostname),
				slog.String("page_hostname", urlData.hostname))

			referrerHostname = DirectOrUnknownReferrer
			referrerPathname = ""
		}
	}

	var userSignature string
	isSubdomainOfSubdomainTrackingEnabledWebsite := baseDomain != urlData.hostname && settings.IsSubdomainTrackingEnabled(db, baseDomain)
	if isSubdomainOfSubdomainTrackingEnabledWebsite {
		userSignature = visitors.BuildUniqueVisitorId(baseDomain, input.IPAddress, input.UserAgent, config.GetConfig().PrivateKey)
	} else {
		userSignature = visitors.BuildUniqueVisitorId(urlData.hostname, input.IPAddress, input.UserAgent, config.GetConfig().PrivateKey)
	}

	return &IngestedEvent{
		WebsiteID:        websiteID,
		UserSignature:    userSignature,
		Hostname:         urlData.hostname,
		Pathname:         urlData.pathname,
		RawURL:           urlData.rawURL,
		ReferrerHostname: referrerHostname,
		ReferrerPathname: referrerPathname,
		EventType:        input.EventType,
		CustomEventName:  input.CustomEventName,
		CustomEventMeta:  input.CustomEventMeta,
		Timestamp:        input.Timestamp,
		UserAgent:        input.UserAgent,
		SecChUa:          input.SecChUa,
		Country:          country,
		CreatedAt:        time.Now().UTC(),
		Processed:        0,
	}, nil
}
