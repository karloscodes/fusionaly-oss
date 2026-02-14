---
name: fusionaly-agent-api
description: Query Fusionaly analytics data via SQL
---

# Fusionaly Agent API

Query your Fusionaly analytics data using SQL.

## Environment Variables

Requires these environment variables to be set:
- `FUSIONALY_HOST` - Your Fusionaly instance URL (e.g., `https://analytics.example.com`)
- `FUSIONALY_API_KEY` - Your Agent API key from Administration → Agents

## Workflow

1. **First, fetch the schema** to understand available tables and columns:

```bash
curl -s -H "Authorization: Bearer $FUSIONALY_API_KEY" "$FUSIONALY_HOST/z/api/v1/schema"
```

2. **Then, execute SQL queries** against the data:

```bash
curl -s -X POST "$FUSIONALY_HOST/z/api/v1/sql" \
  -H "Authorization: Bearer $FUSIONALY_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"sql": "SELECT ...", "website_id": 1}'
```

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
