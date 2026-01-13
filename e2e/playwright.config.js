// e2e/playwright.config.js
const { devices } = require("@playwright/test");

// Make sure we're using the test environment
process.env.FUSIONALY_ENV = "test";

module.exports = {
	testDir: "./",
	testMatch: [
		"**/*.spec.js"
	], // All tests run in numeric order (001, 002, etc.)
	fullyParallel: false, // Sequential execution due to test dependencies
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 1 : 0, // Retry once on CI only
	workers: 1, // Single worker - tests have dependencies (onboarding must run first)
	reporter: process.env.CI ? "github" : "line", // Faster line reporter locally
	timeout: 45000, // 45s timeout (reduced from 60s)
	expect: {
		timeout: 8000, // 8s expect timeout (reduced from 10s)
	},
	use: {
		baseURL: "http://localhost:3000",
		trace: process.env.CI ? "on-first-retry" : "off", // No trace locally for speed
		video: "off", // Disabled for speed - use screenshots
		screenshot: "only-on-failure",
		actionTimeout: 10000, // 10s action timeout (reduced from 15s)
		navigationTimeout: 20000, // 20s navigation timeout (reduced from 30s)
		extraHTTPHeaders: {
			"X-Test-Source": "playwright-e2e",
		},
	},
	projects: [
		{
			name: "chromium",
			use: {
				...devices["Desktop Chrome"],
				// Add viewport for consistency
				viewport: { width: 1280, height: 720 },
				// Disable animations for more reliable tests
				reducedMotion: "reduce",
			},
		},
		// Add more browsers if needed for cross-browser testing
		// {
		//   name: 'firefox',
		//   use: { ...devices['Desktop Firefox'] },
		// },
		// {
		//   name: 'webkit',
		//   use: { ...devices['Desktop Safari'] },
		// },
	],
	// Run your local dev server before starting the tests.
	webServer: {
		command: "cd .. && mkdir -p tmp/go-cache && FUSIONALY_ENV=test LOG_LEVEL=error go run cmd/fusionaly/main.go",
		url: "http://localhost:3000/_health",
		reuseExistingServer: !process.env.CI,
		timeout: 90000, // 90s server startup (reduced from 120s)
		ignoreHTTPSErrors: true,
		retries: 2, // 2 retries instead of 3
		env: {
			...process.env,
			FUSIONALY_ENV: "test",
			LOG_LEVEL: "error",
			GOCACHE: process.cwd() + "/../tmp/go-cache",
		},
	},
	// Global setup and teardown
	globalSetup: require.resolve("./setup-test-env.js"),
};
