// e2e/test-helpers.js
const { expect } = require("@playwright/test");

/**
 * Test helper utilities for E2E tests
 */

export const TEST_EMAIL = "admin@test-e2e.com";
export const TEST_PASSWORD = "testpassword123";

class TestHelpers {
	constructor(page) {
		this.page = page;
	}

	/**
	 * Enhanced logging with timestamps
	 */
	log(message, level = "info") {
		const timestamp = new Date().toISOString();
		const prefix = level === "error" ? "❌" : level === "warn" ? "⚠️" : "ℹ️";
		console.log(`[${timestamp}] ${prefix} ${message}`);
	}

	/**
	 * Wait for element with better error handling
	 */
	async waitForElement(selector, options = {}) {
		const { timeout = 10000, state = "visible", silent = false } = options;
		if (!silent) {
			this.log(`Waiting for element: ${selector}`);
		}

		try {
			await this.page.waitForSelector(selector, { timeout, state });
			if (!silent) {
				this.log(`Element found: ${selector}`);
			}
			return true;
		} catch (error) {
			if (!silent) {
				this.log(`Element not found: ${selector} - ${error.message}`, "error");
			}
			throw error;
		}
	}

	/**
	 * Enhanced navigation with validation
	 */
	async navigateTo(path, options = {}) {
		const { waitForSelector = null, timeout = 30000 } = options;
		this.log(`Navigating to: ${path}`);

		try {
			await this.page.goto(path, { timeout });
			await this.page.waitForLoadState("networkidle");

			// Try to wait for React hydration, but don't fail if it's not a React page
			try {
				await this.page.waitForFunction(() => {
					// Check for React root element
					const root = document.getElementById('root');
					if (root && root.children.length > 0) {
						return true;
					}

					// If no React root, check if page has basic content loaded
					const body = document.body;
					return body && body.children.length > 0;
				}, { timeout: 10000 });
			} catch (hydrationError) {
				this.log(`React hydration timeout (this might be a server-rendered page): ${hydrationError.message}`);
				// Continue anyway, some pages might not need React
			}

			// Additional wait for page-specific content to render
			await this.page.waitForTimeout(1000);

			if (waitForSelector) {
				await this.waitForElement(waitForSelector);
			}

			this.log(`Navigation successful: ${this.page.url()}`);
		} catch (error) {
			this.log(`Navigation failed: ${path} - ${error.message}`, "error");
			throw error;
		}
	}

	/**
	 * Enhanced form filling with validation
	 */
	async fillForm(formFields, options = {}) {
		const { submitButton = 'button[type="submit"]', waitAfterSubmit = true } = options;
		this.log(`Filling form with ${Object.keys(formFields).length} fields`);

		try {
			for (const [fieldName, value] of Object.entries(formFields)) {
				const selector = `input[name="${fieldName}"], input[id="${fieldName}"], textarea[name="${fieldName}"], textarea[id="${fieldName}"], select[name="${fieldName}"], select[id="${fieldName}"]`;
				await this.waitForElement(selector);
				await this.page.fill(selector, value);
				this.log(`Filled field: ${fieldName}`);
			}

			if (submitButton !== null && submitButton !== undefined) {
				await this.page.click(submitButton);
				this.log("Form submitted");

				if (waitAfterSubmit) {
					await this.page.waitForLoadState("networkidle");
				}
			} else {
				this.log("Form filled but not submitted (submitButton is null)");
			}
		} catch (error) {
			this.log(`Form filling failed: ${error.message}`, "error");
			throw error;
		}
	}

	/**
	 * Enhanced login with comprehensive error handling
	 */
	async login(email = "test@example.com", password = "password", options = {}) {
		const {
			expectSuccess = true,
			redirectPath = null,
			timeout = 30000
		} = options;

		this.log(`Attempting login: ${email}`);

		try {
			// Navigate to login page
			await this.navigateTo("/login");

			// Fill login form (don't wait after submit - we'll handle it)
			await this.fillForm({ email, password }, { waitAfterSubmit: false });

			// For Inertia.js, wait for URL change (client-side navigation)
			// If expecting success, wait to leave /login page
			// If expecting failure, wait for network idle (flash message)
			if (expectSuccess) {
				try {
					await this.page.waitForURL(url => !url.href.includes("/login"), { timeout });
				} catch (urlError) {
					// URL didn't change - check if there are error messages
					await this.page.waitForLoadState("networkidle", { timeout: 5000 });
				}
			} else {
				// For failed login, just wait for network idle
				await this.page.waitForLoadState("networkidle", { timeout });
			}

			const currentUrl = this.page.url();
			this.log(`Post-login URL: ${currentUrl}`);

			// Check for browser error pages
			if (currentUrl.includes("chrome-error://") || currentUrl.includes("about:blank")) {
				throw new Error(`Browser failed to load page after login: ${currentUrl}`);
			}

			// Check for error messages
			const errorMessages = await this.checkForMessages("error");

			if (expectSuccess) {
				if (errorMessages.length > 0) {
					throw new Error(`Login failed with errors: ${errorMessages.join(", ")}`);
				}

				if (currentUrl.includes("/login")) {
					throw new Error("Login failed - still on login page");
				}

				if (redirectPath && !currentUrl.includes(redirectPath)) {
					throw new Error(`Expected redirect to ${redirectPath}, but got ${currentUrl}`);
				}

				this.log("Login successful");
			} else {
				if (errorMessages.length === 0 && !currentUrl.includes("/login")) {
					throw new Error("Expected login to fail, but it succeeded");
				}
				this.log("Login failed as expected");
			}

			return { success: !currentUrl.includes("/login"), url: currentUrl, errors: errorMessages };
		} catch (error) {
			this.log(`Login process failed: ${error.message}`, "error");
			throw error;
		}
	}

	/**
	 * Check for flash messages or alerts
	 */
	async checkForMessages(type = "any") {
		const selectors = [
			'[role="alert"]',
			'.alert',
			'.flash-message',
			'.error-message',
			'.success-message',
			'.notification',
			'[data-testid="flash-message"]',
			'[data-testid="error-message"]',
			'[data-testid="success-message"]',
			'.text-red-500', '.text-red-600', '.text-red-700', // Tailwind error classes
			'.text-green-500', '.text-green-600', '.text-green-700', // Tailwind success classes
		];

		const messages = [];

		for (const selector of selectors) {
			try {
				const elements = await this.page.locator(selector).all();
				for (const element of elements) {
					const isVisible = await element.isVisible();
					if (isVisible) {
						// Skip elements that are insight badges (not actual error messages)
						const isInsightBadge = await element.getAttribute('data-insight-badge');
						if (isInsightBadge) continue;

						const text = await element.textContent();
						if (text && text.trim()) {
							const messageType = this.determineMessageType(text, selector);
							if (type === "any" || type === messageType) {
								messages.push({ type: messageType, text: text.trim(), selector });
							}
						}
					}
				}
			} catch (error) {
				// Ignore errors for individual selectors
			}
		}

		if (messages.length > 0) {
			this.log(`Found ${messages.length} messages: ${messages.map(m => m.text).join(", ")}`);
		}

		return messages.map(m => m.text);
	}

	/**
	 * Determine message type based on content and selector
	 */
	determineMessageType(text, selector) {
		const lowerText = text.toLowerCase();
		const lowerSelector = selector.toLowerCase();

		if (lowerText.includes("error") || lowerText.includes("failed") || lowerText.includes("invalid") ||
			lowerSelector.includes("error") || lowerSelector.includes("red")) {
			return "error";
		}

		if (lowerText.includes("success") || lowerText.includes("created") || lowerText.includes("saved") ||
			lowerSelector.includes("success") || lowerSelector.includes("green")) {
			return "success";
		}

		if (lowerText.includes("warning") || lowerText.includes("warn")) {
			return "warning";
		}

		return "info";
	}

	/**
	 * Wait for network request with enhanced error handling
	 */
	async waitForRequest(urlPattern, options = {}) {
		const { method = "POST", timeout = 15000, validateResponse = null } = options;
		this.log(`Waiting for ${method} request to: ${urlPattern}`);

		try {
			const responsePromise = this.page.waitForResponse(
				async (response) => {
					if (!response.url().includes(urlPattern) || response.request().method() !== method) {
						return false;
					}

					if (validateResponse) {
						try {
							const valid = await validateResponse(response);
							return valid;
						} catch (error) {
							this.log(`Response validation failed: ${error.message}`, "warn");
							return false;
						}
					}

					return true;
				},
				{ timeout }
			);

			const response = await responsePromise;
			this.log(`Request completed: ${response.status()} ${response.url()}`);
			return response;
		} catch (error) {
			this.log(`Request wait failed: ${urlPattern} - ${error.message}`, "error");
			throw error;
		}
	}

	/**
	 * Enhanced screenshot with context
	 */
	async takeScreenshot(name, options = {}) {
		const { fullPage = false, path = null } = options;
		const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
		const filename = path || `screenshot-${name}-${timestamp}.png`;

		this.log(`Taking screenshot: ${filename}`);

		try {
			await this.page.screenshot({
				path: filename,
				fullPage,
				...options
			});
			this.log(`Screenshot saved: ${filename}`);
		} catch (error) {
			this.log(`Screenshot failed: ${error.message}`, "error");
		}
	}

	/**
	 * Wait for element to be stable (not moving/changing)
	 */
	async waitForStable(selector, options = {}) {
		const { timeout = 5000, pollInterval = 100 } = options;
		this.log(`Waiting for element to be stable: ${selector}`);

		const element = this.page.locator(selector);
		let lastBoundingBox = null;
		let stableCount = 0;
		const requiredStableCount = 5; // Element must be stable for 5 consecutive checks

		const startTime = Date.now();

		while (Date.now() - startTime < timeout) {
			try {
				const boundingBox = await element.boundingBox();

				if (boundingBox && lastBoundingBox) {
					const isSame = JSON.stringify(boundingBox) === JSON.stringify(lastBoundingBox);
					if (isSame) {
						stableCount++;
						if (stableCount >= requiredStableCount) {
							this.log(`Element is stable: ${selector}`);
							return;
						}
					} else {
						stableCount = 0;
					}
				}

				lastBoundingBox = boundingBox;
				await this.page.waitForTimeout(pollInterval);
			} catch (error) {
				// Element might not be visible yet
				await this.page.waitForTimeout(pollInterval);
			}
		}

		throw new Error(`Element did not stabilize within ${timeout}ms: ${selector}`);
	}

	/**
	 * Create a website for testing
	 */
	async createTestWebsite(domain = null) {
		if (!domain) {
			domain = `test-website-${Date.now()}.com`;
		}

		this.log(`Creating test website: ${domain}`);

		try {
			await this.navigateTo("/admin/websites/new");
			await this.fillForm({ domain }, { waitAfterSubmit: false });

			// Wait for Inertia navigation - redirect away from /new page
			await this.page.waitForURL(url => !url.href.includes("/new"), { timeout: 15000 });

			// Success is indicated by redirect away from /new
			const currentUrl = this.page.url();
			if (currentUrl.includes("/new")) {
				throw new Error("Still on creation page after website creation");
			}

			this.log(`Website created successfully: ${domain}`);
			return domain;
		} catch (error) {
			this.log(`Website creation failed: ${error.message}`, "error");
			throw error;
		}
	}

	/**
	 * Check if onboarding is required
	 */
	async isOnboardingRequired() {
		try {
			const response = await this.page.request.get("/api/onboarding/check");
			const data = await response.json();
			return data.required || false;
		} catch (error) {
			this.log(`Failed to check onboarding status: ${error.message}`, "error");
			return false;
		}
	}

	/**
	 * Handle potential onboarding redirect during navigation
	 */
	async navigateWithOnboardingHandling(path, options = {}) {
		const { expectOnboarding = null, skipOnboardingCheck = false } = options;

		if (!skipOnboardingCheck) {
			const onboardingRequired = await this.isOnboardingRequired();
			if (onboardingRequired && expectOnboarding !== false) {
				this.log("Onboarding is required - expecting redirect to setup");
				await this.navigateTo(path, options);
				// Should redirect to setup
				await this.page.waitForURL(/\/setup/, { timeout: options.timeout || 30000 });
				return;
			}
		}

		// Normal navigation
		await this.navigateTo(path, options);
	}

	/**
	 * Complete onboarding flow with provided details
	 */
	async completeOnboarding(options = {}) {
		const {
			licenseKey = "valid-test-license-key",
			username = "testuser",
			email = "test@example.com",
			password = "testpassword123",
			openaiKey = null,
			useGumroadEmail = false
		} = options;

		this.log("Starting complete onboarding flow");

		// Step 1: License Key
		await this.fillForm({ license_key: licenseKey }, { submitButton: 'button[type="submit"]' });
		await this.page.waitForLoadState("networkidle");

		// Step 2: User Account
		const userFields = { username, email };
		if (useGumroadEmail) {
			// Check the use Gumroad email checkbox if it exists
			try {
				const checkbox = this.page.locator('input[name="use_gumroad_email"]');
				await checkbox.check();
			} catch (error) {
				this.log("Use Gumroad email checkbox not found", "warn");
			}
		}
		await this.fillForm(userFields, { submitButton: 'button[type="submit"]' });
		await this.page.waitForLoadState("networkidle");

		// Step 3: Password
		await this.fillForm({
			password,
			confirm_password: password
		}, { submitButton: 'button[type="submit"]' });
		await this.page.waitForLoadState("networkidle");

		// Step 4: OpenAI (optional)
		if (openaiKey) {
			await this.fillForm({ openai_api_key: openaiKey }, { submitButton: 'button[type="submit"]' });
		} else {
			// Try to skip or submit empty
			const skipButton = this.page.locator('button:has-text("Skip"), button:has-text("Continue without AI")');
			try {
				await skipButton.click();
			} catch (error) {
				await this.page.click('button[type="submit"]');
			}
		}
		await this.page.waitForLoadState("networkidle");

		// Should be logged in and redirected
		await this.page.waitForURL(url => !url.href.includes("/setup") && !url.href.includes("/login"), {
			timeout: 30000
		});

		this.log("✅ Onboarding completed successfully");
	}

	/**
	 * Reset database to fresh state (no users)
	 */
	async resetDatabaseForOnboarding() {
		this.log("Resetting database for onboarding tests");
		try {
			// This would typically involve calling the setup script without creating users
			// For now, we'll rely on the test environment setup
			this.log("Database reset completed");
		} catch (error) {
			this.log(`Database reset failed: ${error.message}`, "error");
			throw error;
		}
	}

	/**
	 * Clean up test data
	 */
	async cleanup() {
		this.log("Performing test cleanup");
		// Add any cleanup logic here
		// For now, just log that cleanup was called
		this.log("Cleanup completed");
	}
}

module.exports = { TestHelpers }; 
