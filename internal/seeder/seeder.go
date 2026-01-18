package seeder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"net/url"
	"strconv"
	"time"

	"log/slog"

	"github.com/karloscodes/cartridge"
	"github.com/karloscodes/cartridge/sqlite"
	"gorm.io/gorm"

	"fusionaly/internal/events"
	"fusionaly/internal/settings"
	"fusionaly/internal/users"
	"fusionaly/internal/websites"
)

// Seeder handles the data seeding process, replicating the logic
// from the original `cmd/tools/seed/main.go`.
type Seeder struct {
	DBManager  cartridge.DBManager
	Logger     *slog.Logger
	EventCount int
}

// NewSeeder creates a new seeder instance
func NewSeeder(dbManager cartridge.DBManager, logger *slog.Logger, eventCount int) *Seeder {
	if logger == nil {
		logger = slog.Default()
	}
	return &Seeder{
		DBManager:  dbManager,
		Logger:     logger,
		EventCount: eventCount,
	}
}

// SeedDomain seeds a specific existing domain with test data
func (s *Seeder) SeedDomain(ctx context.Context, domain string) error {
	start := time.Now()
	s.Logger.Info("Seeding specific domain...", slog.String("domain", domain), slog.Int("eventCount", s.EventCount))

	db := s.DBManager.GetConnection()

	// Find the website by domain
	var website websites.Website
	if err := db.Where("domain = ?", domain).First(&website).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("website with domain %s not found", domain)
		}
		return fmt.Errorf("failed to find website: %w", err)
	}

	s.Logger.Info("Found website", slog.Uint64("id", uint64(uint64(website.ID))), slog.String("domain", website.Domain))

	// Generate data for this website (use full event count since it's a single website)
	if err := s.generateRealisticDataForSingleSite(ctx, &website); err != nil {
		return fmt.Errorf("failed to generate data for %s: %w", website.Domain, err)
	}

	// Process the events
	s.Logger.Info("Processing generated events...")
	if err := s.processAllEvents(); err != nil {
		return fmt.Errorf("failed to process events: %w", err)
	}

	s.Logger.Info("Domain seeding completed successfully", slog.String("domain", domain), slog.Duration("elapsed", time.Since(start)))
	return nil
}

// generateRealisticDataForSingleSite generates data using the full event count for a single website
func (s *Seeder) generateRealisticDataForSingleSite(ctx context.Context, website *websites.Website) error {
	ipPool := generateIPPool(100)
	userAgents := getUserAgents()
	referrers := getReferrers()
	baseDomain := website.Domain
	eventsCreated := 0

	// Use the full event count for this single site
	targetEvents := s.EventCount

	// Define user journey templates
	journeyTemplates := [][]string{
		{"/", "/about", "/contact"},
		{"/", "/features", "/pricing", "/signup"},
		{"/", "/blog", "/blog/article-1", "/signup"},
		{"/pricing", "/features", "/signup"},
		{"/", "/products", "/products/widget-a", "/products/gadget-b", "/pricing"},
		{"/", "/docs", "/docs/getting-started", "/docs/api-reference"},
		{"/", "/blog", "/blog/article-1", "/blog/article-2"},
		{"/", "/signup"},
		{"/", "/features", "/pricing", "/docs", "/signup"},
		{"/products", "/products/widget-a", "/pricing", "/signup"},
		{"/", "/about", "/features", "/pricing", "/docs/getting-started", "/signup"},
		{"/login", "/dashboard", "/settings"},
		{"/blog/article-1", "/about", "/pricing", "/signup"},
	}

	goalEvents := []struct {
		name     string
		metadata map[string]interface{}
	}{
		{name: "newsletter_signup", metadata: map[string]interface{}{"source": "footer", "email": "user@example.com"}},
		{name: "revenue:purchased", metadata: map[string]interface{}{"price": 2999, "currency": "USD", "product": "premium_plan"}},
		{name: "demo_requested", metadata: map[string]interface{}{"company": "Example Corp", "plan": "enterprise"}},
		{name: "account_created", metadata: map[string]interface{}{"plan": "free", "source": "homepage"}},
		{name: "download_started", metadata: map[string]interface{}{"filename": "whitepaper.pdf", "size": "2.5MB"}},
		{name: "contact_form_submitted", metadata: map[string]interface{}{"subject": "General Inquiry", "page": "/contact"}},
		{name: "free_trial_started", metadata: map[string]interface{}{"plan": "pro", "duration": "14_days"}},
		{name: "pricing_page_viewed", metadata: map[string]interface{}{"plan_viewed": "enterprise", "source": "navigation"}},
	}

	avgPagesPerSession := 4
	numSessions := targetEvents / avgPagesPerSession
	if numSessions < 10 {
		numSessions = 10
	}

	for session := 0; session < numSessions; session++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		journey := journeyTemplates[rand.IntN(len(journeyTemplates))]
		ip := ipPool[rand.IntN(len(ipPool))]
		userAgent := userAgents[rand.IntN(len(userAgents))]
		referrer := referrers[rand.IntN(len(referrers))]

		baseTime := time.Now().Add(-time.Duration(rand.IntN(30*24*60*60)) * time.Second)
		cumulativeTime := time.Duration(0)

		for pageIndex, path := range journey {
			if pageIndex > 0 {
				timeBetweenPages := time.Duration(rand.IntN(110)+10) * time.Second
				cumulativeTime += timeBetweenPages
			}
			timestamp := baseTime.Add(cumulativeTime)

			fullPath := path
			if pageIndex == 0 {
				fullPath = addQueryParams(path)
				fullPath = addUTMParams(fullPath)
			}

			input := &events.CollectEventInput{
				IPAddress:   ip,
				UserAgent:   userAgent,
				ReferrerURL: referrer,
				EventType:   events.EventTypePageView,
				Timestamp:   timestamp,
				RawUrl:      fmt.Sprintf("https://%s%s", baseDomain, fullPath),
			}

			if err := events.CollectEvent(s.DBManager, s.Logger, input); err != nil {
				s.Logger.Error("Failed to collect event during seeding", slog.Any("error", err))
			} else {
				eventsCreated++
			}

			if pageIndex == 0 {
				referrer = ""
			}
		}

		if rand.Float64() < 0.2 && len(journey) > 0 {
			goalEvent := goalEvents[rand.IntN(len(goalEvents))]
			metadataBytes, _ := json.Marshal(goalEvent.metadata)
			timestamp := baseTime.Add(time.Duration(len(journey)) * time.Minute)

			input := &events.CollectEventInput{
				IPAddress:       ip,
				UserAgent:       userAgent,
				ReferrerURL:     "",
				EventType:       events.EventTypeCustomEvent,
				CustomEventName: goalEvent.name,
				CustomEventMeta: string(metadataBytes),
				Timestamp:       timestamp,
				RawUrl:          fmt.Sprintf("https://%s%s", baseDomain, journey[len(journey)-1]),
			}

			if err := events.CollectEvent(s.DBManager, s.Logger, input); err != nil {
				s.Logger.Error("Failed to collect custom event during seeding", slog.Any("error", err))
			}
		}
	}

	s.Logger.Info("Generated journey-based events for website",
		slog.String("domain", website.Domain),
		slog.Int("sessions", numSessions),
		slog.Int("totalEvents", eventsCreated))
	return nil
}

// Run executes the seeding process
func (s *Seeder) Run(ctx context.Context) error {
	start := time.Now()
	s.Logger.Info("Starting database seeding...", slog.Int("eventCount", s.EventCount))

	// Seed user
	user, err := s.seedUser()
	if err != nil {
		return fmt.Errorf("failed to seed user: %w", err)
	}

	// Seed websites
	websites, err := s.seedWebsites(user.ID)
	if err != nil {
		return fmt.Errorf("failed to seed websites: %w", err)
	}

	// Generate realistic data for each website
	for _, website := range websites {
		s.Logger.Info("Generating data for website", slog.String("domain", website.Domain))
		if err := s.generateRealisticData(ctx, website); err != nil {
			return fmt.Errorf("failed to generate data for %s: %w", website.Domain, err)
		}
	}

	// Configure website goals
	if err := s.configureWebsiteGoals(websites); err != nil {
		return fmt.Errorf("failed to configure website goals: %w", err)
	}

	// Process events - This step might be redundant if events are processed elsewhere,
	// but replicating the original logic includes it.
	s.Logger.Info("Processing generated events...")
	if err := s.processAllEvents(); err != nil {
		return fmt.Errorf("failed to process events: %w", err)
	}

	s.Logger.Info("Seeding completed successfully", slog.Duration("elapsed", time.Since(start)))
	return nil
}

// seedUser ensures the default admin user exists
func (s *Seeder) seedUser() (*users.User, error) {
	db := s.DBManager.GetConnection()
	user, err := users.FindByEmail(db, "admin@example.com")

	// If user exists, return it
	if err == nil {
		s.Logger.Info("Admin user already exists", slog.String("email", user.Email))
		return user, nil
	}

	// If error is not "not found", return the error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to check for existing user: %w", err)
	}

	// Create new admin user
	s.Logger.Info("Creating admin user")
	if err := users.CreateAdminUser(db, "admin@example.com", "password"); err != nil {
		return nil, fmt.Errorf("failed to create admin user: %w", err)
	}

	// Fetch the newly created user
	newUser, err := users.FindByEmail(db, "admin@example.com")
	if err != nil {
		return nil, fmt.Errorf("failed to fetch newly created user: %w", err)
	}

	s.Logger.Info("Admin user created successfully", slog.Uint64("id", uint64(uint64(newUser.ID))))
	return newUser, nil
}

// seedWebsites creates multiple websites for testing
func (s *Seeder) seedWebsites(userID uint) ([]*websites.Website, error) {
	var websiteList []*websites.Website
	db := s.DBManager.GetConnection()

	// Define the domains we want to seed
	domains := []string{
		"example.com",
		"blog.example.com",
		"app.example.com",
		"mywebsite.com",
	}

	for _, domain := range domains {
		var website websites.Website

		// Check if website already exists
		if err := db.Where("domain = ?", domain).First(&website).Error; err == nil {
			s.Logger.Info("Website already exists", slog.String("domain", website.Domain))
			websiteList = append(websiteList, &website)
			continue
		}

		// Create new website
		website = websites.Website{
			Domain:    domain,
			CreatedAt: time.Now(),
		}

		// Use PerformWrite for transaction handling
		err := sqlite.PerformWrite(s.Logger, db, func(tx *gorm.DB) error {
			return tx.Create(&website).Error
		})
		if err != nil {
			return nil, fmt.Errorf("failed to create website %s: %w", domain, err)
		}

		s.Logger.Info("Website created successfully", slog.Uint64("id", uint64(uint64(website.ID))), slog.String("domain", website.Domain))
		websiteList = append(websiteList, &website)
	}

	return websiteList, nil
}

// generateRealisticData generates realistic events for a given website with coherent user journeys
func (s *Seeder) generateRealisticData(ctx context.Context, website *websites.Website) error {
	ipPool := generateIPPool(100) // Pool of 100 unique IPs
	userAgents := getUserAgents() // Common user agents
	referrers := getReferrers()   // Common referrers
	baseDomain := website.Domain
	eventsCreated := 0

	numWebsites := 4 // Assuming 4 websites are always seeded; ideally pass this count
	targetEventsPerWebsite := s.EventCount / numWebsites
	if targetEventsPerWebsite == 0 {
		targetEventsPerWebsite = 1 // Ensure at least one event if total is less than numWebsites
	}

	// Define user journey templates - realistic paths users take through the site
	journeyTemplates := [][]string{
		// Homepage -> About -> Contact journey
		{"/", "/about", "/contact"},
		// Homepage -> Features -> Pricing -> Signup journey
		{"/", "/features", "/pricing", "/signup"},
		// Homepage -> Blog -> Article -> Signup journey
		{"/", "/blog", "/blog/article-1", "/signup"},
		// Direct to pricing -> Features -> Signup journey
		{"/pricing", "/features", "/signup"},
		// Product browsing journey
		{"/", "/products", "/products/widget-a", "/products/gadget-b", "/pricing"},
		// Documentation exploration journey
		{"/", "/docs", "/docs/getting-started", "/docs/api-reference"},
		// Blog reading journey
		{"/", "/blog", "/blog/article-1", "/blog/article-2"},
		// Quick signup journey
		{"/", "/signup"},
		// Feature exploration journey
		{"/", "/features", "/pricing", "/docs", "/signup"},
		// Product to signup journey
		{"/products", "/products/widget-a", "/pricing", "/signup"},
		// Long exploration journey
		{"/", "/about", "/features", "/pricing", "/docs/getting-started", "/signup"},
		// Dashboard access journey (returning users)
		{"/login", "/dashboard", "/settings"},
		// Blog to signup journey
		{"/blog/article-1", "/about", "/pricing", "/signup"},
	}

	// Define goal events with their metadata
	goalEvents := []struct {
		name     string
		metadata map[string]interface{}
	}{
		{
			name: "newsletter_signup",
			metadata: map[string]interface{}{
				"source": "footer",
				"email":  "user@example.com",
			},
		},
		{
			name: "revenue:purchased",
			metadata: map[string]interface{}{
				"price":    2999, // $29.99 in cents
				"currency": "USD",
				"product":  "premium_plan",
			},
		},
		{
			name: "demo_requested",
			metadata: map[string]interface{}{
				"company": "Example Corp",
				"plan":    "enterprise",
			},
		},
		{
			name: "account_created",
			metadata: map[string]interface{}{
				"plan":   "free",
				"source": "homepage",
			},
		},
		{
			name: "download_started",
			metadata: map[string]interface{}{
				"filename": "whitepaper.pdf",
				"size":     "2.5MB",
			},
		},
		{
			name: "contact_form_submitted",
			metadata: map[string]interface{}{
				"subject": "General Inquiry",
				"page":    "/contact",
			},
		},
		{
			name: "free_trial_started",
			metadata: map[string]interface{}{
				"plan":     "pro",
				"duration": "14_days",
			},
		},
		{
			name: "pricing_page_viewed",
			metadata: map[string]interface{}{
				"plan_viewed": "enterprise",
				"source":      "navigation",
			},
		},
	}

	// Calculate how many sessions we need to generate
	// Average 4 pages per session to get realistic depth
	avgPagesPerSession := 4
	numSessions := targetEventsPerWebsite / avgPagesPerSession
	if numSessions < 10 {
		numSessions = 10 // Minimum sessions for variety
	}

	// Generate user sessions/journeys
	for session := 0; session < numSessions; session++ {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Select a random journey template
		journey := journeyTemplates[rand.IntN(len(journeyTemplates))]

		// Select random user characteristics (consistent for this session)
		ip := ipPool[rand.IntN(len(ipPool))]
		userAgent := userAgents[rand.IntN(len(userAgents))]
		referrer := referrers[rand.IntN(len(referrers))]

		// Base timestamp for this session (random time in last 30 days)
		// Ensure base time is aligned to avoid session boundary issues
		baseTime := time.Now().Add(-time.Duration(rand.IntN(30*24*60*60)) * time.Second)

		// Cumulative time for this session
		cumulativeTime := time.Duration(0)

		// Generate events for each page in the journey
		for pageIndex, path := range journey {
			// Add some time between page views (10 seconds to 2 minutes)
			// Keep it well under 30 minutes total to ensure same session
			if pageIndex > 0 {
				timeBetweenPages := time.Duration(rand.IntN(110)+10) * time.Second
				cumulativeTime += timeBetweenPages
			}
			timestamp := baseTime.Add(cumulativeTime)

			// Add query parameters and UTM tags to first page only (entry point)
			fullPath := path
			if pageIndex == 0 {
				fullPath = addQueryParams(path)
				fullPath = addUTMParams(fullPath)
			}

			// Use the events.CollectEvent function to simulate event ingestion
			input := &events.CollectEventInput{
				IPAddress:   ip,
				UserAgent:   userAgent,
				ReferrerURL: referrer,
				EventType:   events.EventTypePageView,
				Timestamp:   timestamp,
				RawUrl:      fmt.Sprintf("https://%s%s", baseDomain, fullPath),
			}

			if err := events.CollectEvent(s.DBManager, s.Logger, input); err != nil {
				s.Logger.Error("Failed to collect event during seeding", slog.Any("error", err))
			} else {
				eventsCreated++
			}

			// Only use external referrer for first page, then it's internal navigation
			if pageIndex == 0 {
				referrer = "" // Internal navigation for subsequent pages
			}
		}

		// Optionally add a custom event at the end of some journeys (20% chance)
		if rand.Float64() < 0.2 && len(journey) > 0 {
			goalEvent := goalEvents[rand.IntN(len(goalEvents))]
			metadataBytes, _ := json.Marshal(goalEvent.metadata)

			// Add custom event slightly after last page view
			timestamp := baseTime.Add(time.Duration(len(journey)) * time.Minute)

			input := &events.CollectEventInput{
				IPAddress:       ip,
				UserAgent:       userAgent,
				ReferrerURL:     "",
				EventType:       events.EventTypeCustomEvent,
				CustomEventName: goalEvent.name,
				CustomEventMeta: string(metadataBytes),
				Timestamp:       timestamp,
				RawUrl:          fmt.Sprintf("https://%s%s", baseDomain, journey[len(journey)-1]),
			}

			if err := events.CollectEvent(s.DBManager, s.Logger, input); err != nil {
				s.Logger.Error("Failed to collect custom event during seeding", slog.Any("error", err))
			}
		}
	}

	s.Logger.Info("Generated journey-based events for website",
		slog.String("domain", website.Domain),
		slog.Int("sessions", numSessions),
		slog.Int("totalEvents", eventsCreated))
	return nil
}

// processAllEvents processes all generated events (mimics a background job)
func (s *Seeder) processAllEvents() error {
	batchSize := 100 // Or get from config
	_, err := events.ProcessUnprocessedEvents(s.DBManager, s.Logger, batchSize)
	if err != nil {
		return fmt.Errorf("failed during event processing: %w", err)
	}
	s.Logger.Info("Event processing step completed")
	return nil
}

// configureWebsiteGoals configures goals for each website
func (s *Seeder) configureWebsiteGoals(websites []*websites.Website) error {
	db := s.DBManager.GetConnection()

	// Define goal event names for each website
	goalEventNames := []string{
		"newsletter_signup",
		"purchase_completed",
		"demo_requested",
		"account_created",
		"download_started",
		"contact_form_submitted",
		"free_trial_started",
		"pricing_page_viewed",
	}

	// Create goals configuration
	goalsConfig := make(map[string][]string)
	for _, website := range websites {
		// Assign different goals to different websites for variety
		websiteGoals := make([]string, 0)
		for i, goal := range goalEventNames {
			// Give each website a subset of goals
			if i < 4 || rand.Float64() < 0.3 {
				websiteGoals = append(websiteGoals, goal)
			}
		}
		goalsConfig[strconv.FormatUint(uint64(website.ID), 10)] = websiteGoals
	}

	// Create the JSON structure
	websiteGoalsJSON := map[string]interface{}{
		"goals": goalsConfig,
	}

	// Convert to JSON
	jsonData, err := json.Marshal(websiteGoalsJSON)
	if err != nil {
		return fmt.Errorf("failed to marshal website goals: %w", err)
	}

	// Save to settings
	if err := settings.CreateOrUpdateSetting(db, "website_goals", string(jsonData)); err != nil {
		return fmt.Errorf("failed to save website goals setting: %w", err)
	}

	s.Logger.Info("Website goals configured successfully", slog.Int("websites", len(websites)))
	return nil
}

// --- Helper functions --- (Copied from original seed tool) ---

// generateIPPool creates a pool of unique IPv4 addresses
func generateIPPool(count int) []string {
	ipPool := make(map[string]bool)
	var ips []string
	for len(ips) < count {
		ip := fmt.Sprintf("%d.%d.%d.%d", rand.IntN(255)+1, rand.IntN(256), rand.IntN(256), rand.IntN(256))
		if !ipPool[ip] {
			ipPool[ip] = true
			ips = append(ips, ip)
		}
	}
	return ips
}

// getUserAgents returns a list of common user agent strings
func getUserAgents() []string {
	return []string{
		"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36",
		"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.1 Safari/605.1.15",
		"Mozilla/5.0 (iPhone; CPU iPhone OS 16_1_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.1 Mobile/15E148 Safari/605.1",
		"Mozilla/5.0 (Linux; Android 13; Pixel 7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Mobile Safari/537.36",
		"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/108.0.0.0 Safari/537.36",
		"Mozilla/5.0 (iPad; CPU OS 16_1_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/16.1 Mobile/15E148 Safari/605.1",
		"Googlebot/2.1 (+http://www.google.com/bot.html)",
		"curl/7.81.0",
	}
}

// getReferrers returns a list of common referrer domains
func getReferrers() []string {
	return []string{
		"", // Direct visit
		"https://google.com",
		"https://bing.com",
		"https://duckduckgo.com",
		"https://facebook.com",
		"https://twitter.com",
		"https://linkedin.com",
		"https://github.com",
		"https://some-other-website.com/blog/post",
		"android-app://com.google.android.gm", // Example app referrer
	}
}

// addQueryParams adds random query parameters to a path
func addQueryParams(path string) string {
	// Only add params sometimes (e.g., 30% chance)
	if rand.IntN(10) < 7 {
		return path
	}

	params := url.Values{}
	numParams := rand.IntN(3) + 1 // 1 to 3 params
	possibleParams := []string{"ref", "source", "id", "query", "page", "utm_source"}

	for i := 0; i < numParams; i++ {
		key := possibleParams[rand.IntN(len(possibleParams))]
		value := fmt.Sprintf("value%d", rand.IntN(100))
		// Avoid adding utm_source here if we add it later
		if key == "utm_source" && rand.IntN(2) == 0 {
			continue
		}
		params.Add(key, value)
	}

	if len(params) > 0 {
		return path + "?" + params.Encode()
	}
	return path
}

// addUTMParams adds UTM tracking parameters randomly
func addUTMParams(path string) string {
	// Only add UTM params sometimes (e.g., 20% chance)
	if rand.IntN(10) < 8 {
		return path
	}

	// Use url.Parse to handle existing query params
	u, err := url.Parse(path)
	if err != nil {
		log.Printf("Warning: Failed to parse path for UTM params: %v", err)
		return path // Return original path on error
	}
	params := u.Query()

	utms := []struct {
		key   string
		value []string
	}{
		{"utm_source", []string{"google", "facebook", "newsletter", "twitter", "linkedin"}},
		{"utm_medium", []string{"cpc", "social", "email", "organic", "referral"}},
		{"utm_campaign", []string{"spring_sale", "product_launch", "dev_outreach", "q4_promo"}},
		{"utm_term", []string{"analytics_software", "web_tracking", "golang_dev", ""}}, // Optional
		{"utm_content", []string{"sidebar_ad", "header_link", "footer_banner", ""}},    // Optional
	}

	for _, utm := range utms {
		// Add UTM param sometimes (e.g., 80% chance, but source/medium/campaign always)
		if len(utm.value) > 0 && (rand.IntN(10) < 8 || utm.key == "utm_source" || utm.key == "utm_medium" || utm.key == "utm_campaign") {
			value := utm.value[rand.IntN(len(utm.value))]
			if value != "" {
				params.Set(utm.key, value)
			}
		}
	}

	u.RawQuery = params.Encode()
	return u.String()
}
