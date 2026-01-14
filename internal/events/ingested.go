package events

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"time"

	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/sqlite"
	"gorm.io/gorm"

	"fusionaly/internal/config"
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
	Country          string
	CreatedAt        time.Time `gorm:"index"`
	Processed        int       `gorm:"index"`
}

// CollectEventInput defines the input required to collect an event.
type CollectEventInput struct {
	IPAddress       string
	UserAgent       string
	ReferrerURL     string
	EventType       EventType
	CustomEventName string
	CustomEventMeta string
	Timestamp       time.Time
	RawUrl          string
}

// urlData holds parsed URL components
type urlData struct {
	hostname string
	pathname string
	rawURL   string
}

// CollectEvent stores an event in the IngestedEvent table
func CollectEvent(dbManager cartridge.DBManager, logger *slog.Logger, input *CollectEventInput) error {
	if input.UserAgent == "" {
		input.UserAgent = "Unknown User Agent"
	}

	urlData, err := parseInputURL(input.RawUrl, logger)
	if err != nil {
		logger.Warn("Failed to parse URL", slog.Any("error", err), slog.String("url", input.RawUrl))
		return fmt.Errorf("failed to parse URL: %w", err)
	}

	cfg := config.GetConfig()
	if urlData.hostname == "localhost" && cfg.Environment == config.Production {
		logger.Debug("Skipping event for localhost in production environment", slog.String("url", input.RawUrl))
		return nil
	}

	excluded, err := settings.IsIPExcluded(input.IPAddress)
	if err != nil {
		logger.Error("Error checking IP exclusion", slog.Any("error", err))
	} else if excluded {
		logger.Debug("Skipping event for excluded IP", slog.String("ip", input.IPAddress))
		return nil
	}

	country := GetCountryFromIP(input.IPAddress)
	db := dbManager.GetConnection()

	tempEvent, err := prepareTempEvent(db, logger, input, urlData, country)
	if err != nil {
		logger.Error("Failed to prepare temp event", slog.Any("error", err))
		return err
	}

	err = sqlite.PerformWrite(logger, db, func(tx *gorm.DB) error {
		return tx.Create(tempEvent).Error
	})
	if err != nil {
		logger.Error("Failed to store ingested event", slog.Any("error", err))
		return fmt.Errorf("failed to store ingested event: %w", err)
	}

	return nil
}

// parseInputURL parses a URL string into its components
func parseInputURL(urlStr string, logger *slog.Logger) (*urlData, error) {
	// Check if URL is empty
	if urlStr == "" {
		logger.Error("Empty URL provided")
		return nil, fmt.Errorf("empty URL provided")
	}

	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		logger.Error("Failed to parse URL", slog.String("url", urlStr), slog.Any("error", err))
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Ensure the URL has a hostname
	hostname := parsedURL.Hostname()
	if hostname == "" {
		logger.Error("URL missing hostname", slog.String("url", urlStr))
		return nil, fmt.Errorf("URL missing hostname")
	}

	pathname := parsedURL.Path
	if pathname == "" {
		pathname = "/"
	}

	return &urlData{
		hostname: hostname,
		pathname: pathname,
		rawURL:   urlStr,
	}, nil
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
	cfg := config.GetConfig()
	if err != nil && !cfg.IsProduction() && (urlData.hostname == "localhost" || urlData.hostname == "127.0.0.1") {
		logger.Debug("Creating localhost website for testing", slog.String("hostname", urlData.hostname))
		website := &websites.Website{Domain: urlData.hostname}
		if createErr := websites.CreateWebsite(db, website); createErr != nil {
			// If creation failed, try to find it again (race condition)
			websiteID, err = websites.GetWebsiteOrNotFound(db, urlData.hostname)
			if err != nil {
				logger.Error("Failed to create or find localhost website", slog.Any("error", err))
				return nil, err
			}
		} else {
			websiteID = website.ID
			err = nil
		}
	}
	baseDomain := websites.BaseDomainForHost(urlData.hostname)
	websiteDomain := baseDomain

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
	// Check for self-referral and filter it out
	if referrerHostname != DirectOrUnknownReferrer && referrerHostname != "" {
		if IsSelfReferral(referrerHostname, websiteDomain) {
			logger.Debug("Self-referral detected, treating as direct traffic",
				slog.String("referrer", referrerHostname),
				slog.String("website_domain", websiteDomain))

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
		Country:          country,
		CreatedAt:        time.Now().UTC(),
		Processed:        0,
	}, nil
}
