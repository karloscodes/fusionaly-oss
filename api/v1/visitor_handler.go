package v1

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

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
//
// A browser script on another site sends an Origin header. Then only the Origin
// selects the website, and the website must be registered. The response
// allows only that exact origin, so one site cannot read visits to another site.
// Without an Origin (direct navigation), no other site can read the response.
func GetVisitorInfoHandler(ctx *cartridge.Context) error {
	if strings.EqualFold(strings.TrimSpace(ctx.Get("Early-Data")), "1") {
		ctx.Logger.Info("Received early data request, returning 425 to force replay",
			slog.String("path", ctx.Path()))
		return ctx.Status(http.StatusTooEarly).JSON(cartridge.Map{
			"error": "Replay required",
			"code":  "TOO_EARLY",
		})
	}

	ctx.Vary("Origin")
	db := ctx.DBManager.GetConnection()

	var host string
	if origin := strings.TrimSpace(ctx.Get("Origin")); origin != "" {
		if !allowVisitorOrigin(ctx, db, origin) {
			return ctx.Status(http.StatusForbidden).JSON(cartridge.Map{
				"error": "Origin is not a tracked website",
				"code":  "ORIGIN_NOT_ALLOWED",
			})
		}
		host = originHostname(origin)
	} else {
		var ok bool
		host, ok = visitorHostWithoutOrigin(ctx)
		if !ok {
			return ctx.Status(http.StatusBadRequest).JSON(cartridge.Map{
				"error": "Invalid origin context",
				"code":  "INVALID_CONTEXT",
			})
		}
	}
	if host == "" {
		return ctx.Status(http.StatusBadRequest).JSON(cartridge.Map{
			"error": "Missing origin context",
			"code":  "MISSING_CONTEXT",
		})
	}

	websiteID, resolvedDomain, found, err := resolveWebsiteForHost(db, host)
	if err != nil {
		var websiteNotFoundErr *websites.WebsiteNotFoundError
		if errors.As(err, &websiteNotFoundErr) {
			// Return 404 for website not found
			return ctx.Status(http.StatusNotFound).JSON(cartridge.Map{
				"error": websites.NewWebsiteNotFoundError(host).Error(),
				"code":  "WEBSITE_NOT_FOUND",
			})
		}

		ctx.Logger.Error("Failed to resolve website for visitor info",
			slog.String("host", host),
			slog.Any("error", err))
		return ctx.Status(http.StatusInternalServerError).JSON(cartridge.Map{
			"error": "Failed to resolve website",
			"code":  "INTERNAL_ERROR",
		})
	}

	userAgent := ctx.Get("User-Agent")
	if forwardedUA := ctx.Get("X-Forwarded-User-Agent"); forwardedUA != "" {
		userAgent = forwardedUA
	}

	clientIP := getClientIP(ctx)
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
		if err := db.Where("website_id = ? AND user_signature = ? AND event_type != ?", websiteID, userSignature, events.EventTypePageHide).
			Order("timestamp DESC").
			Limit(visitorEventLimit).
			Find(&eventRecords).Error; err != nil {
			ctx.Logger.Error("Failed to load visitor events",
				slog.Any("error", err),
				slog.Uint64("website_id", uint64(websiteID)),
				slog.String("user_signature", userSignature))
			return ctx.Status(http.StatusInternalServerError).JSON(cartridge.Map{
				"error": "Failed to load visitor events",
				"code":  "EVENT_LOAD_ERROR",
			})
		}

		for _, evt := range eventRecords {
			visitorEvents = append(visitorEvents, buildVisitorEventFromProcessed(evt))
		}

		if len(visitorEvents) == 0 {
			ingested := make([]events.IngestedEvent, 0, visitorEventLimit)
			if err := db.Where("website_id = ? AND user_signature = ? AND event_type != ?", websiteID, userSignature, events.EventTypePageHide).
				Order("timestamp DESC").
				Limit(visitorEventLimit).
				Find(&ingested).Error; err != nil {
				ctx.Logger.Error("Failed to load visitor ingested events",
					slog.Any("error", err),
					slog.Uint64("website_id", uint64(websiteID)),
					slog.String("user_signature", userSignature))
				return ctx.Status(http.StatusInternalServerError).JSON(cartridge.Map{
					"error": "Failed to load visitor events",
					"code":  "EVENT_LOAD_ERROR",
				})
			}

			for _, evt := range ingested {
				visitorEvents = append(visitorEvents, buildVisitorEventFromIngested(evt))
			}
		}
	}

	return ctx.Status(http.StatusOK).JSON(cartridge.Map{
		"visitorId":    userSignature,
		"visitorAlias": alias,
		"country":      country,
		"events":       visitorEvents,
		"generatedAt":  time.Now().UTC().Format(time.RFC3339),
	})
}

// VisitorInfoPreflightHandler answers CORS preflight requests. It allows only
// the origin of a registered website.
func VisitorInfoPreflightHandler(ctx *cartridge.Context) error {
	ctx.Vary("Origin")
	origin := strings.TrimSpace(ctx.Get("Origin"))
	if origin != "" && allowVisitorOrigin(ctx, ctx.DBManager.GetConnection(), origin) {
		ctx.Set("Access-Control-Allow-Methods", "GET,OPTIONS")
		ctx.Set("Access-Control-Allow-Headers", "Accept, Content-Type")
	}
	return ctx.SendStatus(http.StatusNoContent)
}

// allowVisitorOrigin sets Access-Control-Allow-Origin to the exact origin when
// the origin belongs to a registered website.
func allowVisitorOrigin(ctx *cartridge.Context, db *gorm.DB, origin string) bool {
	host := originHostname(origin)
	if host == "" {
		return false
	}
	_, _, found, err := resolveWebsiteForHost(db, host)
	if err != nil || !found {
		return false
	}
	ctx.Set("Access-Control-Allow-Origin", origin)
	return true
}

// originHostname returns the host of an Origin header, or "" when the origin
// is "null" or not a valid http(s) origin.
func originHostname(origin string) string {
	parsed, err := url.Parse(origin)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return ""
	}
	return parsed.Hostname()
}

// visitorHostWithoutOrigin selects the host for a request without Origin, in
// this order: the w parameter, the url parameter, the Referer, the Host header.
func visitorHostWithoutOrigin(c *cartridge.Context) (string, bool) {
	if w := strings.TrimSpace(c.Query("w")); w != "" {
		return w, true
	}
	for _, candidate := range []string{c.Query("url"), c.Get("Referer")} {
		value := strings.TrimSpace(candidate)
		if value == "" || strings.EqualFold(value, "null") {
			continue
		}
		parsed, err := url.Parse(value)
		if err != nil || parsed.Host == "" {
			return "", false
		}
		return parsed.Hostname(), true
	}
	return strings.TrimSpace(c.Hostname()), true
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
