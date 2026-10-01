// e2e/007-feed.spec.js - Activity feed home
// Relies on the user created by 001-onboarding.
const { test, expect } = require("@playwright/test");
const { TestHelpers } = require("./test-helpers");

const TEST_EMAIL = "admin@test-e2e.com";
const TEST_PASSWORD = "testpassword123";

test.describe.serial("Feed Home", () => {
	let helpers;

	test.beforeEach(async ({ page }) => {
		helpers = new TestHelpers(page);
		helpers.log("=== Starting Feed Test ===");

		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000
		});
		helpers.log("Login successful for feed test");
	});

	test.afterEach(async ({ page }) => {
		if (helpers) {
			await helpers.cleanup();
		}
	});

	test("admin home renders the activity feed: your sites and what's new", async ({ page }) => {
		helpers.log("Testing the activity-feed home page at /admin");

		// Ensure at least one site exists so the "Your sites" grid has content
		const domain = await helpers.createTestWebsite(`feed-home-${Date.now()}.com`);
		helpers.log(`Created site for feed home: ${domain}`);

		// The feed home lives at /admin (the websites list moved to /admin/websites)
		await helpers.navigateTo("/admin", { timeout: 30000 });

		const currentUrl = page.url();
		expect(currentUrl).toContain("/admin");
		expect(currentUrl).not.toContain("/admin/websites");
		expect(currentUrl).not.toContain("/login");

		// "Your sites" section header
		await helpers.waitForElement('h2:has-text("Your sites")', { timeout: 10000 });
		helpers.log("'Your sites' section is visible");

		// The site we created should appear in the sites grid
		const pageContent = await page.textContent("body");
		expect(pageContent).toContain(domain);
		helpers.log("Created site appears on the feed home");

		// "What's new" activity area
		await helpers.waitForElement('h2:has-text("What\'s new")', { timeout: 10000 });
		helpers.log("'What's new' activity area is visible");
	});
});
