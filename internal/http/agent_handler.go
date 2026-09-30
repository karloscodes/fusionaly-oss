package http

import (
	"net/http"
	"time"

	"github.com/karloscodes/cartridge"

	"fusionaly/internal/agent"
)

// AgentSchemaAction returns the database schema for AI agents
func AgentSchemaAction(ctx *cartridge.Context) error {
	schema, err := agent.GetSchema(ctx.DB())
	if err != nil {
		return ctx.Status(http.StatusInternalServerError).JSON(cartridge.Map{
			"error": "Failed to retrieve schema",
		})
	}
	return ctx.JSON(schema)
}

// AgentSQLAction executes a read-only SQL query
func AgentSQLAction(ctx *cartridge.Context) error {
	var req agent.SQLRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(http.StatusBadRequest).JSON(cartridge.Map{
			"error": "Invalid request body",
		})
	}

	if req.SQL == "" {
		return ctx.Status(http.StatusBadRequest).JSON(cartridge.Map{
			"error": "SQL query is required",
		})
	}

	// Validation (read-only, one statement, allowed tables) happens in Query.
	result, err := agent.Query(ctx.Context(), ctx.DB(), req.SQL, 5*time.Second)
	if err != nil {
		return ctx.Status(http.StatusBadRequest).JSON(cartridge.Map{
			"error": err.Error(),
		})
	}

	return ctx.JSON(result)
}
