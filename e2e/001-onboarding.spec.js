// e2e/onboarding.spec.js - MUST RUN FIRST to set up user account
const { test, expect } = require("@playwright/test");
const { TestHelpers } = require("./test-helpers");

// Global test credentials - shared across all test files
const TEST_EMAIL = "admin@test-e2e.com";
const TEST_PASSWORD = "testpassword123";

test.describe("Onboarding Flow - MUST RUN FIRST", () => {
	let helpers;

	test.beforeEach(async ({ page }) => {
		helpers = new TestHelpers(page);
		helpers.log("=== ONBOARDING SETUP - Creating User Account ===");
	});

	test.afterEach(async ({ page }) => {
		if (helpers) {
			await helpers.cleanup();
		}
	});

	test("1. Complete Onboarding Flow - Creates user account for all other tests", async ({ page }) => {
		helpers.log("=== PHASE 1: ONBOARDING SETUP (OSS Version) ===");

		// Clear any existing data
		await page.context().clearCookies();
		await page.context().clearPermissions();

		// Step 1.1: Start onboarding - OSS version has 3 steps (email, password, completed)
		await helpers.navigateTo("/setup", {
			waitForSelector: 'input[name="email"]',
			timeout: 30000
		});

		// Verify we're on setup page
		const pageContent = await page.textContent('body');
		expect(pageContent).toContain('Initial Setup');
		helpers.log("Setup page loaded");

		// Step 1.2: User account setup (email-based)
		await helpers.fillForm({
			email: TEST_EMAIL
		});

		// Click Continue button
		await page.click('button[type="submit"]');
		await page.waitForTimeout(2000);
		helpers.log(`User account configured with email: ${TEST_EMAIL}`);

		// Step 1.3: Password setup
		await page.waitForSelector('input[name="password"]', { timeout: 10000 });
		await helpers.fillForm({
			password: TEST_PASSWORD,
			confirm_password: TEST_PASSWORD
		});

		// Click Complete Setup button - wait for it to be enabled first
		const submitButton = page.locator('button[type="submit"]');
		await submitButton.waitFor({ state: 'visible', timeout: 10000 });

		// Only click if button is not already processing
		const buttonText = await submitButton.textContent();
		if (!buttonText?.includes('Creating')) {
			await submitButton.click();
		}
		helpers.log("Password configured");

		// Final redirect check - should be logged in
		await page.waitForURL(/\/admin\/websites\/new/, { timeout: 15000 });
		const finalUrl = page.url();
		expect(finalUrl).not.toContain("/setup");
		expect(finalUrl).not.toContain("/login");

		helpers.log(`USER ACCOUNT CREATED: ${TEST_EMAIL} / ${TEST_PASSWORD}`);
		helpers.log(`Final URL: ${finalUrl}`);
		helpers.log("Onboarding completed - all other tests can now use this account");
	});
});
