package v1

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"

	"fusionaly/internal/events"
	"fusionaly/internal/websites"
)

const (
	msgEventAdded     = "Event added successfully"
	errInvalidRequest = "Invalid request"
	errInvalidOrigin  = "Invalid origin"
)

type CreateEventParams struct {
	URL           string                 `json:"url"`
	Referrer      string                 `json:"referrer"`
	Timestamp     time.Time              `json:"timestamp"`
	EventType     events.EventType       `json:"eventType"`
	UserID        string                 `json:"userId"`
	EventKey      string                 `json:"eventKey"`
	EventMetadata map[string]interface{} `json:"eventMetadata"`
	UserAgent     string                 `json:"userAgent"`
}

func CreateEventPublicAPIHandler(ctx *cartridge.Context) error {
	ctx.Logger.Info("Received event request", slog.String("method", ctx.Method()), slog.String("path", ctx.Path()))

	userAgentHeader := ctx.Get("User-Agent")
	if forwardedUA := ctx.Get("X-Forwarded-User-Agent"); forwardedUA != "" {
		userAgentHeader = forwardedUA
	}
	ctx.Logger.Info("Received User-Agent header", slog.String("userAgent", userAgentHeader))

	params, err := validateAndParseRequest(ctx.Ctx, ctx.DBManager, ctx.Logger)
	if err != nil {
		ctx.Logger.Debug("Failed to validate request", slog.Any("error", err))
		return handleError(ctx.Ctx, err)
	}

	input := &events.CollectEventInput{
		IPAddress:       getClientIP(ctx.Ctx),
		UserAgent:       params.UserAgent,
		ReferrerURL:     params.Referrer,
		EventType:       params.EventType,
		CustomEventName: params.EventKey,
		CustomEventMeta: metadataFromMap(params.EventMetadata),
		Timestamp:       params.Timestamp,
		RawUrl:          params.URL,
	}

	// Pass dbManager directly to CollectEvent
	if err := events.CollectEvent(ctx.DBManager, ctx.Logger, input); err != nil {
		ctx.Logger.Error("Failed to collect event", slog.Any("error", err))
		if strings.Contains(err.Error(), "database is locked") || strings.Contains(err.Error(), "busy") {
			return ctx.Status(599).JSON(fiber.Map{}) // custom status code
		}

		// Check for website not found error using the custom error type
		var websiteNotFoundErr *websites.WebsiteNotFoundError
		if errors.As(err, &websiteNotFoundErr) {
			return ctx.Status(http.StatusBadRequest).JSON(fiber.Map{
				"error": "Website not found - please register your domain first",
				"code":  "WEBSITE_NOT_FOUND",
			})
		}

		return ctx.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to collect event",
			"code":  "COLLECTION_ERROR",
		})
	}

	ctx.Logger.Info("Collected event successfully")
	return ctx.Status(http.StatusAccepted).JSON(fiber.Map{
		"message": msgEventAdded,
		"status":  http.StatusAccepted,
	})
}

func validateAndParseRequest(c *fiber.Ctx, dbManager cartridge.DBManager, logger *slog.Logger) (*CreateEventParams, error) {
	var params CreateEventParams
	if err := c.BodyParser(&params); err != nil {
		return nil, fiber.NewError(http.StatusBadRequest, errInvalidRequest)
	}

	// Validate Origin header against registered websites
	// The Origin header is set by the browser and cannot be spoofed by JavaScript
	if err := validateOrigin(c, dbManager, logger); err != nil {
		return nil, err
	}

	return &params, nil
}

// validateOrigin checks if the request comes from a registered website domain
// using the Origin header (set automatically by browsers for cross-origin requests)
// or falls back to Referer header for same-origin requests
func validateOrigin(c *fiber.Ctx, dbManager cartridge.DBManager, logger *slog.Logger) error {
	// Get Origin header (set by browser for cross-origin requests)
	origin := c.Get("Origin")

	// Fall back to Referer if Origin is not present
	if origin == "" {
		origin = c.Get("Referer")
	}

	if origin == "" {
		logger.Debug("No Origin or Referer header present")
		return fiber.NewError(http.StatusForbidden, errInvalidOrigin)
	}

	// Parse the origin URL to extract the hostname
	parsedURL, err := url.Parse(origin)
	if err != nil {
		logger.Debug("Failed to parse origin URL", slog.String("origin", origin), slog.Any("error", err))
		return fiber.NewError(http.StatusForbidden, errInvalidOrigin)
	}

	// Get the base domain (e.g., sub.example.com -> example.com)
	hostname := parsedURL.Hostname()
	baseDomain := websites.BaseDomainForHost(hostname)

	// Check if this domain is registered
	db := dbManager.GetConnection()
	_, err = websites.GetWebsiteByDomain(db, baseDomain)
	if err != nil {
		logger.Debug("Origin domain not registered",
			slog.String("origin", origin),
			slog.String("hostname", hostname),
			slog.String("baseDomain", baseDomain))
		return fiber.NewError(http.StatusForbidden, errInvalidOrigin)
	}

	logger.Debug("Origin validated successfully",
		slog.String("origin", origin),
		slog.String("baseDomain", baseDomain))

	return nil
}

// CreateEventBeaconHandler handles event tracking requests sent via navigator.sendBeacon
func CreateEventBeaconHandler(ctx *cartridge.Context) error {
	ctx.Logger.Info("Received beacon event request",
		slog.String("method", ctx.Method()),
		slog.String("path", ctx.Path()))

	// Parse the beacon request (always sent as text/plain)
	body := ctx.Body()
	var params CreateEventParams
	if err := json.Unmarshal(body, &params); err != nil {
		ctx.Logger.Debug("Failed to parse beacon request", slog.Any("error", err))
		return ctx.SendStatus(http.StatusAccepted) // Always return 202 for beacon requests
	}
	ctx.Logger.Debug("Parsed beacon request", slog.Any("params", params))

	// Validate Origin header against registered websites
	if err := validateOrigin(ctx.Ctx, ctx.DBManager, ctx.Logger); err != nil {
		ctx.Logger.Debug("Invalid origin in beacon request")
		return ctx.SendStatus(http.StatusAccepted) // Always return 202 for beacon requests
	}

	// Ensure required fields have valid values
	if params.EventMetadata == nil {
		params.EventMetadata = make(map[string]interface{})
	}

	// Get User-Agent
	userAgentHeader := ctx.Get("User-Agent")
	if forwardedUA := ctx.Get("X-Forwarded-User-Agent"); forwardedUA != "" {
		userAgentHeader = forwardedUA
	}

	// Prepare event input
	input := &events.CollectEventInput{
		IPAddress:       getClientIP(ctx.Ctx),
		UserAgent:       userAgentHeader,
		ReferrerURL:     params.Referrer,
		EventType:       params.EventType,
		CustomEventName: params.EventKey,
		CustomEventMeta: metadataFromMap(params.EventMetadata),
		Timestamp:       params.Timestamp,
		RawUrl:          params.URL,
	}

	// Collect the event
	if err := events.CollectEvent(ctx.DBManager, ctx.Logger, input); err != nil {
		ctx.Logger.Error("Failed to collect beacon event",
			slog.Any("error", err),
			slog.String("eventName", params.EventKey))
		return ctx.SendStatus(http.StatusAccepted) // Always return 202 for beacon requests
	}

	ctx.Logger.Info("Collected beacon event successfully",
		slog.String("eventName", params.EventKey))

	return ctx.SendStatus(http.StatusAccepted)
}

func handleError(c *fiber.Ctx, err error) error {
	if fiberErr, ok := err.(*fiber.Error); ok {
		return c.Status(fiberErr.Code).JSON(fiber.Map{
			"error": fiberErr.Message,
		})
	}

	return c.Status(http.StatusUnprocessableEntity).JSON(fiber.Map{
		"error": errInvalidRequest,
	})
}

// metadataFromMap converts metadata map to string
func metadataFromMap(metadata map[string]interface{}) string {
	if metadata == nil {
		return ""
	}
	data, err := json.Marshal(metadata)
	if err != nil {
		return ""
	}
	return string(data)
}
