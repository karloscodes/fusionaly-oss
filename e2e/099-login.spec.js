// e2e/login.spec.js
const { test, expect } = require("@playwright/test");
const { TestHelpers } = require("./test-helpers");

const TEST_EMAIL = "admin@test-e2e.com";
const TEST_PASSWORD = "testpassword123";

test.describe("Login Flow", () => {
	let helpers;

	test.beforeEach(async ({ page }) => {
		helpers = new TestHelpers(page);
		helpers.log("=== Starting Login Flow Test ===");

		// Navigate to the login page with validation
		await helpers.navigateTo("/login", {
			waitForSelector: 'input[name="email"]'
		});
	});

	test.afterEach(async ({ page }) => {
		if (helpers) {
			await helpers.cleanup();
		}
	});

	test("should fail with invalid credentials", async ({ page }) => {
		helpers.log("Testing login failure with invalid credentials");

		try {
			// Navigate to login page
			await helpers.navigateTo("/login", {
				waitForSelector: 'input[name="email"]'
			});

			// Fill in invalid credentials
			await helpers.fillForm({
				email: "invalid@example.com",
				password: "invalidpassword"
			}, { waitAfterSubmit: false });

			// For failed login with Inertia.js, wait for the error alert to appear
			// The server redirects to /login with a flash message, Inertia re-renders
			const errorAlert = page.locator('[role="alert"]');
			await expect(errorAlert).toBeVisible({ timeout: 15000 });
			helpers.log("Error alert is now visible");

			// Verify we're still on login page
			await expect(page).toHaveURL(/\/login/);
			helpers.log("Correctly stayed on login page after invalid login");

			// Verify error message content contains expected text
			const errorText = await errorAlert.textContent();
			expect(errorText).toContain("Invalid");
			helpers.log(`Found error message: ${errorText}`);

		} catch (error) {
			helpers.log(`Invalid credentials test failed: ${error.message}`, "error");
			await helpers.takeScreenshot("login-invalid-credentials-failed");
			throw error;
		}
	});

	test("should succeed with valid credentials", async ({ page }) => {
		helpers.log("Testing login success with valid credentials");

		try {
			// Attempt login with valid credentials
			const result = await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
				expectSuccess: true,
				timeout: 30000
			});

			// Verify successful login
			expect(result.success).toBe(true);
			helpers.log("✅ Login successful");

			// Verify we're redirected away from login page
			await expect(page).not.toHaveURL(/\/login$/);
			helpers.log(`✅ Correctly redirected to: ${page.url()}`);

			// Check for success indicators
			const currentUrl = page.url();
			const expectedPaths = ["/admin/websites", "/admin"];
			const isOnExpectedPath = expectedPaths.some(path => currentUrl.includes(path));

			expect(isOnExpectedPath).toBe(true);
			helpers.log("✅ Redirected to expected admin area");

		} catch (error) {
			helpers.log(`Valid credentials test failed: ${error.message}`, "error");
			await helpers.takeScreenshot("login-valid-credentials-failed");
			throw error;
		}
	});

	test("should handle complete logout flow", async ({ page }) => {
		helpers.log("Testing complete login-logout flow");

		try {
			// First, login successfully
			await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
				expectSuccess: true,
				timeout: 30000
			});

			helpers.log("✅ Login phase completed");

			// Test logout functionality - look for logout link with id="logout"
			const logoutSelectors = [
				"a#logout",
				"a[id=\"logout\"]",
				"text=\"Logout\""
			];

			let logoutElement = null;
			let foundSelector = null;

			for (const selector of logoutSelectors) {
				try {
					await helpers.waitForElement(selector, { timeout: 5000, silent: true });
					logoutElement = page.locator(selector);
					foundSelector = selector;
					helpers.log(`Found logout element: ${selector}`);
					break;
				} catch (error) {
					// Try next selector
				}
			}

			if (logoutElement && foundSelector) {
				// Click logout (uses Inertia router.post, so wait for URL change)
				await logoutElement.click();
				await page.waitForURL(/\/login/, { timeout: 10000 });

				// Verify we're back on login page
				await expect(page).toHaveURL(/\/login/);
				helpers.log("✅ Successfully logged out and redirected to login page");
			} else {
				throw new Error("No logout button found - unable to complete logout test");
			}

		} catch (error) {
			helpers.log(`❌ Logout flow test failed: ${error.message}`, "error");
			await helpers.takeScreenshot("login-logout-flow-failed");
			throw error;
		}
	});

	test("should handle form validation errors", async ({ page }) => {
		helpers.log("Testing form validation with empty fields");

		try {
			// Navigate to login page
			await helpers.navigateTo("/login");

			// Try to submit empty form
			await page.click('button[type="submit"]');
			await page.waitForLoadState("networkidle");

			// Should stay on login page
			await expect(page).toHaveURL(/\/login/);
			helpers.log("✅ Stayed on login page with empty form");

			// Test with just email
			await helpers.fillForm({ email: "test@example.com" }, {
				submitButton: null // Don't submit yet
			});

			await page.click('button[type="submit"]');
			await page.waitForLoadState("networkidle");

			// Should still stay on login page
			await expect(page).toHaveURL(/\/login/);
			helpers.log("✅ Stayed on login page with incomplete form");

		} catch (error) {
			helpers.log(`❌ Form validation test failed: ${error.message}`, "error");
			await helpers.takeScreenshot("login-form-validation-failed");
			throw error;
		}
	});
});
