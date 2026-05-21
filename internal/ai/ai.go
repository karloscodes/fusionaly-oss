// Package ai provides AI-powered analytics features for Fusionaly.
package ai

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"fusionaly/internal/settings"
)

// OpenAIRequest represents the request structure for OpenAI API
type OpenAIRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Temperature    float64         `json:"temperature,omitempty"`
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
}

// ResponseFormat for JSON mode
type ResponseFormat struct {
	Type string `json:"type"`
}

// AIQueryResponse is the structured response from OpenAI
type AIQueryResponse struct {
	SQL       string   `json:"sql"`
	QueryType string   `json:"query_type"`
	VegaSpec  any      `json:"vega_spec,omitempty"`
	Summary   string   `json:"summary,omitempty"`
	FollowUps []string `json:"follow_ups,omitempty"`
}

// InvestigationQuery represents a single query in an investigation
type InvestigationQuery struct {
	Title     string `json:"title"`
	SQL       string `json:"sql"`
	QueryType string `json:"query_type"`
	VegaSpec  any    `json:"vega_spec,omitempty"`
}

// AIInvestigationResponse is the response for multi-query investigations
type AIInvestigationResponse struct {
	Summary string               `json:"summary"`
	Queries []InvestigationQuery `json:"queries"`
}

// Message defines a single message in the OpenAI request
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIResponse represents the response from OpenAI API
type OpenAIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// RateLimit holds the rate limit information from the API response headers
type RateLimit struct {
	LimitRequests     int    `json:"limit_requests"`
	LimitTokens       int    `json:"limit_tokens"`
	RemainingRequests int    `json:"remaining_requests"`
	RemainingTokens   int    `json:"remaining_tokens"`
	ResetRequests     string `json:"reset_requests"`
	ResetTokens       string `json:"reset_tokens"`
}

// QueryResult represents the result of an AI-generated query
type QueryResult struct {
	Question  string                   `json:"question"`
	Query     string                   `json:"query"`
	Results   []map[string]interface{} `json:"results"`
	QueryType string                   `json:"query_type"`
	VegaSpec  string                   `json:"vega_spec,omitempty"`
	Summary   string                   `json:"summary,omitempty"`
	FollowUps []string                 `json:"follow_ups,omitempty"`
	WebsiteID uint                     `json:"website_id"`
}

// SavedQuery represents a saved AI query
type SavedQuery struct {
	ID           uint      `json:"id" gorm:"primarykey"`
	WebsiteID    *uint     `json:"website_id,omitempty" gorm:"index"`
	Title        string    `json:"title" gorm:"size:255;not null"`
	GeneratedSQL string    `json:"generated_sql" gorm:"type:text;not null"`
	QueryType    string    `json:"query_type" gorm:"size:50;default:TABLE"`
	VegaSpec     string    `json:"vega_spec,omitempty" gorm:"type:text"`
	Model        string    `json:"model" gorm:"size:50;default:'openai/gpt-4o-mini'"`
	Order        int       `json:"order" gorm:"default:0"`
	CreatedAt    time.Time `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"autoUpdateTime"`
}

// QueryType constants
const (
	TypeScalar       = "SCALAR"
	TypeTimeseries   = "TIMESERIES"
	TypeDistribution = "DISTRIBUTION"
	TypeTable        = "TABLE"
)

// CacheTTL is how long AI query results are cached (6 hours)
const CacheTTL = 6 * time.Hour

// AIQueryCache stores cached AI query results to avoid repeated API calls
type AIQueryCache struct {
	ID        uint      `json:"id" gorm:"primarykey"`
	CacheKey  string    `json:"cache_key" gorm:"uniqueIndex;size:64;not null"`
	WebsiteID int       `json:"website_id" gorm:"index;not null"`
	Question  string    `json:"question" gorm:"type:text;not null"`
	Model     string    `json:"model" gorm:"size:50;default:'openai/gpt-4o-mini'"`
	SQL       string    `json:"sql" gorm:"type:text;not null"`
	QueryType string    `json:"query_type" gorm:"size:50;not null"`
	VegaSpec  string    `json:"vega_spec,omitempty" gorm:"type:text"`
	Results   string    `json:"results" gorm:"type:text"` // JSON-encoded results
	ExpiresAt time.Time `json:"expires_at" gorm:"index;not null"`
	CreatedAt time.Time `json:"created_at" gorm:"autoCreateTime"`
}

// Available AI models for Ask AI. OpenRouter uses provider-prefixed model ids.
var AvailableModels = []string{"openai/gpt-4o-mini", "openai/gpt-4o", "anthropic/claude-3.5-sonnet"}

// DefaultModel is the default model for Ask AI
const DefaultModel = "openai/gpt-4o-mini"

// chatCompletionsURL builds the chat completions endpoint from the configured
// AI base URL. Works with OpenAI or any OpenAI-compatible API (e.g. OpenRouter).
func chatCompletionsURL(db *gorm.DB) string {
	return strings.TrimRight(settings.GetAIBaseURL(db), "/") + "/chat/completions"
}

// generateCacheKey creates a unique key for a question + website + model combination
func generateCacheKey(question string, websiteID int, model string) string {
	// Normalize question: lowercase, trim, collapse whitespace
	normalized := strings.ToLower(strings.TrimSpace(question))
	normalized = regexp.MustCompile(`\s+`).ReplaceAllString(normalized, " ")

	data := fmt.Sprintf("%s:%d:%s", normalized, websiteID, model)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// GetCachedQuery looks up a cached AI query result
func GetCachedQuery(db *gorm.DB, question string, websiteID int, model string) (*AIQueryCache, bool) {
	key := generateCacheKey(question, websiteID, model)

	var cache AIQueryCache
	err := db.Where("cache_key = ? AND expires_at > ?", key, time.Now()).First(&cache).Error
	if err != nil {
		return nil, false
	}

	return &cache, true
}

// SetCachedQuery stores an AI query result in cache
func SetCachedQuery(db *gorm.DB, question string, websiteID int, model, sql, queryType, vegaSpec string, results []map[string]interface{}) error {
	key := generateCacheKey(question, websiteID, model)

	// Encode results as JSON
	resultsJSON, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("failed to marshal results: %w", err)
	}

	cache := AIQueryCache{
		CacheKey:  key,
		WebsiteID: websiteID,
		Question:  question,
		Model:     model,
		SQL:       sql,
		QueryType: queryType,
		VegaSpec:  vegaSpec,
		Results:   string(resultsJSON),
		ExpiresAt: time.Now().Add(CacheTTL),
	}

	// Upsert: update if exists, create if not
	return db.Where("cache_key = ?", key).Assign(cache).FirstOrCreate(&cache).Error
}

// CleanExpiredCache removes expired cache entries (call periodically)
func CleanExpiredCache(db *gorm.DB) error {
	return db.Where("expires_at < ?", time.Now()).Delete(&AIQueryCache{}).Error
}

// AIQueryResult contains the full response from AI query generation
type AIQueryResult struct {
	SQL       string
	QueryType string
	VegaSpec  string
	Summary   string
	FollowUps []string
	RateLimit *RateLimit
}

// GetQueryFromOpenAI generates a SQL query from OpenAI based on a natural language query.
// It includes automatic retry with error feedback if the generated SQL fails to execute.
// Results are cached for 6 hours to avoid repeated API calls.
func GetQueryFromOpenAI(ctx context.Context, db *gorm.DB, query, openAIApiKey string, websiteID int, model string, logger *slog.Logger) (*AIQueryResult, error) {
	if openAIApiKey == "" {
		return nil, fmt.Errorf("OpenAI API key not configured")
	}

	// Validate and default model (falls back to a valid configured default)
	if model == "" {
		model = settings.GetAIModel(db)
	}

	// Check cache first
	if cached, found := GetCachedQuery(db, query, websiteID, model); found {
		if logger != nil {
			logger.Debug("AI query cache hit", slog.String("question", query), slog.String("model", model))
		}
		return &AIQueryResult{
			SQL:       cached.SQL,
			QueryType: cached.QueryType,
			VegaSpec:  cached.VegaSpec,
		}, nil
	}

	// Build the system prompt
	dbSchema, err := getDatabaseSchema(db)
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve database schema: %w", err)
	}

	systemPrompt := buildSystemPrompt(dbSchema, websiteID)

	// Build initial messages
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: query},
	}

	const maxRetries = 2
	var lastResult *AIQueryResult
	var lastError error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		result, err := callOpenAIForQuery(ctx, db, openAIApiKey, model, messages)
		if err != nil {
			return nil, err
		}

		lastResult = result

		// Try to execute the query to validate it
		_, execErr := ExecuteQuery(db, result.SQL, result.QueryType)
		if execErr == nil {
			// Query executed successfully
			return result, nil
		}

		// Query failed - if we have retries left, send error back to OpenAI
		lastError = execErr
		if attempt < maxRetries {
			if logger != nil {
				logger.Info("AI query failed, retrying with error feedback",
					slog.Int("attempt", attempt+1),
					slog.String("error", execErr.Error()))
			}

			// Add the failed attempt and error to messages for context
			messages = append(messages,
				Message{Role: "assistant", Content: fmt.Sprintf(`{"sql": %q, "query_type": %q}`, result.SQL, result.QueryType)},
				Message{Role: "user", Content: fmt.Sprintf("That query failed with error: %s\n\nPlease fix the SQL. Remember to only use columns that exist in the schema.", execErr.Error())},
			)
		}
	}

	// All retries exhausted - return the last result with the execution error
	if lastResult != nil {
		return lastResult, fmt.Errorf("query failed after %d attempts: %w", maxRetries+1, lastError)
	}
	return nil, fmt.Errorf("query failed after %d attempts: %w", maxRetries+1, lastError)
}

// InvestigationResult holds results for a single investigation query
type InvestigationResult struct {
	Title     string                   `json:"title"`
	SQL       string                   `json:"sql"`
	QueryType string                   `json:"query_type"`
	VegaSpec  string                   `json:"vega_spec,omitempty"`
	Results   []map[string]interface{} `json:"results,omitempty"`
	Error     string                   `json:"error,omitempty"`
}

// GetInvestigationFromOpenAI generates multiple SQL queries for a comprehensive investigation.
// Returns all queries upfront - no data is sent to OpenAI, only schema.
func GetInvestigationFromOpenAI(ctx context.Context, db *gorm.DB, question, openAIApiKey string, websiteID int, logger *slog.Logger) (string, []InvestigationResult, error) {
	if openAIApiKey == "" {
		return "", nil, fmt.Errorf("OpenAI API key not configured")
	}

	dbSchema, err := getDatabaseSchema(db)
	if err != nil {
		return "", nil, fmt.Errorf("failed to retrieve database schema: %w", err)
	}

	systemPrompt := buildInvestigationPrompt(dbSchema, websiteID, question)

	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: question},
	}

	// Call OpenAI
	openAIRequest := OpenAIRequest{
		Model:          settings.GetAIModel(db),
		Messages:       messages,
		ResponseFormat: &ResponseFormat{Type: "json_object"},
	}

	requestBody, err := json.Marshal(openAIRequest)
	if err != nil {
		return "", nil, fmt.Errorf("failed to marshal OpenAI request: %w", err)
	}

	client := &http.Client{Timeout: 90 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatCompletionsURL(db), bytes.NewBuffer(requestBody))
	if err != nil {
		return "", nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+openAIApiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("HTTP request to OpenAI failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		var errResponse OpenAIResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResponse); err == nil && errResponse.Error != nil {
			return "", nil, fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, errResponse.Error.Message)
		}
		return "", nil, fmt.Errorf("OpenAI API returned non-200 status: %d", resp.StatusCode)
	}

	var openAIResponse OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResponse); err != nil {
		return "", nil, fmt.Errorf("failed to decode OpenAI response: %w", err)
	}

	if len(openAIResponse.Choices) == 0 {
		return "", nil, fmt.Errorf("no choices in OpenAI response")
	}

	content := strings.TrimSpace(openAIResponse.Choices[0].Message.Content)

	var aiResp AIInvestigationResponse
	if err := json.Unmarshal([]byte(content), &aiResp); err != nil {
		return "", nil, fmt.Errorf("failed to parse investigation response: %w", err)
	}

	if len(aiResp.Queries) == 0 {
		return "", nil, fmt.Errorf("AI returned no investigation queries")
	}

	// Execute all queries locally
	results := make([]InvestigationResult, 0, len(aiResp.Queries))
	for _, q := range aiResp.Queries {
		result := InvestigationResult{
			Title:     q.Title,
			SQL:       q.SQL,
			QueryType: q.QueryType,
		}

		// Convert vega_spec to string if present
		if q.VegaSpec != nil {
			if vegaBytes, err := json.Marshal(q.VegaSpec); err == nil {
				result.VegaSpec = string(vegaBytes)
			}
		}

		// Validate and execute
		if err := ValidateReadOnlyQuery(q.SQL); err != nil {
			result.Error = fmt.Sprintf("invalid query: %s", err.Error())
			results = append(results, result)
			continue
		}

		queryResults, err := ExecuteQuery(db, q.SQL, q.QueryType)
		if err != nil {
			result.Error = err.Error()
			if logger != nil {
				logger.Warn("Investigation query failed", slog.String("title", q.Title), slog.Any("error", err))
			}
		} else {
			result.Results = queryResults
		}

		results = append(results, result)
	}

	return aiResp.Summary, results, nil
}

// QuestionIntent represents the type of analytics question
type QuestionIntent string

const (
	IntentDiagnosis   QuestionIntent = "diagnosis"   // Why, drop, problem, issue
	IntentComparison  QuestionIntent = "comparison"  // vs, compare, change, difference
	IntentDiscovery   QuestionIntent = "discovery"   // best, top, most, highest
	IntentTrend       QuestionIntent = "trend"       // over time, growth, pattern, history
	IntentGeneral     QuestionIntent = "general"     // catch-all
)

// detectQuestionIntent classifies the question to pick the right research strategy
func detectQuestionIntent(question string) QuestionIntent {
	q := strings.ToLower(question)

	// Diagnosis: investigating problems
	if strings.Contains(q, "why") || strings.Contains(q, "drop") ||
		strings.Contains(q, "problem") || strings.Contains(q, "issue") ||
		strings.Contains(q, "wrong") || strings.Contains(q, "decrease") ||
		strings.Contains(q, "lost") || strings.Contains(q, "losing") {
		return IntentDiagnosis
	}

	// Comparison: comparing things
	if strings.Contains(q, " vs ") || strings.Contains(q, "compare") ||
		strings.Contains(q, "versus") || strings.Contains(q, "difference") ||
		strings.Contains(q, "changed") || strings.Contains(q, "week over") ||
		strings.Contains(q, "month over") {
		return IntentComparison
	}

	// Discovery: finding the best/top
	if strings.Contains(q, "best") || strings.Contains(q, "top") ||
		strings.Contains(q, "most") || strings.Contains(q, "highest") ||
		strings.Contains(q, "popular") || strings.Contains(q, "performing") {
		return IntentDiscovery
	}

	// Trend: time-based patterns
	if strings.Contains(q, "over time") || strings.Contains(q, "trend") ||
		strings.Contains(q, "growth") || strings.Contains(q, "pattern") ||
		strings.Contains(q, "history") || strings.Contains(q, "last") {
		return IntentTrend
	}

	return IntentGeneral
}

// buildInvestigationPrompt creates a dynamic system prompt based on question intent
func buildInvestigationPrompt(dbSchema string, websiteID int, question string) string {
	intent := detectQuestionIntent(question)

	basePrompt := fmt.Sprintf(`You are a web analytics expert. Generate a focused investigation with 4-5 SQL queries.

SCHEMA:
%s

RULES:
- Generate SELECT-only SQLite queries
- Always filter website_id = %d
- Each query explores a different angle tailored to the question type
- NEVER specify colors in vega_spec - let the app's theme handle colors

CRITICAL - Table and Column names (use EXACTLY these):
- Table ref_stats: hostname (for referrer domain), pathname, visitors_count, page_views_count
- Table site_stats: visitors, page_views, sessions, bounce_count
- Table page_stats: pathname, visitors_count, page_views_count, entrances, exits
- Table country_stats: country (lowercase ISO codes: us, gb, de, fr, jp)
- NEVER use referrer_domain, referrer_stats, or pageviews - those don't exist

Return JSON:
{
  "summary": "Brief explanation of what this investigation explores",
  "queries": [
    {
      "title": "Short title",
      "sql": "SELECT ...",
      "query_type": "TABLE|SCALAR|DISTRIBUTION|TIMESERIES",
      "vega_spec": null or {"width":"container","mark":"bar","encoding":{...}}
    }
  ]
}

VEGA SPEC - CRITICAL:
- width: always "container"
- mark: "bar" or {"type":"line","point":true}
- encoding: x, y, tooltip only
- NEVER include "color", "fill", or "stroke" properties
- The app automatically applies brand colors`, dbSchema, websiteID)

	// Add intent-specific research strategy
	var strategy string
	switch intent {
	case IntentDiagnosis:
		strategy = `
INVESTIGATION STRATEGY (Diagnosis - finding root causes):
1. Period comparison - Compare recent vs previous period to confirm the problem
2. Source breakdown - Which traffic sources changed? (use ref_stats.hostname)
3. Page analysis - Which pages are affected? Entry pages, exit pages
4. Time pattern - When did it start? Daily trend to spot the inflection point
5. Geographic check - Any country-specific changes?

Focus on BEFORE vs AFTER comparisons. Help identify WHAT changed and WHEN.`

	case IntentComparison:
		strategy = `
INVESTIGATION STRATEGY (Comparison - side-by-side analysis):
1. Overall metrics comparison - Total visitors, pageviews, sessions for both periods
2. Source comparison - How traffic sources differ between periods
3. Page performance comparison - Which pages gained/lost
4. Day-of-week comparison - Different patterns by weekday?
5. Engagement comparison - Bounce rate, time on site changes

Show both periods side-by-side. Calculate differences and percentages.`

	case IntentDiscovery:
		strategy = `
INVESTIGATION STRATEGY (Discovery - finding what works best):
1. Top performers - Ranked list of best pages/sources by volume
2. Engagement leaders - Best by engagement metrics (low bounce, high duration)
3. Growth stars - What's growing fastest? Compare recent vs previous
4. Conversion analysis - If goals exist, what drives conversions?
5. Source quality - Which sources bring engaged visitors?

Focus on RANKINGS and RELATIVE performance. Highlight standouts.`

	case IntentTrend:
		strategy = `
INVESTIGATION STRATEGY (Trend - patterns over time):
1. Daily timeseries - Show the full trend line
2. Day-of-week pattern - Which days perform best?
3. Source evolution - How have traffic sources changed?
4. Page trends - Which pages are trending up/down?
5. Engagement trend - How is engagement changing over time?

Focus on TIMESERIES charts and PERIOD comparisons. Show direction of change.`

	default:
		strategy = `
INVESTIGATION STRATEGY (General exploration):
1. Overview metrics - Current totals and key stats
2. Traffic sources - Where visitors come from
3. Top content - Most visited pages
4. Time patterns - Recent daily trend
5. Audience breakdown - Countries, devices

Provide a well-rounded view of the current state.`
	}

	return basePrompt + strategy
}

// callOpenAIForQuery makes a single call to OpenAI to generate SQL
func callOpenAIForQuery(ctx context.Context, db *gorm.DB, openAIApiKey, model string, messages []Message) (*AIQueryResult, error) {
	// Prepare OpenAI request with JSON mode for reliable parsing
	openAIRequest := OpenAIRequest{
		Model:          model,
		Messages:       messages,
		ResponseFormat: &ResponseFormat{Type: "json_object"},
	}

	requestBody, err := json.Marshal(openAIRequest)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal OpenAI request: %w", err)
	}

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatCompletionsURL(db), bytes.NewBuffer(requestBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+openAIApiKey)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request to OpenAI failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Parse rate limit headers
	rateLimit := &RateLimit{}
	if val := resp.Header.Get("x-ratelimit-limit-requests"); val != "" {
		if v, err := strconv.Atoi(val); err == nil {
			rateLimit.LimitRequests = v
		}
	}
	if val := resp.Header.Get("x-ratelimit-remaining-requests"); val != "" {
		if v, err := strconv.Atoi(val); err == nil {
			rateLimit.RemainingRequests = v
		}
	}

	if resp.StatusCode != http.StatusOK {
		// Read response body for error details
		var errResponse OpenAIResponse
		if err := json.NewDecoder(resp.Body).Decode(&errResponse); err == nil && errResponse.Error != nil {
			return nil, fmt.Errorf("OpenAI API error (%d): %s", resp.StatusCode, errResponse.Error.Message)
		}
		return nil, fmt.Errorf("OpenAI API returned non-200 status: %d", resp.StatusCode)
	}

	var openAIResponse OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResponse); err != nil {
		return nil, fmt.Errorf("failed to decode OpenAI response: %w", err)
	}

	if openAIResponse.Error != nil {
		return nil, fmt.Errorf("OpenAI API error: %s", openAIResponse.Error.Message)
	}

	if len(openAIResponse.Choices) == 0 {
		return nil, fmt.Errorf("no choices in OpenAI response")
	}

	// Parse JSON response directly (JSON mode guarantees valid JSON)
	content := strings.TrimSpace(openAIResponse.Choices[0].Message.Content)

	var aiResp AIQueryResponse
	if err := json.Unmarshal([]byte(content), &aiResp); err != nil {
		return nil, fmt.Errorf("failed to parse AI response: %w", err)
	}

	if aiResp.SQL == "" {
		return nil, fmt.Errorf("AI response did not contain SQL")
	}

	// Validate that the query is read-only
	if err := ValidateReadOnlyQuery(aiResp.SQL); err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	// Convert vega_spec to JSON string if present
	var vegaSpec string
	if aiResp.VegaSpec != nil {
		vegaBytes, err := json.Marshal(aiResp.VegaSpec)
		if err == nil {
			vegaSpec = string(vegaBytes)
		}
	}

	// Default to TABLE if no query type specified
	queryType := aiResp.QueryType
	if queryType == "" {
		queryType = TypeTable
	}

	return &AIQueryResult{
		SQL:       aiResp.SQL,
		QueryType: queryType,
		VegaSpec:  vegaSpec,
		Summary:   aiResp.Summary,
		FollowUps: aiResp.FollowUps,
		RateLimit: rateLimit,
	}, nil
}

// buildSystemPrompt creates the system prompt for SQL generation
func buildSystemPrompt(dbSchema string, websiteID int) string {
	return fmt.Sprintf(`You are a web analytics expert. Generate SQLite queries and provide actionable insights.

SCHEMA:
%s

Return JSON with this structure:
{
  "sql": "SELECT ...",
  "query_type": "TABLE|SCALAR|DISTRIBUTION|TIMESERIES",
  "vega_spec": null or {...},
  "summary": "One sentence verdict about what this data means and whether action is needed",
  "follow_ups": ["Follow-up question 1?", "Follow-up question 2?", "Follow-up question 3?"]
}

RULES:
- Always filter website_id = %d
- For day-of-week questions: ALWAYS return ALL 7 days using a CTE, never LIMIT 1

SUMMARY RULES - Be opinionated and direct:
- Start with a verdict: "Good news:", "Problem:", "Steady:", "Worth noting:"
- Include specific numbers from the query results context
- Tell them if action is needed or not
- Max 30 words, no jargon

FOLLOW-UP RULES:
- Suggest 2-3 natural follow-up questions based on what they asked
- Questions should dig deeper or explore related angles
- Write them as the user would ask them (plain English)
- Make them specific to the topic, not generic

CRITICAL - Table and Column names (use EXACTLY these):
- Table ref_stats: hostname (referrer domain), pathname, visitors_count, page_views_count
- Table site_stats: visitors, page_views, sessions, bounce_count
- Table page_stats: pathname, visitors_count, page_views_count, entrances, exits
- Table country_stats: country (lowercase ISO codes like "us", "gb", "de")
- NEVER use referrer_domain or referrer_stats - those don't exist

COUNTRY CODES:
- "United States"/"America" → 'us', "UK"/"Britain" → 'gb', "Germany" → 'de', etc.

VEGA SPEC RULES:
- Always include "width":"container"
- Do NOT specify colors
- Include tooltips

EXAMPLES:

Q: "Where is my traffic coming from?"
{
  "sql": "SELECT hostname AS name, SUM(visitors_count) AS value FROM ref_stats WHERE website_id = %d AND hour >= datetime('now', '-30 days') AND hostname != '' GROUP BY hostname ORDER BY value DESC LIMIT 15",
  "query_type": "DISTRIBUTION",
  "vega_spec": {"width":"container","mark":"bar","encoding":{"x":{"field":"name","type":"nominal","sort":"-y"},"y":{"field":"value","type":"quantitative","title":"Visitors"},"tooltip":[{"field":"name","type":"nominal","title":"Source"},{"field":"value","type":"quantitative","title":"Visitors"}]}},
  "summary": "Your top traffic source is [X] with [N] visitors. Direct traffic is [Y]%% of total.",
  "follow_ups": ["Which sources have the best engagement?", "How has my traffic mix changed over time?", "What pages do Google visitors land on?"]
}

Q: "Compare this week vs last week"
{
  "sql": "SELECT SUM(CASE WHEN hour >= datetime('now', '-7 days') THEN visitors ELSE 0 END) AS this_week, SUM(CASE WHEN hour < datetime('now', '-7 days') THEN visitors ELSE 0 END) AS last_week FROM site_stats WHERE website_id = %d AND hour >= datetime('now', '-14 days')",
  "query_type": "TABLE",
  "vega_spec": null,
  "summary": "Steady: Traffic is roughly the same as last week. No action needed.",
  "follow_ups": ["Which days had the biggest changes?", "Did any traffic sources change significantly?", "Which pages gained or lost visitors?"]
}

Q: "Top 10 pages"
{
  "sql": "SELECT pathname AS name, SUM(visitors_count) AS value FROM page_stats WHERE website_id = %d AND hour >= datetime('now', '-30 days') GROUP BY pathname ORDER BY value DESC LIMIT 10",
  "query_type": "DISTRIBUTION",
  "vega_spec": {"width":"container","mark":"bar","encoding":{"x":{"field":"name","type":"nominal","sort":"-y"},"y":{"field":"value","type":"quantitative","title":"Visitors"},"tooltip":[{"field":"name","type":"nominal","title":"Page"},{"field":"value","type":"quantitative","title":"Visitors"}]}},
  "summary": "Your homepage dominates at [X]%% of traffic. The top 3 pages account for [Y]%% of all visits.",
  "follow_ups": ["Which pages are trending up?", "What's the bounce rate on my top pages?", "Where do visitors go after the homepage?"]
}`, dbSchema, websiteID, websiteID, websiteID, websiteID)
}

// getDatabaseSchema retrieves the schema of the database
func getDatabaseSchema(db *gorm.DB) (string, error) {
	var schemas []string
	rows, err := db.Raw("SELECT sql FROM sqlite_master WHERE type='table'").Rows()
	if err != nil {
		return "", err
	}
	defer func() { _ = rows.Close() }()

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

// dangerousSQLKeywords matches write/DDL statements and the load_extension
// function on word boundaries, so tab/newline-separated bypasses (e.g.
// "delete\tfrom") are caught, not just space-separated ones.
var dangerousSQLKeywords = regexp.MustCompile(`\b(insert|update|delete|drop|alter|create|truncate|replace|grant|revoke|exec|execute|call|pragma|attach|detach|vacuum|reindex|load_extension)\b`)

// ValidateReadOnlyQuery rejects anything that isn't a single read-only SELECT.
// The SQLite driver will happily run a multi-statement string, and AI-generated
// SQL is not trusted, so this is the security boundary for query execution.
//
// Strategy: strip comments, forbid multiple statements, require the query to
// begin with SELECT or a WITH (CTE), and — as defense in depth — reject
// write/DDL keywords found OUTSIDE of string literals (so a legitimate
// "WHERE pathname LIKE '%/delete%'" is allowed).
func ValidateReadOnlyQuery(sqlQuery string) error {
	// Strip block (incl. multi-line) and line comments.
	query := regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(sqlQuery, " ")
	query = regexp.MustCompile(`--[^\n]*`).ReplaceAllString(query, " ")

	// Only a single statement is allowed: at most one trailing semicolon.
	if strings.Contains(strings.TrimRight(query, " \t\r\n;"), ";") {
		return fmt.Errorf("multiple statements are not allowed")
	}

	// Remove string literals before keyword scanning so data containing a
	// keyword (e.g. a "/create" URL path) doesn't trip the denylist.
	stripped := regexp.MustCompile(`'(?:[^']|'')*'`).ReplaceAllString(query, " ")
	stripped = regexp.MustCompile(`"(?:[^"]|"")*"`).ReplaceAllString(stripped, " ")
	lower := strings.ToLower(strings.TrimSpace(stripped))

	// A read-only query must begin with SELECT or a WITH (CTE).
	if !strings.HasPrefix(lower, "select") && !strings.HasPrefix(lower, "with") {
		return fmt.Errorf("only SELECT queries are allowed")
	}

	if m := dangerousSQLKeywords.FindString(lower); m != "" {
		return fmt.Errorf("dangerous operation detected: %s", m)
	}

	return nil
}

// ExtractTypeFromComment extracts query type from SQL comment
func ExtractTypeFromComment(query string) string {
	lines := strings.Split(query, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "-- QUERY_TYPE:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				typeStr := strings.TrimSpace(parts[1])
				if spaceIndex := strings.Index(typeStr, " "); spaceIndex > 0 {
					typeStr = typeStr[:spaceIndex]
				}
				return typeStr
			}
		}
	}
	return TypeTable
}

// ExecuteQuery executes a SQL query based on the provided query type
func ExecuteQuery(db *gorm.DB, query string, queryType ...string) ([]map[string]interface{}, error) {
	if db == nil {
		return nil, fmt.Errorf("database connection is nil")
	}
	if query == "" {
		return nil, fmt.Errorf("query string is empty")
	}

	if err := ValidateReadOnlyQuery(query); err != nil {
		return nil, fmt.Errorf("invalid query: %w", err)
	}

	var detectedType string
	if len(queryType) > 0 && queryType[0] != "" {
		detectedType = queryType[0]
	} else {
		detectedType = ExtractTypeFromComment(query)
	}

	switch strings.ToUpper(detectedType) {
	case TypeScalar:
		return executeScalarQuery(db, query)
	default:
		return executeStandardQuery(db, query)
	}
}

func executeScalarQuery(db *gorm.DB, query string) ([]map[string]interface{}, error) {
	rows, err := db.Raw(query).Rows()
	if err != nil {
		return nil, fmt.Errorf("scalar query execution failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	if !rows.Next() {
		result := make(map[string]interface{})
		for _, col := range columns {
			result[col] = 0
		}
		return []map[string]interface{}{result}, nil
	}

	if err := rows.Scan(valuePtrs...); err != nil {
		return nil, fmt.Errorf("scalar row scan failed: %w", err)
	}

	rowMap := make(map[string]interface{})
	for i, col := range columns {
		val := values[i]
		if val == nil {
			rowMap[col] = 0
			continue
		}
		if b, ok := val.([]byte); ok {
			rowMap[col] = string(b)
		} else {
			rowMap[col] = val
		}
	}

	return []map[string]interface{}{rowMap}, nil
}

func executeStandardQuery(db *gorm.DB, query string) ([]map[string]interface{}, error) {
	rows, err := db.Raw(query).Rows()
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	// Always return empty slice, never nil
	results := make([]map[string]interface{}, 0)
	values := make([]interface{}, len(columns))
	valuePtrs := make([]interface{}, len(columns))
	for i := range columns {
		valuePtrs[i] = &values[i]
	}

	for rows.Next() {
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("row scan failed: %w", err)
		}

		rowMap := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			if val == nil {
				rowMap[col] = nil
				continue
			}
			if b, ok := val.([]byte); ok {
				rowMap[col] = string(b)
			} else {
				rowMap[col] = val
			}
		}
		results = append(results, rowMap)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	return results, nil
}

// === SavedQuery CRUD Operations ===

// GetSavedQueriesByWebsiteID retrieves saved queries filtered by website ID
func GetSavedQueriesByWebsiteID(db *gorm.DB, websiteID uint) ([]SavedQuery, error) {
	// Always return empty slice, never nil (frontend expects array)
	queries := make([]SavedQuery, 0)
	err := db.Where("website_id = ?", websiteID).Order("\"order\" ASC").Find(&queries).Error
	return queries, err
}

// CreateSavedQueryWithVega creates a new saved query with vega spec
func CreateSavedQueryWithVega(db *gorm.DB, title, generatedSQL, vegaSpec string, websiteID *uint, queryType, model string) (*SavedQuery, error) {
	if model == "" {
		model = DefaultModel
	}
	query := SavedQuery{
		Title:        title,
		GeneratedSQL: generatedSQL,
		VegaSpec:     vegaSpec,
		WebsiteID:    websiteID,
		QueryType:    queryType,
		Model:        model,
		Order:        1,
	}

	// Increment order of all existing queries to make room at the top
	if err := db.Model(&SavedQuery{}).Where("1=1").Update("order", gorm.Expr("\"order\" + 1")).Error; err != nil {
		return nil, err
	}

	if err := db.Create(&query).Error; err != nil {
		return nil, err
	}

	return &query, nil
}

// UpdateSavedQueryWithWebsiteAndVega updates an existing saved query
func UpdateSavedQueryWithWebsiteAndVega(db *gorm.DB, queryID uint, title, generatedSQL, queryType, vegaSpec, model string, websiteID *uint) error {
	if model == "" {
		model = DefaultModel
	}
	updates := map[string]interface{}{
		"title":         title,
		"generated_sql": generatedSQL,
		"query_type":    queryType,
		"vega_spec":     vegaSpec,
		"model":         model,
		"updated_at":    time.Now(),
	}

	if websiteID == nil {
		updates["website_id"] = nil
	} else {
		updates["website_id"] = *websiteID
	}

	return db.Model(&SavedQuery{}).Where("id = ?", queryID).Updates(updates).Error
}

// GetSavedQuery retrieves a single saved query by ID
func GetSavedQuery(db *gorm.DB, queryID uint) (*SavedQuery, error) {
	var query SavedQuery
	if err := db.Where("id = ?", queryID).First(&query).Error; err != nil {
		return nil, err
	}
	return &query, nil
}

// CloneSavedQuery creates a copy of an existing saved query
func CloneSavedQuery(db *gorm.DB, queryID uint) (*SavedQuery, error) {
	var originalQuery SavedQuery
	if err := db.Where("id = ?", queryID).First(&originalQuery).Error; err != nil {
		return nil, err
	}

	clonedQuery := SavedQuery{
		Title:        originalQuery.Title + " (Copy)",
		GeneratedSQL: originalQuery.GeneratedSQL,
		WebsiteID:    originalQuery.WebsiteID,
		QueryType:    originalQuery.QueryType,
		VegaSpec:     originalQuery.VegaSpec,
		Model:        originalQuery.Model,
		Order:        1,
	}

	// Increment order of all existing queries
	if err := db.Model(&SavedQuery{}).Where("1=1").Update("order", gorm.Expr("\"order\" + 1")).Error; err != nil {
		return nil, err
	}

	if err := db.Create(&clonedQuery).Error; err != nil {
		return nil, err
	}

	return &clonedQuery, nil
}

// DeleteSavedQuery removes a saved query
func DeleteSavedQuery(db *gorm.DB, queryID uint) error {
	return db.Where("id = ?", queryID).Delete(&SavedQuery{}).Error
}

// GetOpenAIApiKey retrieves the OpenAI API key from settings
func GetOpenAIApiKey(db *gorm.DB) (string, error) {
	return settings.GetOpenAIKey(db)
}

// GetSummaryFromOpenAI generates a plain English summary of query results
func GetSummaryFromOpenAI(ctx context.Context, db *gorm.DB, question string, results []map[string]interface{}, openAIApiKey string) (string, error) {
	if openAIApiKey == "" {
		return "", fmt.Errorf("OpenAI API key not configured")
	}

	if len(results) == 0 {
		return "No data found for your query.", nil
	}

	// Limit results to avoid token overflow (first 20 rows)
	limitedResults := results
	if len(results) > 20 {
		limitedResults = results[:20]
	}

	resultsJSON, err := json.Marshal(limitedResults)
	if err != nil {
		return "", fmt.Errorf("failed to marshal results: %w", err)
	}

	systemPrompt := `You are a data analyst explaining query results to a non-technical user.

Rules:
- Give a 1-2 sentence summary of what the data shows
- Be specific with numbers (use the actual values)
- Highlight the key insight or trend
- Use plain English, no jargon
- Don't explain what the query did, explain what the data means
- If there's a clear winner/loser or trend, mention it
- Be concise - max 50 words`

	userPrompt := fmt.Sprintf("Question: %s\n\nData:\n%s\n\nSummarize what this data tells us.", question, string(resultsJSON))

	openAIRequest := OpenAIRequest{
		Model: settings.GetAIModel(db),
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.3,
	}

	requestBody, err := json.Marshal(openAIRequest)
	if err != nil {
		return "", fmt.Errorf("failed to marshal OpenAI request: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, chatCompletionsURL(db), bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+openAIApiKey)

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("HTTP request to OpenAI failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OpenAI API returned non-200 status: %d", resp.StatusCode)
	}

	var openAIResponse OpenAIResponse
	if err := json.NewDecoder(resp.Body).Decode(&openAIResponse); err != nil {
		return "", fmt.Errorf("failed to decode OpenAI response: %w", err)
	}

	if openAIResponse.Error != nil {
		return "", fmt.Errorf("OpenAI API error: %s", openAIResponse.Error.Message)
	}

	if len(openAIResponse.Choices) == 0 {
		return "", fmt.Errorf("no choices in OpenAI response")
	}

	return strings.TrimSpace(openAIResponse.Choices[0].Message.Content), nil
}
