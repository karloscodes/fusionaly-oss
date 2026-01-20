# Public Dashboard Sharing

**Date:** 2025-01-20
**Status:** Approved
**Reviewers:** DHH, Jason Fried (philosophies)

## Overview

Allow users to publicly share a read-only version of their dashboard via a unique link. When shared, the dashboard displays the last 30 days of analytics with a "Powered by Fusionaly" footer.

## Why OSS (not Pro)

- Public dashboards are table stakes (Plausible has it)
- "Powered by Fusionaly" links are organic marketing
- Sharing is visibility, not value - Pro should sell insights, not share buttons
- A generous free tier breeds evangelists

## Design Principles

- **Simple:** Toggle on/off, that's it
- **Secure:** Disabling sharing invalidates the old URL forever
- **Read-only:** No annotations, no editing, no navigation to other pages
- **Fixed timeframe:** Last 30 days, no date picker

---

## Data Model

Add `share_token` field to existing Website model:

```go
// internal/websites/website.go
type Website struct {
    ID          uint      `gorm:"primaryKey;autoIncrement" json:"id"`
    Domain      string    `gorm:"unique;not null" json:"domain"`
    PrivacyMode string    `gorm:"default:'tracking'" json:"privacy_mode"`
    ShareToken  *string   `gorm:"uniqueIndex" json:"share_token"` // nullable
    CreatedAt   time.Time `json:"created_at"`
}
```

**Logic:**
- `ShareToken` is nil → not shared
- `ShareToken` has value → shared at `/share/{token}`
- To disable: set to nil (old URL stops working)
- To re-enable: generate new token (new URL)

---

## Migration

Add to migrations or auto-migrate:

```sql
ALTER TABLE websites ADD COLUMN share_token TEXT UNIQUE;
```

---

## Routes

```go
// Public (no auth, rate limited)
srv.Get("/share/:token", PublicDashboardAction, publicRateLimitedConfig)

// Admin (auth required)
srv.Post("/admin/websites/:id/share/enable", EnableShareAction, adminConfig)
srv.Post("/admin/websites/:id/share/disable", DisableShareAction, adminConfig)
```

**Rate limiting:** Apply same rate limit as public event API to prevent abuse.

---

## Context Functions

Create `internal/websites/sharing.go`:

```go
package websites

import (
    "crypto/rand"
    "encoding/base64"

    "gorm.io/gorm"
)

// generateToken creates a URL-safe random token
func generateToken(length int) string {
    bytes := make([]byte, length)
    rand.Read(bytes)
    return base64.URLEncoding.EncodeToString(bytes)[:length]
}

// EnableSharing generates a share token for the website
func EnableSharing(db *gorm.DB, websiteID uint) (string, error) {
    token := generateToken(12)
    err := db.Model(&Website{}).
        Where("id = ?", websiteID).
        Update("share_token", token).Error
    return token, err
}

// DisableSharing removes the share token (invalidates public URL)
func DisableSharing(db *gorm.DB, websiteID uint) error {
    return db.Model(&Website{}).
        Where("id = ?", websiteID).
        Update("share_token", nil).Error
}

// GetWebsiteByShareToken finds a website by its public share token
func GetWebsiteByShareToken(db *gorm.DB, token string) (*Website, error) {
    var website Website
    err := db.Where("share_token = ?", token).First(&website).Error
    return &website, err
}

// GetShareToken returns the share token for a website (nil if not shared)
func GetShareToken(db *gorm.DB, websiteID uint) (*string, error) {
    var website Website
    err := db.Select("share_token").Where("id = ?", websiteID).First(&website).Error
    if err != nil {
        return nil, err
    }
    return website.ShareToken, nil
}
```

---

## HTTP Handlers

Create `internal/http/share_handler.go`:

```go
package http

import (
    "fmt"

    "github.com/gofiber/fiber/v2"
    "github.com/karloscodes/cartridge"
    "github.com/karloscodes/cartridge/flash"
    "github.com/karloscodes/cartridge/inertia"
    "github.com/karloscodes/cartridge/structs"

    "fusionaly/internal/timeframe"
    "fusionaly/internal/websites"
)

// PublicDashboardAction renders a read-only public dashboard
func PublicDashboardAction(ctx *cartridge.Context) error {
    token := ctx.Params("token")
    if token == "" {
        return ctx.Status(fiber.StatusNotFound).SendString("Not found")
    }

    website, err := websites.GetWebsiteByShareToken(ctx.DB(), token)
    if err != nil {
        return ctx.Status(fiber.StatusNotFound).SendString("Dashboard not found")
    }

    // Parse timezone from cookie, default to UTC
    tz := ctx.Cookies("_tz")
    if tz == "" {
        tz = "UTC"
    }

    // Fixed 30-day timeframe
    timeFrame := timeframe.Last30Days(tz)

    metrics, err := fetchMetrics(ctx.DB(), timeFrame, int(website.ID), ctx.Logger)
    if err != nil {
        ctx.Logger.Error("Error fetching public dashboard metrics", "error", err)
        return ctx.Status(fiber.StatusInternalServerError).SendString("Error loading dashboard")
    }

    props := structs.Map(metrics)
    props["website_domain"] = website.Domain
    props["bucket_size"] = string(timeFrame.BucketSize)

    return inertia.RenderPage(ctx.Ctx, "PublicDashboard", props)
}

// EnableShareAction enables public sharing for a website
func EnableShareAction(ctx *cartridge.Context) error {
    websiteID, err := ctx.ParamsInt("id")
    if err != nil {
        return ctx.Status(fiber.StatusBadRequest).SendString("Invalid website ID")
    }

    _, err = websites.EnableSharing(ctx.DB(), uint(websiteID))
    if err != nil {
        ctx.Logger.Error("Failed to enable sharing", "error", err, "websiteID", websiteID)
        flash.SetFlash(ctx.Ctx, "error", "Failed to enable sharing")
    } else {
        flash.SetFlash(ctx.Ctx, "success", "Public sharing enabled")
    }

    return ctx.Redirect(fmt.Sprintf("/admin/websites/%d/dashboard", websiteID), fiber.StatusFound)
}

// DisableShareAction disables public sharing for a website
func DisableShareAction(ctx *cartridge.Context) error {
    websiteID, err := ctx.ParamsInt("id")
    if err != nil {
        return ctx.Status(fiber.StatusBadRequest).SendString("Invalid website ID")
    }

    err = websites.DisableSharing(ctx.DB(), uint(websiteID))
    if err != nil {
        ctx.Logger.Error("Failed to disable sharing", "error", err, "websiteID", websiteID)
        flash.SetFlash(ctx.Ctx, "error", "Failed to disable sharing")
    } else {
        flash.SetFlash(ctx.Ctx, "success", "Public sharing disabled")
    }

    return ctx.Redirect(fmt.Sprintf("/admin/websites/%d/dashboard", websiteID), fiber.StatusFound)
}
```

---

## Timeframe Helper

Add to `internal/timeframe/timeframe.go`:

```go
// Last30Days returns a TimeFrame for the last 30 days
func Last30Days(tz string) *TimeFrame {
    loc, err := time.LoadLocation(tz)
    if err != nil {
        loc = time.UTC
    }

    now := time.Now().In(loc)
    to := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, loc)
    from := to.AddDate(0, 0, -29) // 30 days including today
    from = time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, loc)

    return &TimeFrame{
        From:       from,
        To:         to,
        BucketSize: BucketDay,
    }
}
```

---

## React Component

Create `web/src/pages/PublicDashboard.tsx`:

```tsx
import { usePage } from "@inertiajs/react";
import {
  StatsCards,
  VisitorsChart,
  TopPagesTable,
  TopReferrersTable,
  TopCountriesTable,
  TopDevicesTable,
  TopBrowsersTable,
} from "@/components/dashboard";

interface Props {
  website_domain: string;
  total_visitors: number;
  total_views: number;
  total_sessions: number;
  bounce_rate: number;
  visits_duration: number;
  visitors: Array<{ date: string; count: number }>;
  top_urls: Array<{ name: string; count: number }>;
  top_referrers: Array<{ name: string; count: number }>;
  top_countries: Array<{ name: string; count: number }>;
  top_devices: Array<{ name: string; count: number }>;
  top_browsers: Array<{ name: string; count: number }>;
  bucket_size: string;
}

export default function PublicDashboard() {
  const props = usePage<{ props: Props }>().props as unknown as Props;

  return (
    <div className="min-h-screen bg-gray-50">
      {/* Header */}
      <header className="bg-white border-b border-gray-200">
        <div className="max-w-7xl mx-auto px-4 py-4">
          <h1 className="text-xl font-bold text-gray-900">
            {props.website_domain}
          </h1>
          <p className="text-sm text-gray-500">Last 30 days</p>
        </div>
      </header>

      {/* Dashboard Content */}
      <main className="max-w-7xl mx-auto px-4 py-8">
        <div className="space-y-8">
          {/* Stats Cards */}
          <StatsCards
            visitors={props.total_visitors}
            pageViews={props.total_views}
            sessions={props.total_sessions}
            bounceRate={props.bounce_rate}
            avgDuration={props.visits_duration}
          />

          {/* Visitors Chart */}
          <div className="bg-white border border-gray-200 rounded-lg p-6">
            <VisitorsChart
              data={props.visitors}
              bucketSize={props.bucket_size}
            />
          </div>

          {/* Tables Grid */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <TopPagesTable data={props.top_urls} />
            <TopReferrersTable data={props.top_referrers} />
            <TopCountriesTable data={props.top_countries} />
            <TopDevicesTable data={props.top_devices} />
            <TopBrowsersTable data={props.top_browsers} />
          </div>
        </div>
      </main>

      {/* Footer */}
      <footer className="border-t border-gray-200 bg-white mt-12">
        <div className="max-w-7xl mx-auto px-4 py-6 text-center">
          <a
            href="https://fusionaly.com"
            target="_blank"
            rel="noopener noreferrer"
            className="text-sm text-gray-500 hover:text-gray-700"
          >
            Powered by Fusionaly
          </a>
        </div>
      </footer>
    </div>
  );
}
```

---

## Dashboard Share UI

Add to `web/src/pages/Dashboard.tsx` (or a separate ShareButton component):

```tsx
interface ShareSectionProps {
  websiteId: number;
  shareToken: string | null;
}

function ShareSection({ websiteId, shareToken }: ShareSectionProps) {
  const [copied, setCopied] = useState(false);

  const shareUrl = shareToken
    ? `${window.location.origin}/share/${shareToken}`
    : null;

  const copyToClipboard = () => {
    if (shareUrl) {
      navigator.clipboard.writeText(shareUrl);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }
  };

  return (
    <div className="flex items-center gap-2">
      {shareToken ? (
        <>
          <Button variant="outline" size="sm" onClick={copyToClipboard}>
            {copied ? (
              <><Check className="h-4 w-4 mr-1" /> Copied</>
            ) : (
              <><Copy className="h-4 w-4 mr-1" /> Copy public link</>
            )}
          </Button>
          <form action={`/admin/websites/${websiteId}/share/disable`} method="POST">
            <Button variant="ghost" size="sm" type="submit">
              Disable sharing
            </Button>
          </form>
        </>
      ) : (
        <form action={`/admin/websites/${websiteId}/share/enable`} method="POST">
          <Button variant="outline" size="sm" type="submit">
            <Share2 className="h-4 w-4 mr-1" />
            Share publicly
          </Button>
        </form>
      )}
    </div>
  );
}
```

**Pass `share_token` to Dashboard props** in the handler:

```go
// In WebsiteDashboardAction
props["share_token"] = website.ShareToken
```

---

## Files to Create/Modify

| File | Action |
|------|--------|
| `internal/websites/website.go` | Add `ShareToken` field |
| `internal/websites/sharing.go` | **New** - sharing context functions |
| `internal/http/share_handler.go` | **New** - HTTP handlers |
| `internal/http/dashboard_handler.go` | Pass `share_token` to props |
| `internal/routes.go` | Add share routes |
| `internal/timeframe/timeframe.go` | Add `Last30Days()` helper |
| `web/src/pages/PublicDashboard.tsx` | **New** - React component |
| `web/src/pages/Dashboard.tsx` | Add share toggle UI |
| `web/src/inertia.tsx` | Register PublicDashboard page |

---

## Testing

### Unit Tests

- `TestEnableSharing` - creates token, saves to DB
- `TestDisableSharing` - sets token to nil
- `TestGetWebsiteByShareToken` - finds website by token
- `TestGetWebsiteByShareToken_NotFound` - returns error for invalid token

### E2E Tests

- Enable sharing → copy link → visit link → see public dashboard
- Disable sharing → old link returns 404
- Public dashboard shows correct data (matches admin view)
- Public dashboard has no navigation/edit controls

---

## Security Considerations

- Rate limit `/share/:token` endpoint (same as public API)
- Tokens are 12 random characters (URL-safe base64)
- No auth bypass - public route only shows what's explicitly shared
- Disabling sharing invalidates token permanently
