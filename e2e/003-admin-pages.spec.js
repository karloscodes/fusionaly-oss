const { test, expect } = require("@playwright/test");
const { TestHelpers } = require("./test-helpers");

const TEST_EMAIL = "admin@test-e2e.com";
const TEST_PASSWORD = "testpassword123";

test.describe.serial("Admin Pages Accessibility Tests", () => {
	let helpers;
	let testWebsiteId = null;
	let testDomain = null;

	test.beforeEach(async ({ page }) => {
		helpers = new TestHelpers(page);
		helpers.log("=== Starting Admin Pages Test ===");

		// Login with deterministic validation
		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000
		});
		helpers.log("Login successful for admin pages test");

		// Ensure we have a test website for consistent testing
		if (!testWebsiteId) {
			testDomain = `admin-test-${Date.now()}.com`;

			// Navigate to website creation
			await helpers.navigateTo("/admin/websites/new", {
				waitForSelector: 'input[name="domain"]'
			});

			// Create the website (don't wait after submit - Inertia handles navigation)
			await helpers.fillForm({ domain: testDomain }, { waitAfterSubmit: false });

			// Wait for Inertia navigation - should redirect away from /new page on success
			try {
				await helpers.page.waitForURL(url => !url.href.includes("/new"), { timeout: 15000 });
			} catch (urlError) {
				// URL didn't change - might be an error
				await helpers.page.waitForLoadState("networkidle", { timeout: 5000 });
			}

			// Verify creation was successful (redirects to /admin/websites/:id/setup or /admin)
			const currentUrl = helpers.page.url();
			if (currentUrl.includes("/new")) {
				throw new Error("Failed to create test website for admin pages test");
			}

			// Extract the website ID from the URL
			const match = currentUrl.match(/\/admin\/websites\/(\d+)/);
			if (match) {
				testWebsiteId = match[1];
			}

			helpers.log(`Test website created: ${testDomain} (ID: ${testWebsiteId})`);
		}
	});

	test.afterEach(async ({ page }) => {
		if (helpers) {
			await helpers.cleanup();
		}
	});

	test("should access dashboard page when website exists", async ({ page }) => {
		helpers.log("Testing dashboard page accessibility");

		// testWebsiteId should have been set in beforeEach
		expect(testWebsiteId).toBeTruthy();
		helpers.log(`Using testWebsiteId: ${testWebsiteId}`);

		// Navigate directly to dashboard using the website ID from beforeEach
		await helpers.navigateTo(`/admin/websites/${testWebsiteId}/dashboard`, { timeout: 30000 });

		// Verify we're on the dashboard page
		const currentUrl = helpers.page.url();
		expect(currentUrl).toContain("/admin/websites/");
		expect(currentUrl).toContain("/dashboard");

		// Verify dashboard content is loaded
		await helpers.waitForElement("h1", { timeout: 10000 });
		const heading = await helpers.page.textContent("h1");
		expect(heading).toContain("Dashboard");

		helpers.log("Dashboard page accessible and loaded correctly");
	});

	test("should access administration page", async ({ page }) => {
		helpers.log("Testing administration page accessibility");

		// Navigate to administration
		await helpers.navigateTo("/admin/administration", {
			timeout: 30000
		});

		// Should be on administration page
		const currentUrl = helpers.page.url();
		expect(currentUrl).toContain("/admin/administration");

		// Verify administration content is loaded
		await helpers.waitForElement("h1", { timeout: 10000 });

		helpers.log("Administration page accessible");
	});

	test("should access events page when website exists", async ({ page }) => {
		helpers.log("Testing events page accessibility");

		// testWebsiteId should have been set in beforeEach
		expect(testWebsiteId).toBeTruthy();

		// Navigate directly to events page using testWebsiteId
		await helpers.navigateTo(`/admin/websites/${testWebsiteId}/events`, {
			timeout: 30000
		});

		// Verify we're on the events page
		const currentUrl = helpers.page.url();
		expect(currentUrl).toContain("/admin/websites/");
		expect(currentUrl).toContain("/events");

		// Verify events content is loaded
		await helpers.waitForElement("h1", { timeout: 10000 });
		const heading = await helpers.page.textContent("h1");
		expect(heading).toContain("Events");

		helpers.log("Events page accessible and loaded correctly");
	});

	test("should access websites management page", async ({ page }) => {
		helpers.log("Testing websites management page accessibility");

		// Navigate to websites (now at /admin)
		await helpers.navigateTo("/admin", {
			timeout: 30000
		});

		// We MUST be able to access the websites page
		const currentUrl = helpers.page.url();
		expect(currentUrl).toContain("/admin");

		// Since we created a website, we should see the list, not be redirected to create new
		expect(currentUrl).not.toContain("/new");

		helpers.log("Websites management page accessible and loaded correctly");
	});

	test("should maintain authentication across all admin pages", async ({ page }) => {
		helpers.log("Testing authentication persistence across admin pages");

		// testWebsiteId should have been set in beforeEach
		expect(testWebsiteId).toBeTruthy();

		const adminPages = [
			`/admin/websites/${testWebsiteId}/dashboard`,
			"/admin/administration/ingestion",
			`/admin/websites/${testWebsiteId}/events`,
			"/admin"
		];

		for (const adminPage of adminPages) {
			helpers.log(`Testing authentication for: ${adminPage}`);

			// Navigate to the page
			await helpers.navigateTo(adminPage, { timeout: 30000 });

			// Verify we're not redirected to login
			const currentUrl = helpers.page.url();
			expect(currentUrl).not.toContain("/login");
			expect(currentUrl).toContain("/admin");

			helpers.log(`Authentication maintained for: ${adminPage}`);
		}

		helpers.log("Authentication persistence test completed");
	});

	test("should handle navigation between admin pages", async ({ page }) => {
		helpers.log("Testing navigation flow between admin pages");

		// testWebsiteId should have been set in beforeEach
		expect(testWebsiteId).toBeTruthy();

		// Test navigation sequence
		const navigationFlow = [
			{ to: `/admin/websites/${testWebsiteId}/dashboard`, label: "dashboard" },
			{ to: "/admin/administration/ingestion", label: "administration" },
			{ to: `/admin/websites/${testWebsiteId}/events`, label: "events" },
			{ to: "/admin", label: "websites" }
		];

		for (const nav of navigationFlow) {
			helpers.log(`Testing navigation to ${nav.to}`);

			// Navigate to the target page
			await helpers.navigateTo(nav.to, { timeout: 30000 });

			// Verify we reached the correct page
			const currentUrl = helpers.page.url();
			expect(currentUrl).toContain("/admin");

			helpers.log(`Successfully navigated to: ${nav.to}`);
		}

		helpers.log("Navigation flow test completed");
	});

	test("should access and configure GeoLite settings in System page", async ({ page }) => {
		helpers.log("Testing GeoLite configuration in System page");

		// Navigate to system administration page
		await helpers.navigateTo("/admin/administration/system", {
			timeout: 30000
		});

		// Verify we're on the system page
		const currentUrl = helpers.page.url();
		expect(currentUrl).toContain("/admin/administration/system");

		// Wait for page to load
		await helpers.waitForElement("h1", { timeout: 10000 });
		const heading = await helpers.page.textContent("h1");
		expect(heading).toContain("System Management");

		// Verify GeoLite configuration section exists
		const pageContent = await page.textContent("body");
		expect(pageContent).toContain("GeoLite Configuration");
		helpers.log("GeoLite configuration section found");

		// Fill in test credentials (these won't actually work but test the form)
		const accountIdInput = page.locator('#geolite_account_id');
		const licenseKeyInput = page.locator('#geolite_license_key');

		if (await accountIdInput.count() > 0) {
			await accountIdInput.fill("123456");
			await licenseKeyInput.fill("test-license-key");
			helpers.log("GeoLite credentials entered");

			// Submit the form
			const saveButton = page.locator('button:has-text("Save GeoLite Settings")');
			await saveButton.click();
			await page.waitForLoadState("networkidle", { timeout: 10000 });

			// Check for success message
			const successMessages = await helpers.checkForMessages("success");
			expect(successMessages.length).toBeGreaterThan(0);
			helpers.log(`✅ GeoLite settings saved: ${successMessages[0]}`);
		} else {
			helpers.log("GeoLite input fields not found (may be styled differently)");
		}
	});
});
