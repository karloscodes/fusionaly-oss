/**
 * UI Comparison Script
 *
 * Captures screenshots from both the original fusionaly and fusionaly-oss/pro
 * to verify visual parity.
 *
 * Usage:
 *   1. Start the app you want to capture on port 3000
 *   2. Run: node ui-comparison.js <original|oss|pro>
 *   3. Screenshots saved to e2e/screenshots/<variant>/
 *   4. Compare visually or use image diff tools
 */

const { chromium } = require("@playwright/test");
const fs = require("fs");
const path = require("path");

const VARIANT = process.argv[2] || "original";
const BASE_URL = process.env.BASE_URL || "http://localhost:3000";
const SCREENSHOT_DIR = path.join(__dirname, "screenshots", VARIANT);

// Test credentials - can be overridden via env vars
const TEST_EMAIL = process.env.TEST_EMAIL || "admin@test-e2e.com";
const TEST_PASSWORD = process.env.TEST_PASSWORD || "testpassword123";

// Pages to capture (common to both OSS and original)
const PAGES_TO_CAPTURE = [
  { name: "01-login", path: "/login", requiresAuth: false },
  { name: "02-setup", path: "/setup", requiresAuth: false, skipIfLoggedIn: true },
  { name: "03-websites-list", path: "/admin", requiresAuth: true },
  { name: "04-websites-new", path: "/admin/websites/new", requiresAuth: true },
  { name: "05-dashboard", path: "/admin/websites/1/dashboard", requiresAuth: true },
  { name: "06-admin-ingestion", path: "/admin/administration/ingestion", requiresAuth: true },
  { name: "07-admin-system", path: "/admin/administration/system", requiresAuth: true },
  { name: "08-admin-account", path: "/admin/administration/account", requiresAuth: true },
];

// Pro-only pages (original has these, OSS doesn't)
const PRO_PAGES = [
  { name: "09-lens", path: "/admin/websites/1/lens", requiresAuth: true },
  { name: "10-admin-ai", path: "/admin/administration/ai", requiresAuth: true },
];

async function login(page) {
  console.log(`  Attempting login with: ${TEST_EMAIL}`);
  await page.goto(`${BASE_URL}/login`);
  await page.waitForSelector('input[name="email"]', { timeout: 5000 });
  await page.fill('input[name="email"]', TEST_EMAIL);
  await page.fill('input[name="password"]', TEST_PASSWORD);
  await page.click('button[type="submit"]');

  // Wait a bit and check where we ended up
  await page.waitForTimeout(3000);
  const currentUrl = page.url();
  console.log(`  After login URL: ${currentUrl}`);

  if (currentUrl.includes('/admin')) {
    console.log("  Logged in successfully");
  } else if (currentUrl.includes('/login')) {
    // Check for error message
    const errorText = await page.textContent('body');
    if (errorText.includes('Invalid') || errorText.includes('error')) {
      throw new Error('Invalid credentials');
    }
    throw new Error('Still on login page');
  } else {
    console.log(`  Redirected to: ${currentUrl}`);
  }
}

async function captureScreenshot(page, pageInfo) {
  const { name, path: pagePath, requiresAuth, skipIfLoggedIn } = pageInfo;

  try {
    // Navigate to page
    const response = await page.goto(`${BASE_URL}${pagePath}`, {
      waitUntil: "networkidle",
      timeout: 15000
    });

    // Check if we got redirected (e.g., to login)
    const currentUrl = page.url();
    if (requiresAuth && currentUrl.includes("/login")) {
      console.log(`  ${name}: Skipped (requires auth, got redirected)`);
      return;
    }

    if (skipIfLoggedIn && !currentUrl.includes(pagePath)) {
      console.log(`  ${name}: Skipped (already logged in)`);
      return;
    }

    // Wait for page to settle
    await page.waitForTimeout(1000);

    // Take screenshot
    const screenshotPath = path.join(SCREENSHOT_DIR, `${name}.png`);
    await page.screenshot({
      path: screenshotPath,
      fullPage: true
    });

    console.log(`  ${name}: Captured (${response?.status() || 'OK'})`);
  } catch (error) {
    console.log(`  ${name}: Error - ${error.message}`);
  }
}

async function run() {
  console.log(`\nUI Comparison: Capturing ${VARIANT} screenshots`);
  console.log(`Base URL: ${BASE_URL}`);
  console.log(`Output: ${SCREENSHOT_DIR}\n`);

  // Create screenshot directory
  fs.mkdirSync(SCREENSHOT_DIR, { recursive: true });

  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1280, height: 720 },
    // Set timezone cookie
    extraHTTPHeaders: {
      'Cookie': '_tz=America%2FNew_York'
    }
  });
  const page = await context.newPage();

  // Set timezone cookie directly
  await context.addCookies([{
    name: '_tz',
    value: 'America/New_York',
    domain: 'localhost',
    path: '/'
  }]);

  // Capture unauthenticated pages first
  console.log("Capturing unauthenticated pages...");
  for (const pageInfo of PAGES_TO_CAPTURE.filter(p => !p.requiresAuth)) {
    await captureScreenshot(page, pageInfo);
  }

  // Login
  console.log("\nLogging in...");
  try {
    await login(page);
  } catch (e) {
    console.log("  Login failed - app might need onboarding first");
    console.log("  Run the E2E tests first to set up test data: make test-e2e");
    await browser.close();
    process.exit(1);
  }

  // Capture authenticated pages
  console.log("\nCapturing authenticated pages...");
  for (const pageInfo of PAGES_TO_CAPTURE.filter(p => p.requiresAuth)) {
    await captureScreenshot(page, pageInfo);
  }

  // Capture Pro pages if variant is 'pro' or 'original' (original has Pro features)
  if (VARIANT === "pro" || VARIANT === "original") {
    console.log("\nCapturing Pro pages...");
    for (const pageInfo of PRO_PAGES) {
      await captureScreenshot(page, pageInfo);
    }
  }

  await browser.close();

  console.log(`\nDone! Screenshots saved to: ${SCREENSHOT_DIR}`);
  console.log("\nTo compare, run both variants and use:");
  console.log("  diff -r screenshots/original screenshots/oss");
  console.log("  # Or use a visual diff tool like 'pixelmatch' or ImageMagick");
}

run().catch(console.error);
