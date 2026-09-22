package http

import (
	"encoding/json"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"

	"fusionaly/internal/mcp"
)

// MCPAction handles POST /mcp: the Model Context Protocol over Streamable HTTP.
// AgentAPIKeyAuth on the route checks the same agent key as /z/api.
func MCPAction(ctx *cartridge.Context) error {
	var req mcp.Request
	if err := json.Unmarshal(ctx.Ctx.Body(), &req); err != nil {
		return ctx.Ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"jsonrpc": "2.0",
			"id":      nil,
			"error":   fiber.Map{"code": -32700, "message": "invalid JSON"},
		})
	}

	res, answered := mcp.Handle(ctx.Ctx.Context(), ctx.DB(), req)
	if !answered {
		// A notification. The spec wants an acknowledgement with no body, and
		// SendStatus would fill the body with the status text.
		ctx.Ctx.Status(fiber.StatusAccepted)
		return nil
	}

	return ctx.Ctx.JSON(res)
}
