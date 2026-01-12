# Manual Test Cases

This document contains human-readable test cases that can be executed manually or with AI-assisted testing tools like Playwright MCP.

## Prerequisites

- Development server running (`make dev`)
- Test user created: `./tmp/fnctl create-admin-user test@example.com testpassword123`
- Test data seeded (or use existing data)

---

## Authentication

### TC-AUTH-001: Login with valid credentials
1. Navigate to `/login`
2. Enter email: `test@example.com`
3. Enter password: `testpassword123`
4. Click "Login" button
5. **Expected**: Redirected to `/admin/dashboard`, user is logged in

### TC-AUTH-002: Login with invalid credentials
1. Navigate to `/login`
2. Enter email: `wrong@example.com`
3. Enter password: `wrongpassword`
4. Click "Login" button
5. **Expected**: Error message "Invalid email or password" displayed, stay on login page

### TC-AUTH-003: Logout
1. Login successfully
2. Click "Logout" link in navigation header
3. **Expected**: Redirected to `/login`, session ended

### TC-AUTH-004: Protected routes redirect to login
1. Clear cookies/session
2. Navigate directly to `/admin/dashboard`
3. **Expected**: Redirected to `/login`

---

## Navigation Header

### TC-NAV-001: Header displays on all admin pages
1. Login successfully
2. Visit each admin page:
   - `/admin/dashboard`
   - `/admin/websites`
   - `/admin/events`
   - `/admin/lens`
   - `/admin/administration/ingestion`
3. **Expected**: Each page shows navigation header with links: Dashboard, Websites, Events, Lens AI, Administration, Logout

### TC-NAV-002: Navigation links work correctly
1. Login and go to Dashboard
2. Click each navigation link
3. **Expected**: Each link navigates to the correct page without full page reload (Inertia navigation)

---

## Events Page

### TC-EVENTS-001: Page loads with header and filters
1. Navigate to `/admin/events`
2. **Expected**:
   - Page header shows "Events" with icon
   - Website selector dropdown is visible
   - Date range buttons visible (Today, Yesterday, 7d, 30d, Month)
   - Type filter buttons visible (All, Pages, Events)
   - Search inputs for Referrer and Event Key
   - Sessions toggle button
   - Events table with columns: Time, User, URL, Referrer, Event Key, Type

### TC-EVENTS-002: Website selector works
1. Navigate to `/admin/events`
2. Click on the website selector dropdown
3. **Expected**: Dropdown opens showing all available websites
4. Select a different website
5. **Expected**:
   - URL updates with `website_id` parameter
   - Events table refreshes with data for selected website
   - Website selector shows newly selected website name

### TC-EVENTS-003: Date range filter works
1. Navigate to `/admin/events`
2. Click "30d" button
3. **Expected**:
   - URL updates with `range=last_30_days`
   - Events from last 30 days are displayed
   - "30d" button appears selected (highlighted)
4. Click "Today" button
5. **Expected**:
   - URL updates with `range=today`
   - Only today's events displayed

### TC-EVENTS-004: Type filter works
1. Navigate to `/admin/events` with data
2. Click "Pages" button
3. **Expected**:
   - URL updates with `type=page`
   - Only page view events displayed (Type column shows "Page View")
4. Click "Events" button
5. **Expected**:
   - URL updates with `type=event`
   - Only custom events displayed (Type column shows "Event")
6. Click "All" button
7. **Expected**:
   - `type` parameter removed from URL
   - All event types displayed

### TC-EVENTS-005: Event Key search works
1. Navigate to `/admin/events` with event data
2. Type "revenue" in the Event Key search input
3. Press Enter
4. **Expected**:
   - URL updates with `event_key=revenue`
   - Only events with "revenue" in event key are displayed

### TC-EVENTS-006: Referrer search works
1. Navigate to `/admin/events` with referrer data
2. Type "google" in the Referrer search input
3. Press Enter
4. **Expected**:
   - URL updates with `referrer=google`
   - Only events with "google" in referrer are displayed

### TC-EVENTS-007: Clear button resets filters
1. Navigate to `/admin/events`
2. Apply multiple filters (change date range, type, add search term)
3. **Expected**: Clear button appears
4. Click Clear button
5. **Expected**:
   - All filters reset to defaults
   - URL shows only `page=1&range=last_7_days&website_id=X`
   - Clear button disappears

### TC-EVENTS-008: URL state persistence
1. Navigate to `/admin/events?website_id=6&range=last_30_days&type=event&event_key=revenue`
2. **Expected**:
   - Website selector shows correct website
   - "30d" button is highlighted
   - "Events" type button is highlighted
   - Event Key input contains "revenue"
   - Events are filtered accordingly

### TC-EVENTS-009: Pagination works
1. Navigate to `/admin/events` with enough data for multiple pages
2. **Expected**: Pagination shows "Page X of Y (Z events)"
3. Click "Next" link
4. **Expected**:
   - URL updates with `page=2`
   - Next page of events displayed
   - All other filters preserved
4. Click "Previous" link
5. **Expected**: Returns to previous page

### TC-EVENTS-010: Sessions toggle
1. Navigate to `/admin/events`
2. Click "Sessions" toggle button
3. **Expected**:
   - Button becomes highlighted
   - Events grouped by session (preference saved to localStorage)
4. Refresh the page
5. **Expected**: Sessions toggle state persisted

---

## Dashboard Page

### TC-DASH-001: Page loads with all components
1. Navigate to `/admin/dashboard`
2. **Expected**:
   - Header shows "Dashboard"
   - Website selector visible
   - Time range selector visible
   - Metrics cards: Visitors, Page Views, Sessions, Bounce Rate, Avg Time, Revenue
   - Charts section
   - Pages widget
   - Referrers widget
   - Countries widget
   - Device Analytics widget
   - Events widget

### TC-DASH-002: Website selector updates dashboard
1. Navigate to `/admin/dashboard`
2. Change website in selector
3. **Expected**: All dashboard widgets refresh with data for selected website

### TC-DASH-003: Time range selector works
1. Navigate to `/admin/dashboard`
2. Change time range
3. **Expected**:
   - URL updates with new range
   - All metrics and charts update

---

## Websites Page

### TC-WEB-001: List websites
1. Navigate to `/admin/websites`
2. **Expected**: Table showing all websites with domain names

### TC-WEB-002: Create new website
1. Navigate to `/admin/websites/new`
2. Enter domain: `test-site.com`
3. Submit form
4. **Expected**: Redirected to websites list, new website appears

### TC-WEB-003: Edit website
1. Navigate to `/admin/websites`
2. Click edit on a website
3. Modify domain
4. Submit form
5. **Expected**: Changes saved, redirected to list

### TC-WEB-004: Delete website
1. Navigate to `/admin/websites`
2. Click delete on a website
3. Confirm deletion
4. **Expected**: Website removed from list

---

## Administration Pages

### TC-ADMIN-001: Ingestion page accessible
1. Navigate to `/admin/administration/ingestion`
2. **Expected**: Page loads with tracking code instructions

### TC-ADMIN-002: Database page accessible
1. Navigate to `/admin/administration/database`
2. **Expected**: Page loads with database statistics

### TC-ADMIN-003: Users page accessible
1. Navigate to `/admin/administration/users`
2. **Expected**: Page loads with user management

### TC-ADMIN-004: Branding page accessible
1. Navigate to `/admin/administration/branding`
2. **Expected**: Page loads with branding settings

### TC-ADMIN-005: Sidebar navigation works
1. Navigate to any administration page
2. Click different sidebar links
3. **Expected**: Each link navigates to correct page

---

## Lens AI Page

### TC-LENS-001: Page loads
1. Navigate to `/admin/lens`
2. **Expected**: Page loads with AI query interface

---

## Cross-cutting Concerns

### TC-INERTIA-001: Navigation doesn't cause full page reload
1. Login and go to Dashboard
2. Open browser DevTools Network tab
3. Click on "Events" navigation link
4. **Expected**:
   - Only XHR/Fetch request made (not full document)
   - `X-Inertia` header in request
   - JSON response received
   - Page updates without full reload

### TC-INERTIA-002: Browser back/forward works
1. Navigate: Dashboard → Events → Websites
2. Click browser back button
3. **Expected**: Returns to Events page with correct state
4. Click browser forward button
5. **Expected**: Returns to Websites page

### TC-CSRF-001: Forms include CSRF token
1. Navigate to any page with a form
2. Inspect form in DevTools
3. **Expected**: Hidden CSRF token field present or token in request headers
