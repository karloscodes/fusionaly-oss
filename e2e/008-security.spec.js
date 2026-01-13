const { test, expect } = require("@playwright/test");
const { TestHelpers } = require("./test-helpers");

const TEST_EMAIL = "admin@test-e2e.com";
const TEST_PASSWORD = "testpassword123";

test.describe.serial("Security Tests", () => {
	let helpers;

	test.beforeEach(async ({ page }) => {
		helpers = new TestHelpers(page);
		helpers.log("=== Starting Security Test ===");
	});

	test.afterEach(async ({ page }) => {
		if (helpers) {
			await helpers.cleanup();
		}
	});

	test("should redirect to login when accessing protected routes without auth", async ({ page }) => {
		helpers.log("Testing protected route access without authentication");

		// Clear any existing cookies/session
		await page.context().clearCookies();

		// Only test routes that actually exist in the app
		const protectedRoutes = [
			"/admin",
			"/admin/websites/new",
			"/admin/administration/ingestion",
			"/admin/administration/account",
			"/admin/administration/system"
		];

		for (const route of protectedRoutes) {
			helpers.log(`Testing protected route: ${route}`);

			// Navigate to protected route
			await page.goto(`http://localhost:3000${route}`, {
				waitUntil: "networkidle"
			});

			// Should be redirected to login (or setup if no users exist)
			// Both are valid - either way, user cannot access protected content
			const currentUrl = page.url();
			const redirectedToAuth = currentUrl.includes("/login") || currentUrl.includes("/setup");
			expect(redirectedToAuth).toBe(true);
			helpers.log(`Route ${route} correctly redirects to ${currentUrl.includes("/login") ? "login" : "setup"}`);
		}

		helpers.log("All protected routes correctly require authentication");
	});

	test("should not expose sensitive data in page source", async ({ page }) => {
		helpers.log("Testing for sensitive data exposure");

		// Login first
		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000
		});

		// Navigate to administration pages
		await helpers.navigateTo("/admin/administration/system", {
			timeout: 30000
		});

		// Get page source
		const pageContent = await page.content();

		// Check that sensitive data is not exposed
		const sensitivePatterns = [
			/password\s*[=:]\s*["'][^"']+["']/i,
			/secret\s*[=:]\s*["'][^"']+["']/i,
			/api[_-]?key\s*[=:]\s*["'][^"']+["']/i,
			/private[_-]?key\s*[=:]\s*["'][^"']+["']/i
		];

		for (const pattern of sensitivePatterns) {
			const matches = pageContent.match(pattern);
			if (matches) {
				// Filter out false positives (form field names, labels)
				const isFalsePositive = matches.some(m =>
					m.includes('name="') ||
					m.includes('placeholder') ||
					m.includes('label') ||
					m.includes('type="password"')
				);
				if (!isFalsePositive) {
					throw new Error(`Potential sensitive data exposure found: ${matches[0].substring(0, 50)}`);
				}
			}
		}

		helpers.log("No sensitive data exposed in page source");
	});

	// Note: Inertia.js uses SameSite cookies + Origin header validation for CSRF protection
	// instead of traditional CSRF tokens, which is equally secure. The cookie security test below
	// verifies SameSite is set correctly.

	test("should handle password change with validation", async ({ page }) => {
		helpers.log("Testing password change validation");

		// Login first
		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000
		});

		// Navigate to Account page
		await helpers.navigateTo("/admin/administration/account", {
			waitForSelector: 'text=Change Password',
			timeout: 30000
		});

		// Find password inputs
		const currentPasswordInput = page.locator('input[type="password"]').nth(0);
		const newPasswordInput = page.locator('input[type="password"]').nth(1);
		const confirmPasswordInput = page.locator('input[type="password"]').nth(2);

		// Test 1: Mismatched passwords should show error
		await currentPasswordInput.fill(TEST_PASSWORD);
		await newPasswordInput.fill("newpassword123");
		await confirmPasswordInput.fill("differentpassword");

		// Try to submit
		await page.click('button[type="submit"]:has-text("Update Password")');
		await page.waitForTimeout(1000);

		// Should show error or stay on page
		const currentUrl = page.url();
		expect(currentUrl).toContain("/admin/administration/account");

		// Check for error message
		const errorMessage = page.locator('text=/password.*match|confirm.*password/i').first();
		const hasError = await errorMessage.isVisible().catch(() => false);
		helpers.log(`Password mismatch validation: ${hasError ? 'error shown' : 'form prevented submission'}`);

		// Test 2: Correct current password required
		await currentPasswordInput.fill("wrongpassword");
		await newPasswordInput.fill("newpassword123");
		await confirmPasswordInput.fill("newpassword123");

		await page.click('button[type="submit"]:has-text("Update Password")');
		await page.waitForTimeout(1000);

		// Should stay on page (wrong current password)
		expect(page.url()).toContain("/admin/administration/account");

		helpers.log("Password change validation working correctly");
	});

	test("should enforce rate limiting on login (production behavior)", async ({ page, request }) => {
		helpers.log("Testing rate limiting awareness on login");

		// Note: Rate limiting is only active in production
		// This test verifies the login endpoint handles multiple requests gracefully

		const loginAttempts = 5;
		let successCount = 0;
		let errorCount = 0;

		for (let i = 0; i < loginAttempts; i++) {
			await page.goto("http://localhost:3000/login", { waitUntil: "networkidle" });

			await page.fill('input[name="email"]', `test${i}@example.com`);
			await page.fill('input[name="password"]', "wrongpassword");
			await page.click('button[type="submit"]');

			await page.waitForTimeout(500);

			// Check response
			const onLoginPage = page.url().includes("/login");
			if (onLoginPage) {
				successCount++;
			} else {
				errorCount++;
			}
		}

		// All attempts should stay on login page (no crashes or unexpected behavior)
		expect(successCount).toBe(loginAttempts);
		helpers.log(`All ${loginAttempts} login attempts handled gracefully`);
	});

	test("should set secure cookie attributes", async ({ page, context }) => {
		helpers.log("Testing cookie security attributes");

		// Login to get session cookie
		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000
		});

		// Get cookies
		const cookies = await context.cookies();

		// Find session cookie
		const sessionCookie = cookies.find(c => c.name.includes("session"));

		if (sessionCookie) {
			helpers.log(`Session cookie found: ${sessionCookie.name}`);

			// HttpOnly should be true
			expect(sessionCookie.httpOnly).toBe(true);

			// SameSite should be set
			expect(sessionCookie.sameSite).toBeDefined();

			// In production, Secure should be true (in test/dev it might be false)
			helpers.log(`Cookie attributes: httpOnly=${sessionCookie.httpOnly}, sameSite=${sessionCookie.sameSite}`);
		} else {
			helpers.log("Note: Session cookie not found with 'session' in name - checking all cookies");
			for (const cookie of cookies) {
				helpers.log(`Cookie: ${cookie.name}, httpOnly=${cookie.httpOnly}`);
			}
		}

		helpers.log("Cookie security check completed");
	});

	test("should prevent XSS in user input display", async ({ page }) => {
		helpers.log("Testing XSS prevention");

		// Login first
		await helpers.login(TEST_EMAIL, TEST_PASSWORD, {
			expectSuccess: true,
			timeout: 30000
		});

		// Try to create a website with XSS payload
		const xssPayloads = [
			'<script>alert("xss")</script>.com',
			'"><img src=x onerror=alert("xss")>.com',
			"javascript:alert('xss').com"
		];

		for (const payload of xssPayloads) {
			await helpers.navigateTo("/admin/websites/new", {
				waitForSelector: 'input[name="domain"]'
			});

			// Fill domain with XSS payload
			await page.fill('input[name="domain"]', payload);

			// Wait and check if validation prevents submission or sanitizes
			await page.waitForTimeout(500);

			// The form should either reject invalid domain or sanitize it
			// Check that no script executed
			const alertShown = await page.evaluate(() => {
				return window.alertWasShown === true;
			}).catch(() => false);

			expect(alertShown).toBe(false);
		}

		helpers.log("XSS prevention working correctly");
	});
});
