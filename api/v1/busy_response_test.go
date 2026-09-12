package v1

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRespondDatabaseBusy(t *testing.T) {
	t.Run("answers 503 with a Retry-After the client can act on", func(t *testing.T) {
		app := fiber.New()
		app.Post("/busy", func(c *fiber.Ctx) error {
			return respondDatabaseBusy(c)
		})

		resp, err := app.Test(httptest.NewRequest("POST", "/busy", nil))
		require.NoError(t, err)

		assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)

		// The SDK only follows the hint on a 503, and parses it as seconds.
		retryAfter, err := strconv.Atoi(resp.Header.Get("Retry-After"))
		require.NoError(t, err, "Retry-After must be a whole number of seconds")
		assert.Positive(t, retryAfter)

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		var respBody map[string]interface{}
		require.NoError(t, json.Unmarshal(body, &respBody))
		assert.Equal(t, "DATABASE_BUSY", respBody["code"])
	})
}
