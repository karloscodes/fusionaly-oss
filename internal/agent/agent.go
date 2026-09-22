// Package agent provides the Agent API for external tools like Claude Code
// to query Fusionaly analytics data via SQL.
package agent

import (
	"context"
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

// SchemaResponse is the response format for the schema endpoint
type SchemaResponse struct {
	Schema   string            `json:"schema"`
	Concepts map[string]string `json:"concepts"`
}

// SQLRequest is the request format for the SQL endpoint
type SQLRequest struct {
	SQL string `json:"sql"`
}

// SQLResponse is the response format for the SQL endpoint
type SQLResponse struct {
	Columns   []string        `json:"columns"`
	Rows      [][]interface{} `json:"rows"`
	RowCount  int             `json:"row_count"`
	Truncated bool            `json:"truncated"` // true when the result hit MaxRows
}

// GetSchema returns the database schema with concepts and examples
func GetSchema(db *gorm.DB) (*SchemaResponse, error) {
	schema, err := GetDatabaseSchema(db)
	if err != nil {
		return nil, err
	}

	return &SchemaResponse{
		Schema: schema,
		Concepts: map[string]string{
			"time_filtering":      "Data is aggregated hourly in UTC. Filter by 'hour' column.",
			"website_scoping":     "Data is multi-tenant. Always filter by 'website_id'.",
			"visitors_vs_views":   "visitors_count = unique users, page_views_count = total actions.",
			"country_codes":       "Countries use lowercase ISO codes: 'us', 'gb', 'de', 'fr', etc.",
			"direct_traffic":      "Direct traffic has empty hostname in ref_stats.",
			"bounce_rate":         "Calculate as: (bounce_count * 100.0 / sessions) from site_stats.",
			"aggregation_pattern": "Always SUM() count columns when grouping. Raw values are hourly increments.",
		},
	}, nil
}

// AllowedTables are the tables agents may read: analytics data only. Everything
// else (users, settings, sessions, caches) holds secrets or internals and stays
// out of the schema and out of queries.
var AllowedTables = map[string]bool{
	"websites":              true,
	"events":                true,
	"site_stats":            true,
	"page_stats":            true,
	"ref_stats":             true,
	"event_stats":           true,
	"browser_stats":         true,
	"os_stats":              true,
	"device_stats":          true,
	"country_stats":         true,
	"utm_stats":             true,
	"query_param_stats":     true,
	"flow_transition_stats": true,
	"annotations":           true,
	"feed_items":            true,
}

// MaxRows caps a query result so one SELECT cannot load a whole table.
const MaxRows = 1000

// GetDatabaseSchema returns the CREATE TABLE statements of the allowed tables.
func GetDatabaseSchema(db *gorm.DB) (string, error) {
	type table struct {
		Name string
		SQL  string
	}
	var tables []table
	if err := db.Raw("SELECT name, sql FROM sqlite_master WHERE type = 'table' ORDER BY name").Scan(&tables).Error; err != nil {
		return "", err
	}

	var schemas []string
	for _, t := range tables {
		if AllowedTables[t.Name] && t.SQL != "" {
			schemas = append(schemas, t.SQL)
		}
	}
	return strings.Join(schemas, ";\n") + ";", nil
}

// deniedWords block statements and functions that write, change the session,
// or reach outside the database. query_only already stops writes; this list
// gives a clear error first.
var deniedWords = map[string]bool{
	"insert": true, "update": true, "delete": true, "drop": true, "alter": true,
	"create": true, "replace": true, "truncate": true, "pragma": true,
	"attach": true, "detach": true, "vacuum": true, "reindex": true, "analyze": true,
	"begin": true, "commit": true, "rollback": true, "savepoint": true, "release": true,
	"load_extension": true, "writefile": true, "readfile": true,
}

// ValidateReadOnlyQuery checks that sqlQuery is one SELECT (or WITH) statement
// that only reads allowed tables. It tokenizes string literals, quoted
// identifiers, and comments together, so a "/*" or ";" inside a string cannot
// hide a second statement.
func ValidateReadOnlyQuery(sqlQuery string, tableNames []string) error {
	words, err := sqlWords(sqlQuery)
	if err != nil {
		return err
	}
	if len(words) == 0 || (words[0] != "select" && words[0] != "with") {
		return fmt.Errorf("only SELECT queries are allowed")
	}

	existing := make(map[string]bool, len(tableNames))
	for _, name := range tableNames {
		existing[strings.ToLower(name)] = true
	}

	for _, w := range words {
		if deniedWords[w] {
			return fmt.Errorf("operation not allowed: %s", w)
		}
		if strings.HasPrefix(w, "pragma_") || strings.HasPrefix(w, "sqlite_") {
			return fmt.Errorf("table not allowed: %s", w)
		}
		if existing[w] && !AllowedTables[w] {
			return fmt.Errorf("table not allowed: %s", w)
		}
	}
	return nil
}

// sqlWords returns the lowercased keywords and identifiers of one statement.
// String literals are skipped. Comments and a second statement are errors.
func sqlWords(q string) ([]string, error) {
	var words []string
	for i := 0; i < len(q); {
		c := q[i]
		switch {
		case c == '\'':
			end := closingQuote(q, i, '\'')
			if end < 0 {
				return nil, fmt.Errorf("unterminated string literal")
			}
			i = end + 1
		case c == '"' || c == '`' || c == '[':
			closer := c
			if c == '[' {
				closer = ']'
			}
			end := closingQuote(q, i, closer)
			if end < 0 {
				return nil, fmt.Errorf("unterminated identifier")
			}
			words = append(words, strings.ToLower(q[i+1:end]))
			i = end + 1
		case strings.HasPrefix(q[i:], "--") || strings.HasPrefix(q[i:], "/*"):
			return nil, fmt.Errorf("comments not allowed in queries")
		case c == ';':
			if strings.TrimSpace(q[i+1:]) != "" {
				return nil, fmt.Errorf("multiple statements not allowed")
			}
			return words, nil
		case isWordByte(c):
			start := i
			for i < len(q) && isWordByte(q[i]) {
				i++
			}
			words = append(words, strings.ToLower(q[start:i]))
		default:
			i++
		}
	}
	return words, nil
}

// closingQuote returns the index of the quote that closes the one at start.
// A doubled quote (” or "") is an escaped quote, not the end.
func closingQuote(q string, start int, quote byte) int {
	for i := start + 1; i < len(q); i++ {
		if q[i] != quote {
			continue
		}
		if quote != ']' && i+1 < len(q) && q[i+1] == quote {
			i++
			continue
		}
		return i
	}
	return -1
}

func isWordByte(c byte) bool {
	return c == '_' || c == '$' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= 0x80
}

// Query validates and runs one read-only query with a timeout and a row cap.
// It runs on a dedicated connection with PRAGMA query_only, so SQLite itself
// rejects any write the validator misses. query_only is switched off again
// before the connection returns to the pool; if that fails, the connection
// is discarded instead.
func Query(ctx context.Context, db *gorm.DB, sqlQuery string, timeout time.Duration) (*SQLResponse, error) {
	var tableNames []string
	if err := db.Raw("SELECT name FROM sqlite_master WHERE type = 'table'").Scan(&tableNames).Error; err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	if err := ValidateReadOnlyQuery(sqlQuery, tableNames); err != nil {
		return nil, err
	}

	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	conn, err := sqlDB.Conn(queryCtx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer func() {
		// Background context: the reset must run even after a timeout.
		if _, err := conn.ExecContext(context.Background(), "PRAGMA query_only = OFF"); err != nil {
			_ = conn.Raw(func(any) error { return driver.ErrBadConn })
		}
		_ = conn.Close()
	}()

	if _, err := conn.ExecContext(queryCtx, "PRAGMA query_only = ON"); err != nil {
		return nil, fmt.Errorf("failed to enter read-only mode: %w", err)
	}

	rows, err := conn.QueryContext(queryCtx, sqlQuery)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	resultRows := make([][]interface{}, 0)
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	truncated := false
	for rows.Next() {
		if len(resultRows) == MaxRows {
			truncated = true
			break
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("row scan failed: %w", err)
		}

		row := make([]interface{}, len(columns))
		for i, val := range values {
			if b, ok := val.([]byte); ok {
				row[i] = string(b)
			} else {
				row[i] = val
			}
		}
		resultRows = append(resultRows, row)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return &SQLResponse{
		Columns:   columns,
		Rows:      resultRows,
		RowCount:  len(resultRows),
		Truncated: truncated,
	}, nil
}
