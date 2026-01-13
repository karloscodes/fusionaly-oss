const { test, expect } = require("@playwright/test");
const { TestHelpers } = require("./test-helpers");

const TEST_EMAIL = "admin@test-e2e.com";
const TEST_PASSWORD = "testpassword123";

test.describe("Website Management Flow", () => {
	let helpers;
	let testDomain = null;

	test.beforeEach(async ({ page }) => {
		helpers = new TestHelpers(page);
		helpers.log("=== Starting Website Management Test ===");

		// Login with deterministic validation
		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000
		});
		helpers.log("✅ Login successful for website management test");
	});

	test.afterEach(async ({ page }) => {
		if (helpers) {
			await helpers.cleanup();
		}
	});

	// Helper function to create a website with deterministic validation
	async function createWebsite(domainName) {
		helpers.log(`Creating website with domain: ${domainName}`);

		// Navigate to website creation page
		await helpers.navigateTo("/admin/websites/new", {
			waitForSelector: 'input[name="domain"]'
		});

		// Fill and submit the form (don't wait after submit - Inertia handles navigation)
		await helpers.fillForm({ domain: domainName }, { waitAfterSubmit: false });

		// Wait for Inertia navigation - should redirect away from /new page on success
		try {
			await helpers.page.waitForURL(url => !url.href.includes("/new"), { timeout: 15000 });
		} catch (urlError) {
			// URL didn't change - check for error messages
			await helpers.page.waitForLoadState("networkidle", { timeout: 5000 });
		}

		// Determine success based on URL - this is deterministic
		// After creation, redirects to /admin/websites/:id/setup
		const currentUrl = helpers.page.url();
		if (currentUrl.includes("/admin/websites") && currentUrl.includes("/setup")) {
			helpers.log(`✅ Website created successfully: ${domainName}`);
			return { success: true, domain: domainName, url: currentUrl };
		} else if (currentUrl.includes("/admin") && !currentUrl.includes("/new")) {
			helpers.log(`✅ Website created successfully: ${domainName}`);
			return { success: true, domain: domainName, url: currentUrl };
		} else {
			// Check for error messages if we're still on the creation page
			const errorMessages = await helpers.checkForMessages("error");
			if (errorMessages.length > 0) {
				throw new Error(`Website creation failed: ${errorMessages[0]}`);
			} else {
				throw new Error(`Website creation failed: Still on creation page with no error message`);
			}
		}
	}

	test("should create a new website successfully", async ({ page }) => {
		helpers.log("Testing website creation");

		// Generate unique domain name
		testDomain = `test-website-${Date.now()}.com`;

		const result = await createWebsite(testDomain);
		expect(result.success).toBe(true);
		expect(result.domain).toBe(testDomain);
		expect(result.url).toContain("/admin");

		helpers.log("✅ Website creation test passed");
	});

	test("should display websites list when websites exist", async ({ page }) => {
		helpers.log("Testing websites list display with existing websites");

		// First, create a website to ensure the list has content
		const testDomain = `list-test-${Date.now()}.com`;
		await createWebsite(testDomain);
		helpers.log("Test website created for list display");

		// Navigate to websites list (now at /admin)
		await helpers.navigateTo("/admin", {
			timeout: 30000
		});

		const currentUrl = page.url();

		// With websites existing, we should see the list, not be redirected to /new
		expect(currentUrl).toContain("/admin");
		expect(currentUrl).not.toContain("/new");
		helpers.log("Websites list page loaded correctly");

		// Verify we can see website management elements - wait for page to fully load
		await helpers.page.waitForLoadState("networkidle");

		// Look for the "Your Websites" heading that indicates the list section
		await helpers.waitForElement('text=Your Websites', { timeout: 10000 });
		helpers.log("Website list section found");
	});

	test("should handle website navigation correctly", async ({ page }) => {
		helpers.log("Testing website navigation behavior");

		// Create a website first to ensure list view is shown
		const testDomain = `nav-test-${Date.now()}.com`;
		await createWebsite(testDomain);
		helpers.log("Test website created");

		// Navigate to websites list (now at /admin)
		await helpers.navigateTo("/admin", {
			timeout: 30000
		});

		const currentUrl = page.url();

		// With a website created, we MUST see the list, not be redirected to /new
		expect(currentUrl).toContain("/admin");
		expect(currentUrl).not.toContain("/new");
		helpers.log("Websites list displayed");

		// Verify the websites list section is visible
		await helpers.page.waitForLoadState("networkidle");
		await helpers.waitForElement('text=Your Websites', { timeout: 5000 });
		helpers.log("Website list section found");
	});

	test("should reject duplicate domain creation", async ({ page }) => {
		helpers.log("Testing duplicate domain validation");

		const duplicateDomain = `duplicate-test-${Date.now()}.com`;

		// Create first website
		await createWebsite(duplicateDomain);
		helpers.log("✅ First website created successfully");

		// Try to create second website with same domain - this MUST fail
		helpers.log("Attempting to create duplicate website");

		await helpers.navigateTo("/admin/websites/new", {
			waitForSelector: 'input[name="domain"]'
		});

		await helpers.fillForm({ domain: duplicateDomain }, { waitAfterSubmit: false });

		// Wait for Inertia to process - either redirect or stay on page with error
		try {
			await helpers.page.waitForURL(url => !url.href.includes("/new"), { timeout: 5000 });
		} catch (urlError) {
			// Expected - should stay on page with error
			await helpers.page.waitForLoadState("networkidle", { timeout: 5000 });
		}

		// This MUST show an error - no uncertainty
		const currentUrl = helpers.page.url();
		if (currentUrl.includes("/admin/websites/new")) {
			// Still on creation page - check for error message
			const errorMessages = await helpers.checkForMessages("error");
			expect(errorMessages.length).toBeGreaterThan(0);
			helpers.log(`✅ Duplicate domain properly rejected: ${errorMessages[0]}`);
		} else {
			throw new Error("Duplicate domain was incorrectly accepted");
		}
	});

	test("should validate domain format correctly", async ({ page }) => {
		helpers.log("Testing domain format validation");

		await helpers.navigateTo("/admin/websites/new", {
			waitForSelector: 'input[name="domain"]'
		});

		const domainInput = helpers.page.locator('input[name="domain"]');
		const submitButton = helpers.page.locator('button[type="submit"]');

		// Test 1: Empty domain - submit button MUST be disabled
		await domainInput.fill("");
		await helpers.page.waitForTimeout(100);
		expect(await submitButton.isDisabled()).toBe(true);
		helpers.log("✅ Empty domain correctly disables submit button");

		// Test 2: Invalid domain - submit button MUST be disabled
		await domainInput.fill("invalid-domain");
		await helpers.page.waitForTimeout(100);
		expect(await submitButton.isDisabled()).toBe(true);
		helpers.log("✅ Invalid domain correctly disables submit button");

		// Test 3: Valid domain - submit button MUST be enabled
		await domainInput.fill("valid-domain.com");
		await helpers.page.waitForTimeout(100);
		expect(await submitButton.isEnabled()).toBe(true);
		helpers.log("✅ Valid domain correctly enables submit button");
	});

	test("should navigate between website management pages", async ({ page }) => {
		helpers.log("Testing navigation between website management pages");

		const navigationTests = [
			{
				path: "/admin",
				name: "Websites List",
				validation: async () => {
					const url = helpers.page.url();
					// Either we see the list or we're redirected to create new
					return url.includes("/admin");
				}
			},
			{
				path: "/admin/websites/new",
				name: "New Website",
				validation: async () => {
					await helpers.waitForElement('input[name="domain"]', { timeout: 5000 });
					return helpers.page.url().includes("/admin/websites/new");
				}
			},
			{
				path: "/admin/websites/new",
				name: "New Website (second visit)",
				validation: async () => {
					await helpers.waitForElement('input[name="domain"]', { timeout: 5000 });
					return helpers.page.url().includes("/admin/websites/new");
				}
			}
		];

		for (const navTest of navigationTests) {
			helpers.log(`Testing navigation to ${navTest.name}: ${navTest.path}`);

			await helpers.navigateTo(navTest.path, { timeout: 20000 });

			const isValid = await navTest.validation();
			expect(isValid).toBe(true);

			helpers.log(`✅ Successfully validated navigation to ${navTest.name}`);
		}
	});

	test("should complete website creation and verify accessibility", async ({ page }) => {
		helpers.log("Testing complete website creation flow");

		const e2eDomain = `e2e-test-${Date.now()}.com`;

		// Step 1: Create website - this MUST succeed
		const result = await createWebsite(e2eDomain);
		expect(result.success).toBe(true);
		helpers.log("✅ Step 1: Website created successfully");

		// Step 2: Verify we can access the websites list (now at /admin)
		await helpers.navigateTo("/admin", { timeout: 20000 });
		const websitesUrl = helpers.page.url();
		expect(websitesUrl.includes("/admin")).toBe(true);
		helpers.log("✅ Step 2: Websites list accessible");

		// Step 3: Verify we can access the websites list again
		await helpers.navigateTo("/admin", { timeout: 20000 });
		const adminUrl = helpers.page.url();
		// With a website created, we should see the admin page
		expect(adminUrl.includes("/admin")).toBe(true);
		helpers.log("✅ Step 3: Admin page accessible after website creation");

		helpers.log("✅ End-to-end website creation flow completed successfully");
	});

	test("should edit website settings (conversion goals and subdomain tracking)", async ({ page }) => {
		helpers.log("Testing website edit functionality");

		// Create a website first
		const editDomain = `edit-test-${Date.now()}.com`;
		const result = await createWebsite(editDomain);
		expect(result.success).toBe(true);
		helpers.log("Test website created for editing");

		// Extract website ID from URL (e.g., /admin/websites/123/setup)
		const setupUrl = result.url;
		const websiteIdMatch = setupUrl.match(/\/admin\/websites\/(\d+)/);
		expect(websiteIdMatch).toBeTruthy();
		const websiteId = websiteIdMatch[1];
		helpers.log(`Website ID: ${websiteId}`);

		// Navigate to edit page
		await helpers.navigateTo(`/admin/websites/${websiteId}/edit`, {
			waitForSelector: 'h1',
			timeout: 20000
		});

		// Verify we're on the edit page
		const currentUrl = helpers.page.url();
		expect(currentUrl).toContain(`/admin/websites/${websiteId}/edit`);
		helpers.log("Edit page loaded");

		// The domain should be displayed (read-only)
		const pageContent = await page.textContent('body');
		expect(pageContent).toContain(editDomain);
		helpers.log("Domain is displayed on edit page");

		// Toggle subdomain tracking if the switch exists
		const subdomainSwitch = page.locator('button[role="switch"]');
		if (await subdomainSwitch.count() > 0) {
			await subdomainSwitch.click();
			helpers.log("Subdomain tracking toggle clicked");
		}

		// Submit the form
		const saveButton = page.locator('button[type="submit"]');
		await saveButton.click();
		await page.waitForLoadState("networkidle", { timeout: 10000 });
		helpers.log("Form submitted");

		// Verify we stayed on edit page or got success message
		const afterSubmitUrl = page.url();
		expect(afterSubmitUrl).toContain(`/admin/websites/${websiteId}/edit`);

		// Check for success flash message
		const successMessages = await helpers.checkForMessages("success");
		expect(successMessages.length).toBeGreaterThan(0);
		helpers.log(`✅ Website settings saved successfully: ${successMessages[0]}`);
	});
});


