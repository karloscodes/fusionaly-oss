const { test, expect } = require("@playwright/test");
const { TestHelpers } = require("./test-helpers");

const TEST_EMAIL = "admin@test-e2e.com";
const TEST_PASSWORD = "testpassword123";

test.describe("Conversion Goals", () => {
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

	test("saves a goal for an event that has not fired yet", async ({ page }) => {
		const goalName = `cta:never_fired_${Date.now()}`;
		await helpers.navigateTo("/admin/websites");
		const href = await page
			.getByRole("link", { name: "localhost", exact: true })
			.getAttribute("href");
		const websiteId = href.match(/\/admin\/websites\/(\d+)/)[1];
		const editPath = `/admin/websites/${websiteId}/edit`;

		await helpers.navigateTo(editPath);
		const search = page.getByPlaceholder("Search or type an event name...");
		await search.fill(goalName);
		await search.press("Enter");
		await page.click('button[type="submit"]');

		await expect(page.getByRole("alert")).toContainText("Website updated successfully");
		await helpers.navigateTo(editPath);
		await expect(page.getByLabel(goalName)).toBeChecked();
	});
});
