package testsupport

import (
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/karloscodes/cartridge"
	ctestsupport "github.com/karloscodes/cartridge/testsupport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"fusionaly/internal"
	"fusionaly/internal/analytics"
	"fusionaly/internal/annotations"
	"fusionaly/internal/config"
	"fusionaly/internal/events"
	"fusionaly/internal/onboarding"
	"fusionaly/internal/settings"
	"fusionaly/internal/timeframe"
	"fusionaly/internal/users"
	"fusionaly/internal/websites"
	"github.com/karloscodes/cartridge/cache"
)

// SessionCookieName is the expected cookie name for session cookies in tests.
// This should match the pattern used in routes.go: cfg.AppName + "_session"
const SessionCookieName = "fusionaly_session"

// testDBCache caches test databases by test name to allow multiple calls
// within the same test to share the same database
var testDBCache = make(map[string]*gorm.DB)
var testDBCacheMu sync.Mutex

// TestDBManager wraps cartridge's TestDBManager with fusionaly's interface
type TestDBManager struct {
	*ctestsupport.TestDBManager
}

// NewTestDBManager creates a TestDBManager that implements cartridge.DBManager
func NewTestDBManager(db *gorm.DB) *TestDBManager {
	return &TestDBManager{
		TestDBManager: ctestsupport.NewTestDBManager(db),
	}
}

// Ensure TestDBManager implements cartridge.DBManager
var _ cartridge.DBManager = (*TestDBManager)(nil)

// TestDateStat is a helper struct for testing date-based statistics
type TestDateStat struct {
	Date  time.Time
	Count int
}

// ConvertToTestDateStat converts a DateStat to TestDateStat
func ConvertToTestDateStat(ds timeframe.DateStat) (TestDateStat, error) {
	t, err := time.Parse(time.RFC3339, ds.Date)
	if err != nil {
		return TestDateStat{}, err
	}
	return TestDateStat{Date: t, Count: ds.Count}, nil
}

// allModels returns all fusionaly models for migration
func allModels() []any {
	return []any{
		&cache.CacheRecord{},
		&events.Event{},
		&events.IngestedEvent{},
		&users.User{},
		&settings.Setting{},
		&websites.Website{},
		&analytics.SiteStat{},
		&analytics.PageStat{},
		&analytics.RefStat{},
		&analytics.BrowserStat{},
		&analytics.OSStat{},
		&analytics.DeviceStat{},
		&analytics.CountryStat{},
		&analytics.UTMStat{},
		&analytics.EventStat{},
		&analytics.QueryParamStat{},
		&analytics.FlowTransitionStat{},
		&onboarding.OnboardingSession{},
		&annotations.Annotation{},
	}
}

// SetupTestDB creates a test database with all fusionaly models migrated.
// Uses a named in-memory database with cache=shared to allow multiple connections
// to share the same database within a test. Caches the database by test name
// so multiple calls within the same test return the same database.
func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	testName := t.Name()

	// Use root test name for caching to handle closure issues where
	// setup functions capture the outer t while t.Run has subtest t
	rootName := testName
	if idx := strings.Index(testName, "/"); idx > 0 {
		rootName = testName[:idx]
	}

	// Check cache first
	testDBCacheMu.Lock()
	if db, exists := testDBCache[rootName]; exists {
		testDBCacheMu.Unlock()
		return db
	}
	testDBCacheMu.Unlock()

	// Create a unique named in-memory database for each test
	// cache=shared allows multiple connections to the same database
	sanitizedName := strings.ReplaceAll(rootName, "/", "_")
	dsn := fmt.Sprintf("file:test_%s_%d?mode=memory&cache=shared", sanitizedName, time.Now().UnixNano())

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("testsupport: failed to open test database: %v", err)
	}

	// Apply SQLite pragmas
	db.Exec("PRAGMA foreign_keys = ON")
	db.Exec("PRAGMA journal_mode = WAL")

	// Auto-migrate models
	if err := db.AutoMigrate(allModels()...); err != nil {
		t.Fatalf("testsupport: failed to migrate models: %v", err)
	}

	// Cache the database
	testDBCacheMu.Lock()
	testDBCache[rootName] = db
	testDBCacheMu.Unlock()

	// Register cleanup
	t.Cleanup(func() {
		testDBCacheMu.Lock()
		delete(testDBCache, rootName)
		testDBCacheMu.Unlock()
		sqlDB, err := db.DB()
		if err == nil {
			sqlDB.Close()
		}
	})

	return db
}

// SetupTestDBManager creates a test DB manager using cartridge's testsupport
func SetupTestDBManager(t *testing.T) (*TestDBManager, *slog.Logger) {
	cfg := config.GetConfig()

	// SAFETY CHECK: Ensure we're in test environment
	if cfg.Environment != config.Test {
		t.Fatalf("CRITICAL: Tests must run in test environment! Current: %s. Set FUSIONALY_ENV=test", cfg.Environment)
	}

	db := SetupTestDB(t)
	logger := GetLogger()
	dbManager := NewTestDBManager(db)

	return dbManager, logger
}

// SetupTestDBManagerWithWebsite creates a test database manager with a test website
func SetupTestDBManagerWithWebsite(t *testing.T, domain string) (*TestDBManager, *slog.Logger, websites.Website) {
	dbManager, logger := SetupTestDBManager(t)
	website := CreateTestWebsite(dbManager.GetConnection(), domain)
	return dbManager, logger, website
}

// CleanAllTables clears all non-system tables in the database
func CleanAllTables(db *gorm.DB) {
	var tableNames []string
	db.Raw("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").Scan(&tableNames)

	var tables []string
	for _, name := range tableNames {
		if name != "migrations" && name != "schema_migrations" {
			tables = append(tables, name)
		}
	}

	if len(tables) == 0 {
		return
	}

	db.Exec("PRAGMA foreign_keys = OFF")
	defer db.Exec("PRAGMA foreign_keys = ON")

	db.Transaction(func(tx *gorm.DB) error {
		for _, table := range tables {
			tx.Exec("DELETE FROM " + table)
			tx.Exec("DELETE FROM sqlite_sequence WHERE name=?", table)
		}
		return nil
	})
}

// CleanTables cleans specific tables or all tables if none specified
func CleanTables(db *gorm.DB, tables []string) {
	if len(tables) == 0 {
		CleanAllTables(db)
		return
	}

	db.Exec("PRAGMA foreign_keys = OFF")
	defer db.Exec("PRAGMA foreign_keys = ON")

	db.Transaction(func(tx *gorm.DB) error {
		for _, table := range tables {
			tx.Exec("DELETE FROM " + table)
			tx.Exec("DELETE FROM sqlite_sequence WHERE name=?", table)
		}
		return nil
	})
}

// CleanAllAggregates cleans all aggregate tables
func CleanAllAggregates(db *gorm.DB) {
	CleanTables(db, []string{
		"site_stats", "page_stats", "ref_stats", "device_stats",
		"browser_stats", "os_stats", "country_stats", "utm_stats",
		"event_stats", "flow_transition_stats",
	})
}

// CreateTestWebsite creates a test website in the database
func CreateTestWebsite(db *gorm.DB, domain string) websites.Website {
	var website websites.Website
	if db.Where("domain = ?", domain).First(&website).Error != nil {
		website = websites.Website{Domain: domain, CreatedAt: time.Now().UTC()}
		db.Create(&website)
	}
	return website
}

// CreateTestUser creates a test user in the database
func CreateTestUser(db *gorm.DB, email, password string) users.User {
	var user users.User
	if db.Where("email = ?", email).First(&user).Error == nil {
		return user
	}

	user = users.User{
		Email:             email,
		EncryptedPassword: password,
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	db.Create(&user)
	return user
}

// CreateTestUserForAuth creates a user with properly hashed password for auth testing
func CreateTestUserForAuth(t *testing.T, db *gorm.DB, email, password string) *users.User {
	t.Helper()

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	require.NoError(t, err)

	user := &users.User{
		Email:             email,
		EncryptedPassword: string(hashedPassword),
		CreatedAt:         time.Now().UTC(),
		UpdatedAt:         time.Now().UTC(),
	}
	require.NoError(t, db.Create(user).Error)
	return user
}

// GetLogger returns a test logger
func GetLogger() *slog.Logger {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelError})
	return slog.New(handler)
}

// GetFirstDayOfISOWeek returns the first day of the given ISO week
func GetFirstDayOfISOWeek(year, week int) time.Time {
	jan4 := time.Date(year, time.January, 4, 0, 0, 0, 0, time.UTC)
	isoYearStart := jan4.AddDate(0, 0, -int(jan4.Weekday()-time.Monday))
	return isoYearStart.AddDate(0, 0, (week-1)*7)
}

// GetTimeInISOWeek returns a time in the specified ISO week
func GetTimeInISOWeek(year, week, dayOffset, hour, min int) time.Time {
	return GetFirstDayOfISOWeek(year, week).AddDate(0, 0, dayOffset).
		Add(time.Duration(hour)*time.Hour + time.Duration(min)*time.Minute)
}

// CreatePageViewEvent creates a page view event for testing
func CreatePageViewEvent(
	eventID, websiteID uint,
	userSignature, hostname, pathname string,
	timestamp time.Time,
	isNewVisitor, isNewSession, isEntrance, isExit bool,
	isBounce ...bool,
) *events.EventProcessingData {
	bounce := len(isBounce) > 0 && isBounce[0]

	return &events.EventProcessingData{
		EventID: eventID, WebsiteID: websiteID,
		UserSignature: userSignature, Hostname: hostname, Pathname: pathname,
		DeviceType: "desktop", Browser: "chrome", OperatingSystem: "windows",
		Country: "US", EventType: events.EventTypePageView,
		IsNewVisitor: isNewVisitor, IsNewSession: isNewSession,
		Timestamp: timestamp, IsEntrance: isEntrance, IsExit: isExit,
		IsBounce: bounce, HasUTM: false,
	}
}

// CreateCustomEvent creates a custom event for testing
func CreateCustomEvent(
	eventID, websiteID uint,
	userSignature, hostname, pathname string,
	timestamp time.Time,
	isNewVisitor bool,
	eventName, eventKey string,
) *events.EventProcessingData {
	return &events.EventProcessingData{
		EventID: eventID, WebsiteID: websiteID,
		UserSignature: userSignature, Hostname: hostname, Pathname: pathname,
		DeviceType: "desktop", Browser: "chrome", OperatingSystem: "windows",
		Country: "US", EventType: events.EventTypeCustomEvent,
		CustomEventName: eventName, CustomEventKey: eventKey,
		IsNewVisitor: isNewVisitor, Timestamp: timestamp, HasUTM: false,
	}
}

// CreateTestEventInput creates a CollectEventInput for testing
func CreateTestEventInput(
	ipAddress, userAgent string,
	eventType events.EventType,
	timestamp time.Time,
	rawUrl, referrerURL, customEventName, customEventMeta string,
) *events.CollectEventInput {
	return &events.CollectEventInput{
		IPAddress: ipAddress, UserAgent: userAgent,
		EventType: eventType, Timestamp: timestamp,
		RawUrl: rawUrl, ReferrerURL: referrerURL,
		CustomEventName: customEventName, CustomEventMeta: customEventMeta,
	}
}

// CreateEvent creates an event directly in the database for testing
func CreateEvent(t *testing.T, dbManager cartridge.DBManager, websiteID uint, visitorID, path string, timestamp time.Time) {
	db := dbManager.GetConnection()
	event := &events.Event{
		WebsiteID: websiteID, UserSignature: visitorID + "-signature",
		Hostname: "example.com", Pathname: path,
		ReferrerHostname: "direct", EventType: events.EventTypePageView,
		Timestamp: timestamp, CreatedAt: time.Now().UTC(),
	}
	require.NoError(t, db.Create(event).Error)
}

// CreateRandomEvents creates random events for testing
func CreateRandomEvents(dbManager cartridge.DBManager, logger *slog.Logger, website websites.Website, count int) error {
	ip := fmt.Sprintf("%d.%d.%d.%d", rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256))
	userAgent := "Mozilla/5.0 Test Browser"

	for i := 0; i < count; i++ {
		input := &events.CollectEventInput{
			IPAddress: ip, UserAgent: userAgent,
			ReferrerURL: "https://google.com", EventType: events.EventTypePageView,
			Timestamp: time.Now().Add(-time.Duration(i) * time.Minute),
			RawUrl:    fmt.Sprintf("https://%s/page-%d", website.Domain, i),
		}
		if err := events.CollectEvent(dbManager, logger, input); err != nil {
			return err
		}
		time.Sleep(50 * time.Millisecond)
	}
	return nil
}

// ProcessAllTestEvents processes all unprocessed events
func ProcessAllTestEvents(dbManager cartridge.DBManager, logger *slog.Logger) error {
	for {
		result, err := events.ProcessUnprocessedEvents(dbManager, logger, 10)
		if err != nil {
			return err
		}
		if len(result.ProcessedEvents) == 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil
}

// CreateMinimalTestApp creates a test Fiber app with all routes
func CreateMinimalTestApp(t *testing.T, db *gorm.DB) *fiber.App {
	t.Helper()

	dbManager := NewTestDBManager(db)
	appConfig := config.GetConfig()
	appConfig.Environment = config.Test
	appConfig.PublicDirectory = "../../../web"

	cfg := cartridge.DefaultServerConfig()
	cfg.Config = appConfig
	cfg.Logger = GetLogger()
	cfg.DBManager = dbManager
	cfg.StaticDirectory = appConfig.PublicDirectory
	cfg.StaticPrefix = appConfig.PublicAssetsUrlPrefix
	cfg.TemplatesDirectory = appConfig.PublicDirectory
	// Enable SecFetchSite validation in tests to match production behavior
	// This blocks requests without Sec-Fetch-Site header (server-to-server requests)
	cfg.EnableSecFetchSite = true
	cfg.SecFetchSiteAllowedValues = []string{"cross-site", "same-site", "same-origin"}

	srv, err := cartridge.NewServer(cfg)
	require.NoError(t, err)

	internal.MountAppRoutes(srv)
	return srv.App()
}

// ExtractCSRFToken extracts the CSRF token from response body
func ExtractCSRFToken(body string) string {
	re := regexp.MustCompile(`<meta name="csrf-token" content="([^"]+)">`)
	if matches := re.FindStringSubmatch(body); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// LoginTestUser simulates login and returns session cookie, CSRF token, and CSRF cookie
func LoginTestUser(t *testing.T, app *fiber.App, email, password string) (string, string, string) {
	t.Helper()

	// GET /login for CSRF token
	req := httptest.NewRequest("GET", "/login", nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 Test Browser")
	resp, err := app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusOK, resp.StatusCode)

	body, _ := io.ReadAll(resp.Body)
	csrfToken := ExtractCSRFToken(string(body))

	var csrfCookie string
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "csrf_" {
			if csrfToken == "" {
				csrfToken = cookie.Value
			}
			csrfCookie = fmt.Sprintf("%s=%s", cookie.Name, cookie.Value)
			break
		}
	}
	require.NotEmpty(t, csrfToken)
	require.NotEmpty(t, csrfCookie)

	// POST /login
	loginData := url.Values{}
	loginData.Add("email", email)
	loginData.Add("password", password)
	loginData.Add("_csrf", csrfToken)
	loginData.Add("_tz", "UTC")

	req = httptest.NewRequest("POST", "/login", strings.NewReader(loginData.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 Test Browser")
	req.Header.Set("Cookie", csrfCookie)

	resp, err = app.Test(req)
	require.NoError(t, err)
	assert.Equal(t, fiber.StatusFound, resp.StatusCode)
	assert.Equal(t, "/admin/dashboard", resp.Header.Get("Location"))

	var sessionValue string
	for _, cookie := range resp.Cookies() {
		if cookie.Name == SessionCookieName {
			sessionValue = cookie.Value
			break
		}
	}
	require.NotEmpty(t, sessionValue)

	return sessionValue, csrfToken, csrfCookie
}

// ============ Test Case Framework ============

// TestCase represents a reusable test case structure
type TestCase struct {
	Name          string
	Setup         func(t *testing.T, dbManager cartridge.DBManager, logger *slog.Logger)
	ExpectedCount int
	Validate      func(t *testing.T, dbManager cartridge.DBManager, result interface{})
}

// RunEventProcessingTests runs a series of event processing tests
func RunEventProcessingTests(t *testing.T, testCases []TestCase) {
	for _, tc := range testCases {
		t.Run(tc.Name, func(t *testing.T) {
			dbManager, logger, _ := SetupTestDBManagerWithWebsite(t, "example.com")

			if tc.Setup != nil {
				tc.Setup(t, dbManager, logger)
			}

			result, err := events.ProcessUnprocessedEvents(dbManager, logger, 10)
			require.NoError(t, err)

			if tc.Validate != nil {
				tc.Validate(t, dbManager, result)
			} else if tc.ExpectedCount > 0 {
				assert.Equal(t, tc.ExpectedCount, len(result.ProcessedEvents))
			}
		})
	}
}

// CreatePageViewEventTest creates a standard page view test case
func CreatePageViewEventTest(name string) TestCase {
	return TestCase{
		Name: name,
		Setup: func(t *testing.T, dbManager cartridge.DBManager, logger *slog.Logger) {
			input := CreateTestEventInput(
				"192.168.1.1",
				"Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/91.0",
				events.EventTypePageView,
				time.Now().UTC(),
				"https://example.com/page1",
				"https://google.com/search",
				"", "",
			)
			require.NoError(t, events.CollectEvent(dbManager, logger, input))
		},
		ExpectedCount: 1,
		Validate: func(t *testing.T, dbManager cartridge.DBManager, result interface{}) {
			res := result.(*events.EventProcessingResult)
			require.Equal(t, 1, len(res.ProcessedEvents))

			event := res.ProcessedEvents[0]
			assert.Equal(t, events.EventTypePageView, event.EventType)
			assert.Equal(t, "example.com", event.Hostname)
			assert.Equal(t, "/page1", event.Pathname)

			var siteStat analytics.SiteStat
			require.NoError(t, dbManager.GetConnection().Where("website_id = ?", event.WebsiteID).First(&siteStat).Error)
			assert.GreaterOrEqual(t, siteStat.PageViews, 1)
		},
	}
}

// CreateCustomEventTest creates a custom event test case
func CreateCustomEventTest(name, eventName, metadataJSON string) TestCase {
	return TestCase{
		Name: name,
		Setup: func(t *testing.T, dbManager cartridge.DBManager, logger *slog.Logger) {
			input := CreateTestEventInput(
				"192.168.1.2",
				"Mozilla/5.0 (iPhone; CPU iPhone OS 13_2_3) Safari/604.1",
				events.EventTypeCustomEvent,
				time.Now().UTC(),
				"https://example.com/form",
				"https://example.com/",
				eventName, metadataJSON,
			)
			require.NoError(t, events.CollectEvent(dbManager, logger, input))
		},
		ExpectedCount: 1,
		Validate: func(t *testing.T, dbManager cartridge.DBManager, result interface{}) {
			res := result.(*events.EventProcessingResult)
			require.Equal(t, 1, len(res.ProcessedEvents))

			event := res.ProcessedEvents[0]
			assert.Equal(t, events.EventTypeCustomEvent, event.EventType)
			assert.Equal(t, eventName, event.CustomEventName)

			var eventStat analytics.EventStat
			require.NoError(t, dbManager.GetConnection().Where("website_id = ? AND event_name = ?",
				event.WebsiteID, eventName).First(&eventStat).Error)
			assert.GreaterOrEqual(t, eventStat.PageViewsCount, 1)
		},
	}
}

// CreateSequentialPageViewTest creates a multi-page session test
func CreateSequentialPageViewTest(name string) TestCase {
	return TestCase{
		Name: name,
		Setup: func(t *testing.T, dbManager cartridge.DBManager, logger *slog.Logger) {
			timestamp := time.Now().UTC()
			userAgent := "Mozilla/5.0 (Windows NT 10.0; Win64; x64) Chrome/91.0"
			ipAddress := "192.168.1.3"

			pages := []struct {
				url      string
				referrer string
				delay    time.Duration
			}{
				{"https://example.com/landing", "https://twitter.com/post", 0},
				{"https://example.com/product", "https://example.com/landing", 30 * time.Second},
				{"https://example.com/checkout", "https://example.com/product", 60 * time.Second},
			}

			for _, p := range pages {
				input := CreateTestEventInput(ipAddress, userAgent, events.EventTypePageView,
					timestamp.Add(p.delay), p.url, p.referrer, "", "")
				require.NoError(t, events.CollectEvent(dbManager, logger, input))
			}
		},
		ExpectedCount: 3,
		Validate: func(t *testing.T, dbManager cartridge.DBManager, result interface{}) {
			db := dbManager.GetConnection()

			var entrancePage analytics.PageStat
			if db.Where("entrances > ?", 0).First(&entrancePage).Error == nil {
				assert.Equal(t, "/landing", entrancePage.Pathname)
			}

			var exitPage analytics.PageStat
			if db.Where("exits > ?", 0).First(&exitPage).Error == nil {
				assert.Equal(t, "/checkout", exitPage.Pathname)
			}

			var siteStat analytics.SiteStat
			require.NoError(t, db.First(&siteStat).Error)
			assert.GreaterOrEqual(t, siteStat.Sessions, 1)
		},
	}
}
