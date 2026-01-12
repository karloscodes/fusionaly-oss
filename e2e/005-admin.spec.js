const { test, expect } = require("@playwright/test");
const { TestHelpers } = require("./test-helpers");

const TEST_EMAIL = "admin@test-e2e.com";
const TEST_PASSWORD = "testpassword123";

test.describe.serial("Administration Pages Tests", () => {
	let helpers;

	test.beforeEach(async ({ page }) => {
		helpers = new TestHelpers(page);
		helpers.log("=== Starting Administration Test ===");

		// Login with deterministic validation
		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000
		});
		helpers.log("Login successful for administration test");
	});

	test.afterEach(async ({ page }) => {
		if (helpers) {
			await helpers.cleanup();
		}
	});

	test("should navigate to Administration section and see all pages", async ({ page }) => {
		helpers.log("Testing administration navigation");

		// Navigate to administration
		await helpers.navigateTo("/admin/administration/ingestion", {
			waitForSelector: 'h1:has-text("Ingestion Settings")',
			timeout: 30000
		});
		helpers.log("Administration page loaded");

		// Verify sidebar navigation is present
		const sidebar = await page.locator('aside:has-text("Administration")');
		await expect(sidebar).toBeVisible();

		// Verify all navigation links are present in the administration sidebar
		const ingestionLink = await page.locator('aside:has-text("Administration") a:has-text("Ingestion")');
		const aiLink = await page.locator('aside:has-text("Administration") a:has-text("AI")');
		const accountLink = await page.locator('aside:has-text("Administration") a:has-text("Account")');
		const systemLink = await page.locator('aside:has-text("Administration") a:has-text("System")');

		await expect(ingestionLink).toBeVisible();
		await expect(aiLink).toBeVisible();
		await expect(accountLink).toBeVisible();
		await expect(systemLink).toBeVisible();

		helpers.log("All sidebar navigation links are visible");
	});

	test("should update Ingestion settings via form submission", async ({ page }) => {
		helpers.log("Testing Ingestion page form submission");

		// Navigate to Ingestion page
		await helpers.navigateTo("/admin/administration/ingestion", {
			waitForSelector: 'h1:has-text("Ingestion Settings")',
			timeout: 30000
		});

		// Wait for the textarea to be visible
		await page.waitForSelector('textarea[name="excluded_ips"]', { state: "visible", timeout: 10000 });

		// Get current value from textarea
		const currentValue = await page.inputValue('textarea[name="excluded_ips"]');
		helpers.log(`Current excluded IPs: ${currentValue}`);

		// Update with test IP
		const testIP = "192.168.1.100, 10.0.0.1";
		await page.fill('textarea[name="excluded_ips"]', testIP);

		// Submit the form (Inertia form submission)
		await page.click('button[type="submit"]:has-text("Save Filtering")');

		// Wait for page to reload/update with success message
		await page.waitForSelector('text=/Ingestion settings/', {
			timeout: 10000
		});
		helpers.log("Success flash message displayed");

		// Verify value persisted
		const updatedValue = await page.inputValue('textarea[name="excluded_ips"]');
		expect(updatedValue).toBe(testIP);
		helpers.log("Ingestion settings updated successfully");
	});

	// Note: AI page test removed - AI features only exist in Pro version
	// Note: License key input test is removed - License management is a Pro feature

	test("should display password change form in Account page", async ({ page }) => {
		helpers.log("Testing password change form display");

		// Navigate to Account page
		await helpers.navigateTo("/admin/administration/account", {
			waitForSelector: 'h1:has-text("Account Settings")',
			timeout: 30000
		});

		// Wait for the password change card to be visible
		await page.waitForSelector('text=Change Password', { state: "visible", timeout: 10000 });

		// Find password inputs by their label text (inputs don't have name attributes in React controlled forms)
		// Use exact text matching to avoid "New Password" matching "Confirm New Password"
		const currentPasswordLabel = page.getByText('Current Password', { exact: true });
		const newPasswordLabel = page.getByText('New Password', { exact: true });
		const confirmPasswordLabel = page.getByText('Confirm New Password', { exact: true });

		// Verify all labels are visible (which means their inputs are also present)
		await expect(currentPasswordLabel).toBeVisible();
		await expect(newPasswordLabel).toBeVisible();
		await expect(confirmPasswordLabel).toBeVisible();

		// Verify password inputs exist (find them relative to their labels)
		const passwordInputs = page.locator('input[type="password"]');
		const inputCount = await passwordInputs.count();
		expect(inputCount).toBeGreaterThanOrEqual(3);
		helpers.log(`Found ${inputCount} password input fields`);

		// Verify the Update Password button
		const updateButton = page.locator('button[type="submit"]:has-text("Update Password")');
		await expect(updateButton).toBeVisible();

		helpers.log("Password change form displayed correctly");
	});

	test("should navigate between administration pages", async ({ page }) => {
		helpers.log("Testing navigation between administration pages");

		// Start at Ingestion
		await helpers.navigateTo("/admin/administration/ingestion", {
			waitForSelector: 'h1:has-text("Ingestion Settings")',
			timeout: 30000
		});

		// Navigate to AI (use sidebar to avoid collision with main nav)
		// OSS version shows paywall instead of form
		await page.click('aside:has-text("Administration") a:has-text("AI")');
		await page.waitForSelector('text=AI Features are Pro Only', { state: "visible", timeout: 5000 });
		helpers.log("Navigated to AI page (paywall)");

		// Navigate to Account
		await page.click('aside:has-text("Administration") a:has-text("Account")');
		await page.waitForSelector('text=Change Password', { state: "visible", timeout: 5000 });
		helpers.log("Navigated to Account page");

		// Navigate to System
		await page.click('aside:has-text("Administration") a:has-text("System")');
		await page.waitForSelector('h1:has-text("System Management")', { timeout: 5000 });
		helpers.log("Navigated to System page");

		// Navigate back to Ingestion
		await page.click('aside:has-text("Administration") a:has-text("Ingestion")');
		await page.waitForSelector('textarea[name="excluded_ips"]', { state: "visible", timeout: 10000 });
		helpers.log("Navigated back to Ingestion page");

		helpers.log("Page navigation handled correctly");
	});

	test("should display System Management page with all features", async ({ page }) => {
		helpers.log("Testing System Management page");

		// Navigate to System page
		await helpers.navigateTo("/admin/administration/system", {
			waitForSelector: 'h1:has-text("System Management")',
			timeout: 30000
		});

		// Verify Cache Management section
		const purgeCacheButton = await page.locator('button:has-text("Purge All Caches")');
		await expect(purgeCacheButton).toBeVisible();
		helpers.log("Cache purge button visible");

		// Verify Database Export section
		const exportDbButton = await page.locator('button:has-text("Export Database")');
		await expect(exportDbButton).toBeVisible();
		helpers.log("Database export button visible");

		// Verify Application Logs section - just check that "Application Logs" text exists
		const logsHeading = await page.locator('text=Application Logs').first();
		await expect(logsHeading).toBeVisible();
		helpers.log("Application Logs section visible");

		helpers.log("System Management page displayed correctly");
	});

	test("should successfully purge caches", async ({ page }) => {
		helpers.log("Testing cache purge functionality");

		// Navigate to System page
		await helpers.navigateTo("/admin/administration/system", {
			waitForSelector: 'h1:has-text("System Management")',
			timeout: 30000
		});

		// Click purge cache button and confirm
		page.once('dialog', dialog => dialog.accept());
		await page.click('button:has-text("Purge All Caches")');

		// Wait for success message
		await page.waitForSelector('text=All caches have been purged successfully', {
			timeout: 10000
		});
		helpers.log("Cache purge completed successfully");
	});

	test("should have database export button available", async ({ page }) => {
		helpers.log("Testing database export button is available");

		// Navigate to System page
		await helpers.navigateTo("/admin/administration/system", {
			waitForSelector: 'h1:has-text("System Management")',
			timeout: 30000
		});

		// Verify the Export Database button is visible
		const exportButton = await page.locator('button:has-text("Export Database")');
		await expect(exportButton).toBeVisible();

		helpers.log("Database export button is available");
	});
});
