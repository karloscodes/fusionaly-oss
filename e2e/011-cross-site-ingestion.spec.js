const { test, expect } = require("@playwright/test");
const { TestHelpers } = require("./test-helpers");

const TEST_EMAIL = "admin@test-e2e.com";
const TEST_PASSWORD = "testpassword123";
const FUSIONALY = "http://localhost:3000";

// The real deployment shape: a customer site on its own domain loads the SDK
// from Fusionaly on a different domain, so every event is a cross-site request.
// The customer page is served by page.route on a ".test" host, so no extra
// server or DNS is needed; the SDK and its events hit the real app.
//
// Chromium lets a public page load from localhost only from a secure context
// and with the Local Network Access permission. A customer site is https, so
// the page is https; the visitor grants the permission, which no real visitor
// is asked for, because a real Fusionaly is not on localhost.
test.describe("Cross-site Ingestion", () => {
	let helpers;

	test.beforeEach(async ({ page }) => {
		helpers = new TestHelpers(page);
		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000,
		});
	});

	test.afterEach(async () => {
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

	// visitCustomerSite opens a fresh browser (like a real visitor, without the
	// suite's extra test headers) on https://<domain>/ and returns the response
	// Fusionaly gives to the first event.
	async function visitCustomerSite(browser, domain) {
		const visitor = await browser.newContext({ permissions: ["local-network-access"] });
		const page = await visitor.newPage();
		await page.route(`https://${domain}/**`, (route) =>
			route.fulfill({
				contentType: "text/html",
				body: `<!doctype html>
<html>
  <head><title>Customer site</title></head>
  <body>
    <h1>Welcome to ${domain}</h1>
    <script defer src="${FUSIONALY}/y/api/v1/sdk.js"></script>
  </body>
</html>`,
			}),
		);

		const eventResponse = page.waitForResponse(
			(response) =>
				response.url().startsWith(`${FUSIONALY}/x/api/v1/events`) &&
				response.request().method() === "POST",
			{ timeout: 15000 },
		);
		await page.goto(`https://${domain}/`);
		const response = await eventResponse;
		const headers = await response.request().allHeaders();

		await visitor.close();
		return { response, headers };
	}

	test("accepts events from a registered website on another domain", async ({ browser }) => {
		const domain = `shop-${Date.now()}.test`;
		await createWebsite(domain);

		const { response, headers } = await visitCustomerSite(browser, domain);

		// Playwright can't see the browser-added Sec-Fetch-Site header, but the
		// Origin proves the request left another site, and ingestion rejects
		// requests without Sec-Fetch-Site, so a 202 means the browser sent it.
		expect(headers["origin"], "the event must come from the customer's domain").toBe(`https://${domain}`);
		expect(response.status(), "a cross-site page view from a registered website should be accepted").toBe(202);
	});

	test("rejects events from a website that is not registered", async ({ browser }) => {
		const domain = `unknown-${Date.now()}.test`;

		const { response, headers } = await visitCustomerSite(browser, domain);

		expect(headers["origin"]).toBe(`https://${domain}`);
		expect(response.status(), "events from an unregistered domain should be rejected").toBe(403);
	});
});
