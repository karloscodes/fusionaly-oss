package v1

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"
	"gorm.io/gorm"

	"fusionaly/internal/config"
	"fusionaly/internal/events"
	"fusionaly/internal/visitors"
	"fusionaly/internal/websites"
)

const visitorEventLimit = 25

type visitorEvent struct {
	Timestamp      time.Time        `json:"timestamp"`
	URL            string           `json:"url"`
	Referrer       string           `json:"referrer,omitempty"`
	EventType      events.EventType `json:"eventType"`
	CustomEventKey string           `json:"customEventKey,omitempty"`
}

// GetVisitorInfoHandler returns current visitor metadata based on the request context.
func GetVisitorInfoHandler(ctx *cartridge.Context) error {
	requestURL := resolveVisitorContextURL(ctx.Ctx)
	hostParam := strings.TrimSpace(ctx.Query("w"))
	var host string
	switch {
	case strings.EqualFold(strings.TrimSpace(ctx.Get("Early-Data")), "1"):
		ctx.Logger.Info("Received early data request, returning 425 to force replay",
			slog.String("path", ctx.Path()))
		return ctx.Status(fiber.StatusTooEarly).JSON(fiber.Map{
			"error": "Replay required",
			"code":  "TOO_EARLY",
		})
	case hostParam != "":
		host = hostParam
	case requestURL != "":
		parsedURL, err := url.Parse(requestURL)
		if err != nil || parsedURL.Host == "" {
			return ctx.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid origin context",
				"code":  "INVALID_CONTEXT",
			})
		}
		host = parsedURL.Hostname()
	default:
		host = strings.TrimSpace(ctx.Hostname())
	}
	if host == "" {
		return ctx.Status(http.StatusBadRequest).JSON(fiber.Map{
			"error": "Missing origin context",
			"code":  "MISSING_CONTEXT",
		})
	}
	db := ctx.DBManager.GetConnection()

	websiteID, resolvedDomain, found, err := resolveWebsiteForHost(db, host)
	if err != nil {
		var websiteNotFoundErr *websites.WebsiteNotFoundError
		if errors.As(err, &websiteNotFoundErr) {
			// Return 404 for website not found
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"error": websites.NewWebsiteNotFoundError(host).Error(),
				"code":  "WEBSITE_NOT_FOUND",
			})
		}

		ctx.Logger.Error("Failed to resolve website for visitor info",
			slog.String("host", host),
			slog.Any("error", err))
		return ctx.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to resolve website",
			"code":  "INTERNAL_ERROR",
		})
	}

	userAgent := ctx.Get("User-Agent")
	if forwardedUA := ctx.Get("X-Forwarded-User-Agent"); forwardedUA != "" {
		userAgent = forwardedUA
	}

	clientIP := getClientIP(ctx.Ctx)
	country := events.GetCountryFromIP(clientIP)

	signatureDomain := resolvedDomain
	if signatureDomain == "" {
		signatureDomain = websites.BaseDomainForHost(host)
	}
	if signatureDomain == "" {
		signatureDomain = host
	}

	// Type-assert to get fusionaly-specific config fields
	cfg := ctx.Config.(*config.Config)
	userSignature := visitors.BuildUniqueVisitorId(signatureDomain, clientIP, userAgent, cfg.PrivateKey)
	alias := visitors.VisitorAlias(userSignature)

	visitorEvents := make([]visitorEvent, 0, visitorEventLimit)

	if found {
		eventRecords := make([]events.Event, 0, visitorEventLimit)
		if err := db.Where("website_id = ? AND user_signature = ?", websiteID, userSignature).
			Order("timestamp DESC").
			Limit(visitorEventLimit).
			Find(&eventRecords).Error; err != nil {
			ctx.Logger.Error("Failed to load visitor events",
				slog.Any("error", err),
				slog.Uint64("website_id", uint64(websiteID)),
				slog.String("user_signature", userSignature))
			return ctx.Status(http.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to load visitor events",
				"code":  "EVENT_LOAD_ERROR",
			})
		}

		for _, evt := range eventRecords {
			visitorEvents = append(visitorEvents, buildVisitorEventFromProcessed(evt))
		}

		if len(visitorEvents) == 0 {
			ingested := make([]events.IngestedEvent, 0, visitorEventLimit)
			if err := db.Where("website_id = ? AND user_signature = ?", websiteID, userSignature).
				Order("timestamp DESC").
				Limit(visitorEventLimit).
				Find(&ingested).Error; err != nil {
				ctx.Logger.Error("Failed to load visitor ingested events",
					slog.Any("error", err),
					slog.Uint64("website_id", uint64(websiteID)),
					slog.String("user_signature", userSignature))
				return ctx.Status(http.StatusInternalServerError).JSON(fiber.Map{
					"error": "Failed to load visitor events",
					"code":  "EVENT_LOAD_ERROR",
				})
			}

			for _, evt := range ingested {
				visitorEvents = append(visitorEvents, buildVisitorEventFromIngested(evt))
			}
		}
	}

	return ctx.Status(http.StatusOK).JSON(fiber.Map{
		"visitorId":    userSignature,
		"visitorAlias": alias,
		"country":      country,
		"events":       visitorEvents,
		"generatedAt":  time.Now().UTC().Format(time.RFC3339),
	})
}

func resolveVisitorContextURL(c *fiber.Ctx) string {
	for _, candidate := range []string{
		c.Get("Origin"),
		c.Query("url"),
		c.Get("Referer"),
	} {
		value := strings.TrimSpace(candidate)
		if value == "" || strings.EqualFold(value, "null") {
			continue
		}
		return value
	}
	return ""
}

func resolveWebsiteForHost(db *gorm.DB, host string) (uint, string, bool, error) {
	if host == "" {
		return 0, "", false, websites.NewWebsiteNotFoundError(host)
	}

	websiteID, err := websites.GetWebsiteOrNotFound(db, host)
	if err == nil {
		return websiteID, host, true, nil
	}

	var notFound *websites.WebsiteNotFoundError
	if errors.As(err, &notFound) {
		baseDomain := websites.BaseDomainForHost(host)
		if baseDomain != host {
			websiteID, baseErr := websites.GetWebsiteOrNotFound(db, baseDomain)
			if baseErr == nil {
				return websiteID, baseDomain, true, nil
			}

			if baseErr != nil && !errors.As(baseErr, &notFound) {
				return 0, "", false, baseErr
			}
			return 0, baseDomain, false, nil
		}

		// No matching website found; return graceful fallback
		return 0, host, false, nil
	}

	return 0, "", false, err
}

func buildVisitorEventFromProcessed(evt events.Event) visitorEvent {
	return visitorEvent{
		Timestamp:      evt.Timestamp,
		URL:            evt.Hostname + evt.Pathname,
		Referrer:       safeReferrer(evt.ReferrerHostname, evt.ReferrerPathname),
		EventType:      evt.EventType,
		CustomEventKey: evt.CustomEventName,
	}
}

func buildVisitorEventFromIngested(evt events.IngestedEvent) visitorEvent {
	return visitorEvent{
		Timestamp:      evt.Timestamp,
		URL:            evt.Hostname + evt.Pathname,
		Referrer:       safeReferrer(evt.ReferrerHostname, evt.ReferrerPathname),
		EventType:      evt.EventType,
		CustomEventKey: evt.CustomEventName,
	}
}

func safeReferrer(host, path string) string {
	if host == "" || host == events.DirectOrUnknownReferrer {
		return ""
	}
	return host + path
}
