// Package agent provides the Agent API for external tools like Claude Code
// to query Fusionaly analytics data via SQL.
package agent

import (
	"context"
	"fmt"
	"regexp"
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
	SQL       string `json:"sql"`
	WebsiteID int    `json:"website_id"`
}

// SQLResponse is the response format for the SQL endpoint
type SQLResponse struct {
	Columns  []string        `json:"columns"`
	Rows     [][]interface{} `json:"rows"`
	RowCount int             `json:"row_count"`
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

// GetDatabaseSchema retrieves the schema from sqlite_master
func GetDatabaseSchema(db *gorm.DB) (string, error) {
	var schemas []string
	rows, err := db.Raw("SELECT sql FROM sqlite_master WHERE type='table'").Rows()
	if err != nil {
		return "", err
	}
	defer rows.Close()

	for rows.Next() {
		var schema string
		if err := rows.Scan(&schema); err != nil {
			return "", err
		}
		if schema != "" {
			schemas = append(schemas, schema)
		}
	}

	return strings.Join(schemas, ";\n") + ";", nil
}

// ValidateReadOnlyQuery checks if the SQL query is safe (read-only)
func ValidateReadOnlyQuery(sqlQuery string) error {
	// Reject queries with comments - no legitimate reason for agents to use them
	if strings.Contains(sqlQuery, "/*") || strings.Contains(sqlQuery, "--") {
		return fmt.Errorf("comments not allowed in queries")
	}

	// Reject multiple statements
	if strings.Count(sqlQuery, ";") > 1 {
		return fmt.Errorf("multiple statements not allowed")
	}

	// Remove string literals before keyword checking (replace with empty placeholder)
	// This prevents false positives on pathnames like '/delete-account'
	withoutStrings := regexp.MustCompile(`'[^']*'`).ReplaceAllString(sqlQuery, "''")

	// Normalize: lowercase and collapse whitespace
	normalized := strings.ToLower(withoutStrings)
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")
	normalized = strings.TrimSpace(normalized)

	// Must start with SELECT or WITH (CTEs are read-only)
	if !strings.HasPrefix(normalized, "select ") && !strings.HasPrefix(normalized, "with ") {
		return fmt.Errorf("only SELECT queries are allowed")
	}

	// Block dangerous keywords (word boundary check)
	dangerous := []string{
		"insert", "update", "delete", "drop", "alter", "create",
		"truncate", "replace", "grant", "revoke", "exec", "execute",
		"call", "pragma", "attach", "detach", "vacuum", "reindex",
		"load_extension", "writefile", "readfile",
	}

	for _, keyword := range dangerous {
		pattern := regexp.MustCompile(`\b` + keyword + `\b`)
		if pattern.MatchString(normalized) {
			return fmt.Errorf("dangerous operation not allowed: %s", keyword)
		}
	}

	return nil
}

// ExecuteQuery runs a validated SQL query and returns the results
func ExecuteQuery(ctx context.Context, db *gorm.DB, sqlQuery string, timeout time.Duration) (*SQLResponse, error) {
	// Validate first
	if err := ValidateReadOnlyQuery(sqlQuery); err != nil {
		return nil, err
	}

	// Create context with timeout
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Execute query
	rows, err := db.WithContext(queryCtx).Raw(sqlQuery).Rows()
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()

	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	// Scan results
	var resultRows [][]interface{}
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	for rows.Next() {
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
		Columns:  columns,
		Rows:     resultRows,
		RowCount: len(resultRows),
	}, nil
}
