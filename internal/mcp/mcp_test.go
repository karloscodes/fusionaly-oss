package mcp_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"fusionaly/internal/mcp"
	"fusionaly/internal/testsupport"
)

func call(t *testing.T, method string, params any) *mcp.Response {
	t.Helper()
	dbManager, _ := testsupport.SetupTestDBManager(t)
	raw, _ := json.Marshal(params)

	res, answered := mcp.Handle(context.Background(), dbManager.GetConnection(), mcp.Request{
		JSONRPC: "2.0", ID: json.RawMessage(`1`), Method: method, Params: raw,
	})

	require.True(t, answered)
	return res
}

// toolText returns the text of a tools/call result and whether it is an error.
func toolText(t *testing.T, res *mcp.Response) (string, bool) {
	t.Helper()
	result := res.Result.(map[string]any)
	content := result["content"].([]map[string]any)
	isError, _ := result["isError"].(bool)
	return content[0]["text"].(string), isError
}

func TestHandle(t *testing.T) {
	t.Run("initialize answers with the client's protocol when known", func(t *testing.T) {
		res := call(t, "initialize", map[string]any{"protocolVersion": "2025-03-26"})

		result := res.Result.(map[string]any)
		assert.Equal(t, "2025-03-26", result["protocolVersion"])
		assert.Equal(t, "fusionaly", result["serverInfo"].(map[string]any)["name"])
	})

	t.Run("tools/list lists the read-only tools", func(t *testing.T) {
		res := call(t, "tools/list", map[string]any{})

		var names []string
		for _, tool := range res.Result.(map[string]any)["tools"].([]map[string]any) {
			names = append(names, tool["name"].(string))
		}
		assert.Equal(t, []string{"list_websites", "whats_new", "get_schema", "query"}, names)
	})

	t.Run("an unknown method is a JSON-RPC error", func(t *testing.T) {
		res := call(t, "resources/list", map[string]any{})

		require.NotNil(t, res.Error)
		assert.Equal(t, -32601, res.Error.Code)
	})

	t.Run("a notification gets no response", func(t *testing.T) {
		dbManager, _ := testsupport.SetupTestDBManager(t)

		_, answered := mcp.Handle(context.Background(), dbManager.GetConnection(), mcp.Request{
			JSONRPC: "2.0", Method: "notifications/initialized",
		})

		assert.False(t, answered)
	})
}

func TestTools(t *testing.T) {
	t.Run("list_websites returns ids and domains", func(t *testing.T) {
		// SetupTestDBManager caches the DB per root test, so call() sees this website.
		testsupport.SetupTestDBManagerWithWebsite(t, "shop.test")

		res := call(t, "tools/call", map[string]any{"name": "list_websites"})

		text, isError := toolText(t, res)
		assert.False(t, isError)
		assert.Contains(t, text, `"domain":"shop.test"`)
	})

	t.Run("get_schema lists analytics tables only", func(t *testing.T) {
		res := call(t, "tools/call", map[string]any{"name": "get_schema"})

		text, isError := toolText(t, res)
		assert.False(t, isError)
		assert.Contains(t, text, "site_stats")
		assert.NotContains(t, text, "CREATE TABLE `users`")
		assert.NotContains(t, text, "CREATE TABLE `settings`")
	})

	t.Run("query runs a SELECT", func(t *testing.T) {
		res := call(t, "tools/call", map[string]any{
			"name":      "query",
			"arguments": map[string]any{"sql": "SELECT 1 AS one"},
		})

		text, isError := toolText(t, res)
		assert.False(t, isError)
		assert.Contains(t, text, `"columns":["one"]`)
	})

	t.Run("query refuses tables that hold secrets", func(t *testing.T) {
		res := call(t, "tools/call", map[string]any{
			"name":      "query",
			"arguments": map[string]any{"sql": "SELECT key, value FROM settings"},
		})

		text, isError := toolText(t, res)
		assert.True(t, isError)
		assert.Contains(t, text, "table not allowed")
	})

	t.Run("query refuses writes", func(t *testing.T) {
		res := call(t, "tools/call", map[string]any{
			"name":      "query",
			"arguments": map[string]any{"sql": "DELETE FROM site_stats"},
		})

		_, isError := toolText(t, res)
		assert.True(t, isError)
	})

	t.Run("whats_new returns feed items", func(t *testing.T) {
		res := call(t, "tools/call", map[string]any{"name": "whats_new"})

		text, isError := toolText(t, res)
		assert.False(t, isError)
		assert.Contains(t, text, `"items":`)
	})
}
