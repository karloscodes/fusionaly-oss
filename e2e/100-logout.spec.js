const { test, expect } = require("@playwright/test");

const TEST_EMAIL = "admin@test-e2e.com";
const TEST_PASSWORD = "testpassword123";


test.describe("Logout Flow", () => {
  test("should properly logout and redirect to login page", async ({
    page,
    context,
  }) => {
    console.log("Starting complete logout test flow");

    // First login with valid credentials
    await page.goto("/login");
    await page.locator('input[name="email"]').fill(TEST_EMAIL);
    await page.locator('input[name="password"]').fill(TEST_PASSWORD);
    console.log("Filled form with valid credentials");

    // Submit login form
    await page.locator('button[type="submit"]').click();
    console.log("Submitted login form");

    // Wait for response
    await page.waitForLoadState("networkidle");
    console.log("Current URL after login attempt:", page.url());

    // Login MUST succeed - this is not optional
    expect(page.url()).toContain("/admin");
    console.log("Login successful, testing logout");

    // Click logout and wait for redirect to login page
    await page.locator('a[id="logout"]').click({ timeout: 5000 });

    // Wait for the URL to change to /login (Inertia handles this via JavaScript)
    await page.waitForURL(/\/login/, { timeout: 10000 });
    console.log("Current URL after logout attempt:", page.url());

    // Test passes if we're back at the login page
    expect(page.url()).toContain("/login");
  });
});
