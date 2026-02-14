---
name: fusionaly-agent-api
description: Query Fusionaly analytics data via SQL
---

# Fusionaly Agent API

Query your Fusionaly analytics data using SQL.

**Your role:** The user will ask questions in plain English (e.g., "What were my top pages last week?" or "How many visitors did I get from Google?"). You translate these questions into SQL queries, execute them via the API, and present the results in the most readable format possible (tables, summaries, bullet points, or charts as appropriate).

## Environment Variables

Requires these environment variables to be set:
- `FUSIONALY_HOST` - Your Fusionaly instance URL (e.g., `https://analytics.example.com`)
- `FUSIONALY_API_KEY` - Your Agent API key from Administration → Agents

## Workflow

### Step 1: Fetch and Interpret the Schema

```bash
curl -s -H "Authorization: Bearer $FUSIONALY_API_KEY" "$FUSIONALY_HOST/z/api/v1/schema"
```

The response contains:
- **`schema`**: Raw SQLite CREATE TABLE statements showing all tables and columns
- **`concepts`**: Domain-specific explanations of what each table/column means

**How to use the schema:**
1. Parse the CREATE TABLE statements to learn exact table names and column names
2. Use the `concepts` section to understand what each column represents
3. Build your SQL queries using only the columns that exist in the schema
4. Always use the exact column names as shown (case-sensitive)

### Step 2: Execute SQL Queries

```bash
curl -s -X POST "$FUSIONALY_HOST/z/api/v1/sql" \
  -H "Authorization: Bearer $FUSIONALY_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"sql": "SELECT ...", "website_id": 1}'
```

Build your SELECT queries based on:
- Table names from the schema (e.g., `events`, `sessions`, `visitors`)
- Column names exactly as defined in CREATE TABLE statements
- Relationships implied by foreign keys (e.g., `website_id`, `visitor_id`)

## Constraints

- **Read-only**: Only SELECT queries are allowed
- **Rate limited**: 30 requests per minute
- **Query timeout**: 5 seconds
- **No comments**: SQL comments (`--` or `/**/`) are not allowed

## Domain Concepts

### Visitors vs Sessions
- **Visitor**: A unique user identified by a hash signature (no cookies, privacy-first)
- **Session**: A browsing session, expires after 30 minutes of inactivity

### Events
- **Pageview**: A page visit with path, referrer, UTM parameters
- **Custom Event**: User-defined events with optional properties

### Time-based Analysis
- Data is typically queried by date ranges
- Use `DATE(timestamp)` for daily aggregations
- Timestamps are stored in UTC

### Website Context
- All queries should filter by `website_id`
- Pass `website_id` in the request body alongside the SQL query
