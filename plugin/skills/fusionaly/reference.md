# Fusionaly reference

`get_schema` is the source of truth. This page explains the tables you use most and gives queries that work.

## Tables

Every `*_stats` table has `website_id`, `hour` (the start of a 30-minute UTC bucket), and count columns you `SUM()`.

| Table | Columns you use | Notes |
|-------|-----------------|-------|
| `site_stats` | `visitors`, `page_views`, `sessions`, `bounce_count` | Site totals. Bounce rate: `SUM(bounce_count) * 100.0 / SUM(sessions)`. |
| `page_stats` | `pathname`, `visitors_count`, `page_views_count`, `entrances`, `exits` | Per page. Entry pages: rank by `entrances`. |
| `ref_stats` | `hostname`, `pathname`, `visitors_count`, `page_views_count` | Referrers. `hostname` is the referrer domain; `__direct_or_unknown__` means direct. |
| `country_stats` | `country`, `visitors_count` | Lowercase ISO codes: `us`, `gb`, `de`, `fr`. |
| `device_stats` | `device_type`, `visitors_count` | |
| `browser_stats` | `browser`, `visitors_count` | |
| `os_stats` | `operating_system`, `visitors_count` | |
| `utm_stats` | `utm_source`, `utm_medium`, `utm_campaign`, `utm_term`, `utm_content`, `visitors_count` | Campaigns. |
| `query_param_stats` | `param_name`, `param_value`, `visitors_count` | Other query parameters. |
| `event_stats` | `event_name`, `event_key`, `visitors_count` | Custom events and goals, per 30 minutes. |
| `flow_transition_stats` | `step_position`, `source_page`, `target_page`, `transitions` | User flows: page A to page B. |
| `events` | `pathname`, `referrer_hostname`, `event_type`, `custom_event_name`, `custom_event_meta`, `timestamp` | Raw events. `event_type` 1 is a page view, 2 is a custom event, 3 is a page hide (only marks when a visitor left a page; leave it out of counts). Use it for revenue and event metadata; prefer the stats tables otherwise. |
| `websites` | `id`, `domain` | |
| `annotations` | `title`, `annotation_date` | Deploys, campaigns, incidents marked on the timeline. |
| `feed_items` | `item_type`, `title`, `description`, `period_start` | What `whats_new` returns. |

User accounts, settings, and keys are not readable. A query that names them fails with "table not allowed".

## Queries that work

Replace `1` with the `website_id` from `list_websites`.

**Where is my traffic coming from?**

```sql
SELECT hostname AS source, SUM(visitors_count) AS visitors
FROM ref_stats
WHERE website_id = 1 AND hour >= datetime('now', '-30 days') AND hostname != '__direct_or_unknown__'
GROUP BY hostname ORDER BY visitors DESC LIMIT 15
```

**This week vs last week**

```sql
SELECT
  SUM(CASE WHEN hour >= datetime('now', '-7 days') THEN visitors ELSE 0 END) AS this_week,
  SUM(CASE WHEN hour < datetime('now', '-7 days') THEN visitors ELSE 0 END) AS last_week
FROM site_stats
WHERE website_id = 1 AND hour >= datetime('now', '-14 days')
```

**Top pages**

```sql
SELECT pathname, SUM(visitors_count) AS visitors
FROM page_stats
WHERE website_id = 1 AND hour >= datetime('now', '-30 days')
GROUP BY pathname ORDER BY visitors DESC LIMIT 10
```

**Daily trend** (visitors as the dashboard counts them: distinct signatures with a page view per day; days are UTC here, so shift `timestamp` for a local day, e.g. `DATE(timestamp, '-7 hours')`)

```sql
SELECT DATE(timestamp) AS day,
  COUNT(DISTINCT user_signature) AS visitors,
  COUNT(*) AS page_views
FROM events
WHERE website_id = 1 AND event_type = 1 AND timestamp >= datetime('now', '-30 days')
GROUP BY day ORDER BY day
```

**Best day of the week** (all 7 days, never `LIMIT 1`)

```sql
WITH days(dow, name) AS (
  VALUES (0,'Sunday'),(1,'Monday'),(2,'Tuesday'),(3,'Wednesday'),(4,'Thursday'),(5,'Friday'),(6,'Saturday')
)
SELECT d.name, COALESCE(SUM(s.visitors), 0) AS visitors
FROM days d
LEFT JOIN site_stats s
  ON CAST(strftime('%w', s.hour) AS INTEGER) = d.dow
 AND s.website_id = 1 AND s.hour >= datetime('now', '-8 weeks')
GROUP BY d.dow ORDER BY d.dow
```

**Revenue** (as the dashboard counts it: price is in cents, `quantity` defaults to 1, currency codes are trimmed and upper case, and a repeated `order_id` counts once)

```sql
WITH purchases AS (
  SELECT
    CAST(json_extract(custom_event_meta, '$.price') AS REAL)
      * COALESCE(CAST(json_extract(custom_event_meta, '$.quantity') AS INTEGER), 1) AS cents,
    COALESCE(NULLIF(UPPER(TRIM(json_extract(custom_event_meta, '$.currency'))), ''), 'USD') AS currency,
    ROW_NUMBER() OVER (
      PARTITION BY COALESCE(NULLIF(CAST(json_extract(custom_event_meta, '$.order_id') AS TEXT), ''), 'event-' || id)
      ORDER BY timestamp, id
    ) AS copy
  FROM events
  WHERE website_id = 1 AND event_type = 2
    AND LOWER(custom_event_name) = 'revenue:purchased'
    AND json_valid(custom_event_meta) = 1
    AND CAST(json_extract(custom_event_meta, '$.price') AS REAL) > 0
    AND timestamp >= datetime('now', '-30 days')
)
SELECT currency, SUM(cents) / 100.0 AS revenue, COUNT(*) AS sales
FROM purchases WHERE copy = 1
GROUP BY currency ORDER BY sales DESC
```

**Where visitors go after the homepage**

```sql
SELECT target_page, SUM(transitions) AS visits
FROM flow_transition_stats
WHERE website_id = 1 AND source_page = '/' AND hour >= datetime('now', '-30 days')
GROUP BY target_page ORDER BY visits DESC LIMIT 10
```

## Errors and fixes

| Error | Fix |
|-------|-----|
| `no such table` / `no such column` | Check the exact name in `get_schema`. |
| `table not allowed` | That table holds accounts or secrets. Use the analytics tables. |
| `only SELECT queries are allowed` | Start with `SELECT` or `WITH`. |
| `multiple statements not allowed` | Send one statement. |
| `comments not allowed in queries` | Remove `--` and `/* */`. |
| `truncated: true` in the result | You hit 1000 rows. Aggregate more, or add `LIMIT`. |

## SQL API without MCP

Clients without MCP support (or scripts) use the same key over HTTP. `FUSIONALY_URL` and `FUSIONALY_AGENT_KEY` are the values the `connect` skill verified.

```bash
curl -s -H "Authorization: Bearer $FUSIONALY_AGENT_KEY" "$FUSIONALY_URL/z/api/v1/schema"

curl -s -X POST "$FUSIONALY_URL/z/api/v1/sql" \
  -H "Authorization: Bearer $FUSIONALY_AGENT_KEY" \
  -H "Content-Type: application/json" \
  -d '{"sql": "SELECT ..."}'
```

Both endpoints allow 30 requests per minute.
