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
	Tables   []TableSchema     `json:"tables"`
	Concepts map[string]string `json:"concepts"`
	Examples []QueryExample    `json:"examples"`
}

// TableSchema describes a database table
type TableSchema struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Columns     []ColumnSchema `json:"columns"`
}

// ColumnSchema describes a column in a table
type ColumnSchema struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
}

// QueryExample provides example SQL queries
type QueryExample struct {
	Question string `json:"question"`
	SQL      string `json:"sql"`
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

// GetSchema returns the database schema with descriptions and examples
func GetSchema() *SchemaResponse {
	return &SchemaResponse{
		Tables: []TableSchema{
			{
				Name:        "site_stats",
				Description: "Aggregated site-wide statistics by hour. Use this for total visitors, pageviews, sessions.",
				Columns: []ColumnSchema{
					{Name: "website_id", Type: "integer", Description: "Website identifier (always filter by this)"},
					{Name: "hour", Type: "datetime", Description: "Hour bucket in UTC"},
					{Name: "visitors", Type: "integer", Description: "Unique visitors in this hour"},
					{Name: "page_views", Type: "integer", Description: "Total page views in this hour"},
					{Name: "sessions", Type: "integer", Description: "Number of sessions started"},
					{Name: "bounce_count", Type: "integer", Description: "Sessions with only one pageview"},
				},
			},
			{
				Name:        "page_stats",
				Description: "Aggregated statistics per page (URL path). Use for top pages, entry/exit pages.",
				Columns: []ColumnSchema{
					{Name: "website_id", Type: "integer", Description: "Website identifier"},
					{Name: "hour", Type: "datetime", Description: "Hour bucket in UTC"},
					{Name: "hostname", Type: "string", Description: "Domain name"},
					{Name: "pathname", Type: "string", Description: "URL path without domain (e.g., /pricing)"},
					{Name: "visitors_count", Type: "integer", Description: "Unique visitors to this page"},
					{Name: "page_views_count", Type: "integer", Description: "Total views of this page"},
					{Name: "entrances", Type: "integer", Description: "Sessions that started on this page"},
					{Name: "exits", Type: "integer", Description: "Sessions that ended on this page"},
				},
			},
			{
				Name:        "ref_stats",
				Description: "Referrer statistics. Use for traffic sources analysis.",
				Columns: []ColumnSchema{
					{Name: "website_id", Type: "integer", Description: "Website identifier"},
					{Name: "hour", Type: "datetime", Description: "Hour bucket in UTC"},
					{Name: "hostname", Type: "string", Description: "Referrer domain (e.g., google.com, twitter.com)"},
					{Name: "pathname", Type: "string", Description: "Referrer path (usually empty for search engines)"},
					{Name: "visitors_count", Type: "integer", Description: "Visitors from this referrer"},
					{Name: "page_views_count", Type: "integer", Description: "Page views from this referrer"},
				},
			},
			{
				Name:        "country_stats",
				Description: "Visitor statistics by country.",
				Columns: []ColumnSchema{
					{Name: "website_id", Type: "integer", Description: "Website identifier"},
					{Name: "hour", Type: "datetime", Description: "Hour bucket in UTC"},
					{Name: "country", Type: "string", Description: "ISO country code lowercase (us, gb, de, fr, jp)"},
					{Name: "visitors_count", Type: "integer", Description: "Visitors from this country"},
					{Name: "page_views_count", Type: "integer", Description: "Page views from this country"},
				},
			},
			{
				Name:        "browser_stats",
				Description: "Visitor statistics by browser.",
				Columns: []ColumnSchema{
					{Name: "website_id", Type: "integer", Description: "Website identifier"},
					{Name: "hour", Type: "datetime", Description: "Hour bucket in UTC"},
					{Name: "browser", Type: "string", Description: "Browser name (Chrome, Firefox, Safari, etc.)"},
					{Name: "visitors_count", Type: "integer", Description: "Visitors using this browser"},
					{Name: "page_views_count", Type: "integer", Description: "Page views from this browser"},
				},
			},
			{
				Name:        "os_stats",
				Description: "Visitor statistics by operating system.",
				Columns: []ColumnSchema{
					{Name: "website_id", Type: "integer", Description: "Website identifier"},
					{Name: "hour", Type: "datetime", Description: "Hour bucket in UTC"},
					{Name: "operating_system", Type: "string", Description: "OS name (Windows, macOS, iOS, Android, Linux)"},
					{Name: "visitors_count", Type: "integer", Description: "Visitors using this OS"},
					{Name: "page_views_count", Type: "integer", Description: "Page views from this OS"},
				},
			},
			{
				Name:        "device_stats",
				Description: "Visitor statistics by device type.",
				Columns: []ColumnSchema{
					{Name: "website_id", Type: "integer", Description: "Website identifier"},
					{Name: "hour", Type: "datetime", Description: "Hour bucket in UTC"},
					{Name: "device_type", Type: "string", Description: "Device type (desktop, mobile, tablet)"},
					{Name: "visitors_count", Type: "integer", Description: "Visitors using this device type"},
					{Name: "page_views_count", Type: "integer", Description: "Page views from this device type"},
				},
			},
			{
				Name:        "utm_stats",
				Description: "UTM campaign parameter statistics.",
				Columns: []ColumnSchema{
					{Name: "website_id", Type: "integer", Description: "Website identifier"},
					{Name: "hour", Type: "datetime", Description: "Hour bucket in UTC"},
					{Name: "utm_source", Type: "string", Description: "Campaign source (google, newsletter)"},
					{Name: "utm_medium", Type: "string", Description: "Campaign medium (cpc, email)"},
					{Name: "utm_campaign", Type: "string", Description: "Campaign name"},
					{Name: "utm_term", Type: "string", Description: "Paid search term"},
					{Name: "utm_content", Type: "string", Description: "Ad content identifier"},
					{Name: "visitors_count", Type: "integer", Description: "Visitors from this campaign"},
					{Name: "page_views_count", Type: "integer", Description: "Page views from this campaign"},
				},
			},
			{
				Name:        "event_stats",
				Description: "Custom event statistics (conversions, clicks, etc.).",
				Columns: []ColumnSchema{
					{Name: "website_id", Type: "integer", Description: "Website identifier"},
					{Name: "hour", Type: "datetime", Description: "Hour bucket in UTC"},
					{Name: "event_name", Type: "string", Description: "Event name (signup, purchase, download)"},
					{Name: "event_key", Type: "string", Description: "Optional event key/value"},
					{Name: "visitors_count", Type: "integer", Description: "Unique visitors who triggered this event"},
					{Name: "page_views_count", Type: "integer", Description: "Times this event was triggered"},
				},
			},
		},
		Concepts: map[string]string{
			"time_filtering":      "Always filter by 'hour' column using datetime('now', '-7 days') or similar. Data is aggregated hourly in UTC.",
			"website_scoping":     "ALWAYS include 'website_id = ?' in WHERE clause. Data is multi-tenant.",
			"visitors_vs_views":   "visitors_count = unique users, page_views_count/page_views = total actions. Use visitors for audience size, views for engagement.",
			"country_codes":       "Countries use lowercase ISO codes: 'us' (USA), 'gb' (UK), 'de' (Germany), 'fr' (France), etc.",
			"direct_traffic":      "Direct traffic has empty hostname in ref_stats. Filter with hostname = '' or hostname != ''.",
			"bounce_rate":         "Calculate as: (bounce_count * 100.0 / sessions) from site_stats.",
			"aggregation_pattern": "Always SUM() the count columns when grouping. Raw values are hourly increments.",
		},
		Examples: []QueryExample{
			{
				Question: "How many visitors did I get this week?",
				SQL:      "SELECT SUM(visitors) as total_visitors FROM site_stats WHERE website_id = 1 AND hour >= datetime('now', '-7 days')",
			},
			{
				Question: "What are my top 10 pages?",
				SQL:      "SELECT pathname, SUM(visitors_count) as visitors FROM page_stats WHERE website_id = 1 AND hour >= datetime('now', '-30 days') GROUP BY pathname ORDER BY visitors DESC LIMIT 10",
			},
			{
				Question: "Where is my traffic coming from?",
				SQL:      "SELECT hostname as source, SUM(visitors_count) as visitors FROM ref_stats WHERE website_id = 1 AND hour >= datetime('now', '-30 days') AND hostname != '' GROUP BY hostname ORDER BY visitors DESC LIMIT 10",
			},
			{
				Question: "Daily visitors trend for the last 7 days",
				SQL:      "SELECT date(hour) as day, SUM(visitors) as visitors FROM site_stats WHERE website_id = 1 AND hour >= datetime('now', '-7 days') GROUP BY date(hour) ORDER BY day",
			},
			{
				Question: "Top countries by visitors",
				SQL:      "SELECT country, SUM(visitors_count) as visitors FROM country_stats WHERE website_id = 1 AND hour >= datetime('now', '-30 days') GROUP BY country ORDER BY visitors DESC LIMIT 10",
			},
		},
	}
}

// ValidateReadOnlyQuery checks if the SQL query is safe (read-only)
func ValidateReadOnlyQuery(sqlQuery string) error {
	// Remove comments
	query := sqlQuery
	query = regexp.MustCompile(`--.*?\n`).ReplaceAllString(query, "\n")
	query = regexp.MustCompile(`/\*.*?\*/`).ReplaceAllString(query, "")
	query = strings.ToLower(strings.TrimSpace(query))

	// Must start with SELECT
	if !strings.HasPrefix(query, "select") {
		return fmt.Errorf("only SELECT queries are allowed")
	}

	// Block dangerous keywords
	dangerous := []string{
		"insert ", "update ", "delete ", "drop ", "alter ", "create ",
		"truncate ", "replace ", "grant ", "revoke ", "exec ", "execute ",
		"call ", "pragma ", "attach ", "detach ", "vacuum ", "reindex ",
	}

	for _, keyword := range dangerous {
		if strings.Contains(query, keyword) {
			return fmt.Errorf("dangerous operation not allowed: %s", strings.TrimSpace(keyword))
		}
	}

	// Block multiple statements
	if strings.Count(query, ";") > 1 {
		return fmt.Errorf("multiple statements not allowed")
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
