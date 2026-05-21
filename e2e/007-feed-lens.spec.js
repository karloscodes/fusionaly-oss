// e2e/007-feed-lens.spec.js - Activity feed home + AI Lens empty state
// Relies on the user created by 001-onboarding. The OpenAI key is never
// configured in the test environment, so the Lens page renders its no-key
// empty state and no live OpenAI call is ever made.
const { test, expect } = require("@playwright/test");
const { TestHelpers } = require("./test-helpers");

const TEST_EMAIL = "admin@test-e2e.com";
const TEST_PASSWORD = "testpassword123";

test.describe.serial("Feed Home and AI Lens", () => {
	let helpers;

	test.beforeEach(async ({ page }) => {
		helpers = new TestHelpers(page);
		helpers.log("=== Starting Feed/Lens Test ===");

		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000
		});
		helpers.log("Login successful for feed/lens test");
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

	test("Lens page shows the add-your-OpenAI-key empty state when no key is configured", async ({ page }) => {
		helpers.log("Testing the Lens no-key empty state");

		// Create a site and grab its ID from the post-create URL
		const domain = `lens-empty-${Date.now()}.com`;
		await helpers.navigateTo("/admin/websites/new", {
			waitForSelector: 'input[name="domain"]'
		});
		await helpers.fillForm({ domain }, { waitAfterSubmit: false });
		await page.waitForURL(url => !url.href.includes("/new"), { timeout: 15000 });

		const setupUrl = page.url();
		const match = setupUrl.match(/\/admin\/websites\/(\d+)/);
		expect(match).toBeTruthy();
		const websiteId = match[1];
		helpers.log(`Created site ${domain} (ID: ${websiteId})`);

		// Open the Lens page for that site
		await helpers.navigateTo(`/admin/websites/${websiteId}/lens`, {
			waitForSelector: 'h1:has-text("Ask")',
			timeout: 30000
		});

		const currentUrl = page.url();
		expect(currentUrl).toContain(`/admin/websites/${websiteId}/lens`);

		// With no OpenAI key configured, the empty state prompts to add a key
		await helpers.waitForElement('text=Add your OpenAI key to get started', { timeout: 10000 });
		helpers.log("Lens empty-state title is visible");

		// And links to the AI settings page to add the key
		const settingsLink = page.locator('a[href="/admin/administration/ai"]');
		await expect(settingsLink.first()).toBeVisible();
		helpers.log("Lens empty state links to AI settings");
	});
});
