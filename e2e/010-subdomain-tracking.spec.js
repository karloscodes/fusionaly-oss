const { test, expect } = require("@playwright/test");
const { TestHelpers } = require("./test-helpers");

const TEST_EMAIL = "admin@test-e2e.com";
const TEST_PASSWORD = "testpassword123";

// These tests exercise real Origin-header validation end-to-end by pointing the
// browser at actual "*.localhost" hostnames (which every OS/browser resolves to
// 127.0.0.1 per RFC 6761) instead of plain "localhost" - visiting plain
// "localhost" would take the dev-mode bypass in validateOrigin() and never
// touch the registered-domain lookup this is meant to cover.
test.describe("Subdomain Origin Validation", () => {
	let helpers;

	test.beforeEach(async ({ page }) => {
		helpers = new TestHelpers(page);
		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000,
		});
	});

	test.afterEach(async ({ page }) => {
		if (helpers) {
			await helpers.cleanup();
		}
	});

	async function createWebsite(domainName) {
		await helpers.navigateTo("/admin/websites/new", {
			waitForSelector: 'input[name="domain"]',
		});
		await helpers.fillForm({ domain: domainName }, { waitAfterSubmit: false });
		await helpers.page.waitForURL((url) => !url.href.includes("/new"), { timeout: 15000 });
	}

	function waitForEventResponse(page) {
		return page.waitForResponse(
			(response) =>
				response.url().includes("/x/api/v1/events") &&
				response.request().method() === "POST",
			{ timeout: 15000 },
		);
	}

	test("accepts tracking traffic from a website registered directly as a subdomain", async ({
		context,
	}) => {
		const domain = `sub-${Date.now()}.localhost`;
		await createWebsite(domain);

		const demoPage = await context.newPage();
		const responsePromise = waitForEventResponse(demoPage);

		await demoPage.goto(`http://${domain}:3000/_demo`);
		const response = await responsePromise;

		expect(
			response.status(),
			"page view from a website registered as a subdomain should be accepted",
		).toBe(202);

		await demoPage.close();
	});

	test("respects the subdomain-tracking toggle for a base-domain website", async ({
		page,
		context,
	}) => {
		// "localhost" is pre-registered for the whole e2e suite (see setup-test-env.js)
		// with subdomain tracking left at its default (off).
		const subHost = `probe-${Date.now()}.localhost`;

		const rejectPage = await context.newPage();
		const rejectedPromise = waitForEventResponse(rejectPage);
		await rejectPage.goto(`http://${subHost}:3000/_demo`);
		const rejectedResponse = await rejectedPromise;
		expect(
			rejectedResponse.status(),
			"an unregistered subdomain should be rejected while subdomain tracking is off",
		).toBe(403);
		await rejectPage.close();

		// Enable subdomain tracking for "localhost" via the website edit page
		await helpers.navigateTo("/admin/websites");
		const localhostLink = page.getByRole("link", { name: "localhost", exact: true });
		const href = await localhostLink.getAttribute("href");
		const websiteId = href.match(/\/admin\/websites\/(\d+)/)[1];

		await helpers.navigateTo(`/admin/websites/${websiteId}/edit`);
		const toggleContainer = page.locator("div.border.rounded-lg.p-4", {
			hasText: "Track all subdomains under",
		});
		await toggleContainer.locator('input[type="checkbox"]').click({ force: true });
		await page.click('button[type="submit"]');
		await page.waitForLoadState("networkidle");

		// The exact same subdomain should now be accepted
		const acceptPage = await context.newPage();
		const acceptedPromise = waitForEventResponse(acceptPage);
		await acceptPage.goto(`http://${subHost}:3000/_demo`);
		const acceptedResponse = await acceptedPromise;
		expect(
			acceptedResponse.status(),
			"the same subdomain should be accepted once subdomain tracking is enabled",
		).toBe(202);
		await acceptPage.close();
	});
});
