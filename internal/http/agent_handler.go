package http

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"

	"fusionaly/internal/agent"
)

// AgentSchemaAction returns the database schema for AI agents
func AgentSchemaAction(ctx *cartridge.Context) error {
	schema, err := agent.GetSchema(ctx.DB())
	if err != nil {
		return ctx.Ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve schema",
		})
	}
	return ctx.Ctx.JSON(schema)
}

// AgentSQLAction executes a read-only SQL query
func AgentSQLAction(ctx *cartridge.Context) error {
	var req agent.SQLRequest
	if err := ctx.Ctx.BodyParser(&req); err != nil {
		return ctx.Ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if req.SQL == "" {
		return ctx.Ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "SQL query is required",
		})
	}

	// Validation (read-only, one statement, allowed tables) happens in Query.
	result, err := agent.Query(ctx.Ctx.Context(), ctx.DB(), req.SQL, 5*time.Second)
	if err != nil {
		return ctx.Ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return ctx.Ctx.JSON(result)
}
