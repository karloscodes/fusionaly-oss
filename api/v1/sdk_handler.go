package v1

import (
	"bytes"
	_ "embed"
	"text/template"

	"github.com/gofiber/fiber/v2"
	"log/slog"

	"github.com/karloscodes/cartridge"
)

//go:embed sdk.js
var sdkTemplate string

func GetSDKAction(ctx *cartridge.Context) error {
	// Parse the SDK template
	tmpl, err := template.New("./api/v1/sdk.js").Parse(sdkTemplate)
	if err != nil {
		ctx.Logger.Error("Failed to parse SDK template", slog.Any("error", err))
		return ctx.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}

	// Execute the template with the base URL
	var buf bytes.Buffer
	data := map[string]string{
		"BaseURL": ctx.BaseURL(),
	}
	if err := tmpl.Execute(&buf, data); err != nil {
		ctx.Logger.Error("Failed to render SDK template", slog.Any("error", err))
		return ctx.Status(fiber.StatusInternalServerError).SendString("Internal Server Error")
	}

	// Generate ETag for the rendered content
	content := buf.Bytes()
	etag := generateETag(content)

	// Check if the client has the latest version via If-None-Match
	ifNoneMatch := ctx.Get("If-None-Match")
	if ifNoneMatch == etag {
		ctx.Logger.Debug("ETag match, returning 304",
			slog.String("etag", etag),
			slog.String("path", ctx.Path()))
		return ctx.Status(fiber.StatusNotModified).Send(nil) // No body for 304
	}

	// Set response headers and send content
	ctx.Set("Content-Type", "application/javascript")
	ctx.Set("Cache-Control", "public, max-age=3600") // 1 hour
	ctx.Set("ETag", etag)
	ctx.Set("Cross-Origin-Resource-Policy", "cross-origin") // Allow cross-origin requests
	ctx.Logger.Debug("Serving SDK with new ETag",
		slog.String("etag", etag),
		slog.String("path", ctx.Path()))
	return ctx.Send(content)
}
