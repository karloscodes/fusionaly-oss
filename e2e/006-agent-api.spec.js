const { test, expect } = require("@playwright/test");
const { TestHelpers } = require("./test-helpers");

const TEST_EMAIL = "admin@test-e2e.com";
const TEST_PASSWORD = "testpassword123";

test.describe.serial("Agent API Tests", () => {
	let helpers;
	let apiKey = null;

	test.beforeEach(async ({ page }) => {
		helpers = new TestHelpers(page);
		helpers.log("=== Starting Agent API Test ===");

		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000
		});
		helpers.log("Login successful for agent API test");
	});

	test.afterEach(async ({ page }) => {
		if (helpers) {
			await helpers.cleanup();
		}
	});

	test("should access Agents administration page", async ({ page }) => {
		helpers.log("Testing Agents administration page");

		await helpers.navigateTo("/admin/administration/agents", {
			timeout: 30000
		});

		const currentUrl = helpers.page.url();
		expect(currentUrl).toContain("/admin/administration/agents");

		await helpers.waitForElement("h1", { timeout: 10000 });
		const heading = await helpers.page.textContent("h1");
		expect(heading).toContain("Agent API");

		const pageContent = await page.textContent("body");
		expect(pageContent).toContain("API Key");
		expect(pageContent).toContain("Setup instructions");

		helpers.log("Agents administration page accessible");
	});

	test("should generate API key", async ({ page }) => {
		helpers.log("Testing API key generation");

		await helpers.navigateTo("/admin/administration/agents", {
			timeout: 30000
		});

		// Check if key already exists or needs to be generated
		const pageContent = await page.textContent("body");

		if (pageContent.includes("Generate API Key")) {
			// Click generate button
			const generateButton = page.locator('button:has-text("Generate API Key")');
			await generateButton.click();
			await page.waitForLoadState("networkidle", { timeout: 10000 });
			helpers.log("Generated new API key");
		}

		// Now reveal the key
		const revealButton = page.locator('button:has-text("Reveal")');
		if (await revealButton.count() > 0) {
			await revealButton.click();
			await page.waitForLoadState("networkidle", { timeout: 10000 });
			helpers.log("Revealed API key");
		}

		// Get the API key from the input
		const keyInput = page.locator('input[readonly]').first();
		apiKey = await keyInput.inputValue();

		expect(apiKey).toBeTruthy();
		expect(apiKey.length).toBeGreaterThan(20);
		helpers.log(`API key retrieved: ${apiKey.substring(0, 8)}...`);
	});

	test("should access schema endpoint with valid API key", async ({ page, request }) => {
		helpers.log("Testing schema endpoint");

		// First get the API key if we don't have it
		if (!apiKey) {
			await helpers.navigateTo("/admin/administration/agents", { timeout: 30000 });

			const revealButton = page.locator('button:has-text("Reveal")');
			if (await revealButton.count() > 0) {
				await revealButton.click();
				await page.waitForLoadState("networkidle", { timeout: 10000 });
			}

			const keyInput = page.locator('input[readonly]').first();
			apiKey = await keyInput.inputValue();
		}

		expect(apiKey).toBeTruthy();

		// Call the schema endpoint
		const response = await request.get("/z/api/v1/schema", {
			headers: {
				"Authorization": `Bearer ${apiKey}`
			}
		});

		expect(response.status()).toBe(200);

		const data = await response.json();
		expect(data).toHaveProperty("schema");
		expect(data).toHaveProperty("concepts");
		expect(data.schema).toContain("CREATE TABLE");

		helpers.log("Schema endpoint returned valid response");
	});

	test("should reject schema endpoint without API key", async ({ request }) => {
		helpers.log("Testing schema endpoint without auth");

		const response = await request.get("/z/api/v1/schema");

		expect(response.status()).toBe(401);
		helpers.log("Schema endpoint correctly rejected unauthenticated request");
	});

	test("should execute valid SQL query", async ({ page, request }) => {
		helpers.log("Testing SQL endpoint");

		if (!apiKey) {
			await helpers.navigateTo("/admin/administration/agents", { timeout: 30000 });

			const revealButton = page.locator('button:has-text("Reveal")');
			if (await revealButton.count() > 0) {
				await revealButton.click();
				await page.waitForLoadState("networkidle", { timeout: 10000 });
			}

			const keyInput = page.locator('input[readonly]').first();
			apiKey = await keyInput.inputValue();
		}

		expect(apiKey).toBeTruthy();

		// Execute a simple SELECT query
		const response = await request.post("/z/api/v1/sql", {
			headers: {
				"Authorization": `Bearer ${apiKey}`,
				"Content-Type": "application/json"
			},
			data: {
				sql: "SELECT 1 as test",
				website_id: 1
			}
		});

		expect(response.status()).toBe(200);

		const data = await response.json();
		expect(data).toHaveProperty("columns");
		expect(data).toHaveProperty("rows");
		expect(data.columns).toContain("test");

		helpers.log("SQL endpoint executed query successfully");
	});

	test("should reject dangerous SQL queries", async ({ page, request }) => {
		helpers.log("Testing SQL injection protection");

		if (!apiKey) {
			await helpers.navigateTo("/admin/administration/agents", { timeout: 30000 });

			const revealButton = page.locator('button:has-text("Reveal")');
			if (await revealButton.count() > 0) {
				await revealButton.click();
				await page.waitForLoadState("networkidle", { timeout: 10000 });
			}

			const keyInput = page.locator('input[readonly]').first();
			apiKey = await keyInput.inputValue();
		}

		const dangerousQueries = [
			"DELETE FROM site_stats",
			"DROP TABLE users",
			"INSERT INTO settings VALUES (1, 'evil', 'data')",
			"SELECT * FROM site_stats; DELETE FROM users;",
			"SELECT * FROM site_stats /* comment */",
		];

		for (const sql of dangerousQueries) {
			const response = await request.post("/z/api/v1/sql", {
				headers: {
					"Authorization": `Bearer ${apiKey}`,
					"Content-Type": "application/json"
				},
				data: {
					sql: sql,
					website_id: 1
				}
			});

			expect(response.status()).toBe(400);
			helpers.log(`Correctly rejected: ${sql.substring(0, 30)}...`);
		}

		helpers.log("SQL injection protection working correctly");
	});

	test("should allow WITH (CTE) queries", async ({ page, request }) => {
		helpers.log("Testing WITH (CTE) queries");

		if (!apiKey) {
			await helpers.navigateTo("/admin/administration/agents", { timeout: 30000 });

			const revealButton = page.locator('button:has-text("Reveal")');
			if (await revealButton.count() > 0) {
				await revealButton.click();
				await page.waitForLoadState("networkidle", { timeout: 10000 });
			}

			const keyInput = page.locator('input[readonly]').first();
			apiKey = await keyInput.inputValue();
		}

		const response = await request.post("/z/api/v1/sql", {
			headers: {
				"Authorization": `Bearer ${apiKey}`,
				"Content-Type": "application/json"
			},
			data: {
				sql: "WITH test AS (SELECT 1 as val) SELECT * FROM test",
				website_id: 1
			}
		});

		expect(response.status()).toBe(200);

		const data = await response.json();
		expect(data.columns).toContain("val");

		helpers.log("WITH (CTE) queries work correctly");
	});

	test("should regenerate API key", async ({ page }) => {
		helpers.log("Testing API key regeneration");

		await helpers.navigateTo("/admin/administration/agents", {
			timeout: 30000
		});

		// Store old key
		const revealButton = page.locator('button:has-text("Reveal")');
		if (await revealButton.count() > 0) {
			await revealButton.click();
			await page.waitForLoadState("networkidle", { timeout: 10000 });
		}

		const keyInput = page.locator('input[readonly]').first();
		const oldKey = await keyInput.inputValue();

		// Click regenerate and accept confirm dialog
		page.on('dialog', dialog => dialog.accept());

		const regenerateButton = page.locator('button:has-text("Regenerate")');
		await regenerateButton.click();
		await page.waitForLoadState("networkidle", { timeout: 10000 });

		// Check for success message
		const successMessages = await helpers.checkForMessages("success");
		expect(successMessages.length).toBeGreaterThan(0);

		helpers.log("API key regenerated successfully");
	});
});
