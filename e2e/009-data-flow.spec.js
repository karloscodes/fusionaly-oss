const { test, expect } = require("@playwright/test");
const { TestHelpers } = require("./test-helpers");

const TEST_EMAIL = "admin@test-e2e.com";
const TEST_PASSWORD = "testpassword123";

test.describe.serial("Dashboard Data Display Tests", () => {
	let helpers;
	let websiteId = null;

	test.beforeEach(async ({ page }) => {
		helpers = new TestHelpers(page);
		helpers.log("=== Starting Dashboard Data Display Test ===");
	});

	test.afterEach(async ({ page }) => {
		if (helpers) {
			await helpers.cleanup();
		}
	});

	test("should create a test website and access its dashboard", async ({ page }) => {
		helpers.log("Creating test website for dashboard tests");

		// Login
		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000
		});

		// Create test website
		const websiteDomain = `dashboard-data-test-${Date.now()}.com`;
		await helpers.navigateTo("/admin/websites/new", {
			waitForSelector: 'input[name="domain"]'
		});
		await helpers.fillForm({ domain: websiteDomain }, { waitAfterSubmit: false });
		await page.waitForURL(url => url.href.includes("/setup"), { timeout: 15000 });

		// Extract website ID
		const setupUrl = page.url();
		const match = setupUrl.match(/\/admin\/websites\/(\d+)\/setup/);
		if (match) {
			websiteId = match[1];
		}
		helpers.log(`Created test website: ${websiteDomain} (ID: ${websiteId})`);

		expect(websiteId).toBeTruthy();
	});

	test("should display dashboard with all main sections", async ({ page }) => {
		helpers.log("Testing dashboard main sections display");

		if (!websiteId) {
			test.skip();
			return;
		}

		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000
		});

		// Navigate to dashboard
		await helpers.navigateTo(`/admin/websites/${websiteId}/dashboard`, {
			timeout: 30000
		});

		await page.waitForSelector('text=Dashboard', { timeout: 10000 });
		helpers.log("Dashboard loaded");

		// Verify main sections exist
		const sections = [
			'Visitors',
			'Page Views',
			'Sessions',
			'Bounce Rate',
			'Avg Time',
			'Revenue'
		];

		for (const section of sections) {
			const element = page.locator(`text=${section}`).first();
			await expect(element).toBeVisible({ timeout: 5000 });
			helpers.log(`Section "${section}" visible`);
		}

		helpers.log("All main dashboard sections displayed");
	});

	test("should display Pages section with tabs", async ({ page }) => {
		helpers.log("Testing Pages section");

		if (!websiteId) {
			test.skip();
			return;
		}

		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000
		});

		await helpers.navigateTo(`/admin/websites/${websiteId}/dashboard`, {
			timeout: 30000
		});

		await page.waitForSelector('text=Dashboard', { timeout: 10000 });

		// Find the Pages section
		const pagesSection = page.locator('text=Pages').first();
		await expect(pagesSection).toBeVisible({ timeout: 5000 });

		// Verify tab buttons exist
		const topPagesTab = page.locator('button:has-text("Top Pages")');
		const entryPagesTab = page.locator('button:has-text("Entry Pages")');
		const exitPagesTab = page.locator('button:has-text("Exit Pages")');

		await expect(topPagesTab).toBeVisible();
		await expect(entryPagesTab).toBeVisible();
		await expect(exitPagesTab).toBeVisible();

		// Click through tabs to verify they work
		await entryPagesTab.click();
		await page.waitForTimeout(300);
		await exitPagesTab.click();
		await page.waitForTimeout(300);
		await topPagesTab.click();

		helpers.log("Pages section with tabs working correctly");
	});

	test("should display Countries section", async ({ page }) => {
		helpers.log("Testing Countries section");

		if (!websiteId) {
			test.skip();
			return;
		}

		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000
		});

		await helpers.navigateTo(`/admin/websites/${websiteId}/dashboard`, {
			timeout: 30000
		});

		await page.waitForSelector('text=Dashboard', { timeout: 10000 });

		// Scroll to find countries section
		await page.evaluate(() => window.scrollTo(0, 500));
		await page.waitForTimeout(500);

		// Verify Countries section
		const countriesSection = page.locator('text=Countries').first();
		await expect(countriesSection).toBeVisible({ timeout: 5000 });

		helpers.log("Countries section displayed correctly");
	});

	test("should display Device Analytics section with tabs", async ({ page }) => {
		helpers.log("Testing Device Analytics section");

		if (!websiteId) {
			test.skip();
			return;
		}

		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000
		});

		await helpers.navigateTo(`/admin/websites/${websiteId}/dashboard`, {
			timeout: 30000
		});

		await page.waitForSelector('text=Dashboard', { timeout: 10000 });

		// Scroll to find devices section
		await page.evaluate(() => window.scrollTo(0, 500));
		await page.waitForTimeout(500);

		// Verify Device Analytics section
		const devicesSection = page.locator('text=Device Analytics').first();
		await expect(devicesSection).toBeVisible({ timeout: 5000 });

		// Check device tabs
		const devicesTab = page.locator('button:has-text("Devices")');
		const browsersTab = page.locator('button:has-text("Browsers")');
		const osTab = page.locator('button:has-text("OSs")');

		await expect(devicesTab).toBeVisible();
		await expect(browsersTab).toBeVisible();
		await expect(osTab).toBeVisible();

		helpers.log("Device Analytics section displayed correctly");
	});

	test("should display Events section", async ({ page }) => {
		helpers.log("Testing Events section");

		if (!websiteId) {
			test.skip();
			return;
		}

		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000
		});

		await helpers.navigateTo(`/admin/websites/${websiteId}/dashboard`, {
			timeout: 30000
		});

		await page.waitForSelector('text=Dashboard', { timeout: 10000 });

		// Scroll to events section
		await page.evaluate(() => window.scrollTo(0, 800));
		await page.waitForTimeout(500);

		// Verify Events section header
		const eventsSection = page.locator('text=Events').first();
		await expect(eventsSection).toBeVisible({ timeout: 5000 });

		helpers.log("Events section displayed correctly");
	});

	test("should display Visitor Flows section (free feature)", async ({ page }) => {
		helpers.log("Testing Visitor Flows section");

		if (!websiteId) {
			test.skip();
			return;
		}

		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000
		});

		await helpers.navigateTo(`/admin/websites/${websiteId}/dashboard`, {
			timeout: 30000
		});

		await page.waitForSelector('text=Dashboard', { timeout: 10000 });

		// Scroll to bottom where Visitor Flows is located
		await page.evaluate(() => window.scrollTo(0, document.body.scrollHeight));
		await page.waitForTimeout(1500); // Wait for deferred loading

		// Verify Visitor Flows section is visible
		const visitorFlows = page.locator('text=Visitor Flows').first();
		await expect(visitorFlows).toBeVisible({ timeout: 10000 });

		helpers.log("Visitor Flows section displayed");
	});

	test("should have working time range selector", async ({ page }) => {
		helpers.log("Testing time range selector");

		if (!websiteId) {
			test.skip();
			return;
		}

		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000
		});

		await helpers.navigateTo(`/admin/websites/${websiteId}/dashboard`, {
			timeout: 30000
		});

		await page.waitForSelector('text=Dashboard', { timeout: 10000 });

		// Find time range selector button
		const timeRangeButton = page.locator('button:has-text("Last")').first();
		await expect(timeRangeButton).toBeVisible({ timeout: 5000 });

		// Click to open dropdown
		await timeRangeButton.click();
		await page.waitForTimeout(500);

		// Verify time range options are available
		const todayOption = page.locator('text=Today').first();
		await expect(todayOption).toBeVisible({ timeout: 5000 });

		// Click Today to select it
		await todayOption.click();
		await page.waitForTimeout(500);

		// URL should now contain range parameter
		const currentUrl = page.url();
		expect(currentUrl).toContain('range=today');

		helpers.log("Time range selector working correctly");
	});
});
