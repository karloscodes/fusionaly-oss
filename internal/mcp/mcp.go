// Package mcp answers Model Context Protocol calls, so an AI client (Claude,
// ChatGPT, Cursor) can answer questions about your analytics.
//
// The transport is Streamable HTTP: one JSON-RPC request per POST, one JSON
// response back. Every tool is read-only, and SQL runs through agent.Query:
// one SELECT, analytics tables only, on a read-only connection.
package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"fusionaly/internal/agent"
	"fusionaly/internal/feed"
	"fusionaly/internal/websites"
)

// SupportedProtocols are the protocol revisions we understand. We answer with
// the client's revision when we know it, otherwise with the newest one.
var SupportedProtocols = []string{"2025-06-18", "2025-03-26", "2024-11-05"}

// Version is the server version reported to clients. The app binary carries
// no version stamp yet, so this stays fixed; bump it when the tools change.
const Version = "1"

// queryTimeout bounds one SQL tool call, like the agent API.
const queryTimeout = 5 * time.Second

type Request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Handle answers one JSON-RPC request. The second return is false for a
// notification, which the spec says to acknowledge without a body.
func Handle(ctx context.Context, db *gorm.DB, req Request) (*Response, bool) {
	if req.ID == nil {
		return nil, false
	}

	res := &Response{JSONRPC: "2.0", ID: req.ID}

	switch req.Method {
	case "initialize":
		res.Result = initialize(req.Params)
	case "tools/list":
		res.Result = map[string]any{"tools": Tools}
	case "tools/call":
		res.Result = callTool(ctx, db, req.Params)
	case "ping":
		res.Result = map[string]any{}
	default:
		res.Error = &Error{Code: -32601, Message: "unknown method: " + req.Method}
	}

	return res, true
}

func initialize(params json.RawMessage) map[string]any {
	var p struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	_ = json.Unmarshal(params, &p)

	protocol := SupportedProtocols[0]
	for _, v := range SupportedProtocols {
		if p.ProtocolVersion == v {
			protocol = v
			break
		}
	}

	return map[string]any{
		"protocolVersion": protocol,
		"capabilities":    map[string]any{"tools": map[string]any{}},
		"serverInfo":      map[string]any{"name": "fusionaly", "version": Version},
		"instructions": "Fusionaly is privacy-first web analytics. Call list_websites for a website_id, " +
			"get_schema before your first query, then query with SQL. whats_new lists what changed recently.",
	}
}

// Tools describes what an agent may do. The descriptions are written for the
// agent reading them, so they say when to reach for each one.
var Tools = []map[string]any{
	{
		"name":        "list_websites",
		"description": "List the websites Fusionaly tracks, with their ids. Call this first: every query filters by website_id.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
	},
	{
		"name": "whats_new",
		"description": "What changed recently, already detected for you: traffic spikes and drops against the usual day, " +
			"new referrers, goal spikes, trending pages, milestones, and monthly summaries. " +
			"Start here when asked how things are going.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"website_id": map[string]any{"type": "integer", "description": "Restrict to one website, from list_websites."},
				"limit":      map[string]any{"type": "integer", "description": "Max items, 1 to 100. Default 20."},
			},
		},
	},
	{
		"name": "get_schema",
		"description": "The analytics tables and their columns, plus how to use them: hourly UTC buckets, " +
			"always SUM the count columns, always filter website_id. Read it before your first query.",
		"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
	},
	{
		"name": "query",
		"description": "Run one read-only SQLite SELECT (or WITH) against the analytics tables. " +
			"Returns columns and rows, at most 1000 rows. Always filter website_id and a time range on hour.",
		"inputSchema": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"sql": map[string]any{"type": "string", "description": "One SELECT statement, no comments."},
			},
			"required": []string{"sql"},
		},
	},
}

func callTool(ctx context.Context, db *gorm.DB, params json.RawMessage) map[string]any {
	var p struct {
		Name      string         `json:"name"`
		Arguments map[string]any `json:"arguments"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return toolError("could not read the tool call: " + err.Error())
	}

	payload, err := runTool(ctx, db, p.Name, p.Arguments)
	if err != nil {
		return toolError(err.Error())
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return toolError("could not encode the result: " + err.Error())
	}

	return map[string]any{
		"content": []map[string]any{{"type": "text", "text": string(body)}},
	}
}

func runTool(ctx context.Context, db *gorm.DB, name string, args map[string]any) (any, error) {
	switch name {
	case "list_websites":
		return listWebsites(db)
	case "whats_new":
		return whatsNew(db, args)
	case "get_schema":
		return agent.GetSchema(db)
	case "query":
		return runQuery(ctx, db, args)
	}

	return nil, fmt.Errorf("unknown tool: %s", name)
}

func listWebsites(db *gorm.DB) (any, error) {
	all, err := websites.GetAllWebsites(db)
	if err != nil {
		return nil, fmt.Errorf("could not read the websites: %w", err)
	}

	list := make([]map[string]any, len(all))
	for i, w := range all {
		list[i] = map[string]any{"id": w.ID, "domain": w.Domain}
	}
	return map[string]any{"websites": list}, nil
}

func whatsNew(db *gorm.DB, args map[string]any) (any, error) {
	var ids []uint
	if id := argInt(args, "website_id"); id > 0 {
		ids = []uint{uint(id)}
	} else {
		all, err := websites.GetAllWebsites(db)
		if err != nil {
			return nil, fmt.Errorf("could not read the websites: %w", err)
		}
		for _, w := range all {
			ids = append(ids, w.ID)
		}
	}

	items, err := feed.GetUserFeed(db, ids, clampLimit(argInt(args, "limit"), 20, 100))
	if err != nil {
		return nil, fmt.Errorf("could not read the feed: %w", err)
	}

	list := make([]map[string]any, len(items))
	for i, item := range items {
		list[i] = map[string]any{
			"website_id":   item.WebsiteID,
			"type":         item.ItemType,
			"title":        item.Title,
			"description":  item.Description,
			"period_start": item.PeriodStart,
			"period_end":   item.PeriodEnd,
			"details":      item.MetadataMap(),
		}
	}
	return map[string]any{"items": list}, nil
}

func runQuery(ctx context.Context, db *gorm.DB, args map[string]any) (any, error) {
	sql := argString(args, "sql")
	if sql == "" {
		return nil, fmt.Errorf("sql is required: one SELECT statement")
	}
	return agent.Query(ctx, db, sql, queryTimeout)
}

func clampLimit(limit, fallback, max int) int {
	switch {
	case limit <= 0:
		return fallback
	case limit > max:
		return max
	default:
		return limit
	}
}

func argString(args map[string]any, key string) string {
	s, _ := args[key].(string)
	return strings.TrimSpace(s)
}

// argInt reads a number that arrived as JSON, so float64, or as a string from
// a client that quotes its arguments.
func argInt(args map[string]any, key string) int {
	switch v := args[key].(type) {
	case float64:
		return int(v)
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(v))
		return n
	}
	return 0
}

func toolError(message string) map[string]any {
	return map[string]any{
		"isError": true,
		"content": []map[string]any{{"type": "text", "text": message}},
	}
}
