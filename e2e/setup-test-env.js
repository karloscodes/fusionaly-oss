const { execSync } = require("node:child_process");
const path = require("node:path");
const fs = require("node:fs");

// Ensure we're using the test environment
process.env.FUSIONALY_ENV = "test";

console.log("=== E2E Test Environment Setup ===");
console.log(`Environment: ${process.env.FUSIONALY_ENV}`);
console.log(`Node version: ${process.version}`);
console.log(`Platform: ${process.platform}`);

// Helper function to run commands with better error handling
function runCommand(command, description, options = {}) {
	console.log(`\n📋 ${description}...`);
	try {
		const result = execSync(command, {
			stdio: "inherit",
			encoding: "utf8",
			...options,
		});
		console.log(`✅ ${description} completed successfully`);
		return result;
	} catch (error) {
		console.error(`❌ ${description} failed:`);
		console.error(`Command: ${command}`);
		console.error(`Exit code: ${error.status}`);
		console.error(`Error: ${error.message}`);
		throw error;
	}
}

// Helper function to check if a file exists
function checkFileExists(filePath, description) {
	if (fs.existsSync(filePath)) {
		console.log(`✅ ${description} exists: ${filePath}`);
		return true;
	} else {
		console.log(`❌ ${description} missing: ${filePath}`);
		return false;
	}
}

// Helper function to validate database
function validateDatabase(projectRoot) {
	const dbPath = path.join(projectRoot, "storage", "fusionaly-test.db");
	if (!checkFileExists(dbPath, "Test database")) {
		throw new Error("Test database was not created properly");
	}

	// Check if user was created
	// try {
	// 	const result = runCommand(
	// 		`sqlite3 "${dbPath}" "SELECT email FROM users WHERE email = 'test@example.com';"`,
	// 		"Validating test user",
	// 		{
	// 			stdio: "pipe",
	// 			env: { ...process.env, FUSIONALY_ENV: "test" },
	// 			cwd: projectRoot,
	// 		}
	// 	);

	// 	if (result.trim() === "test@example.com") {
	// 		console.log("✅ Test user validation successful");
	// 	} else {
	// 		throw new Error("Test user not found in database");
	// 	}
	// } catch (error) {
	// 	console.error("❌ Test user validation failed:", error.message);
	// 	throw error;
	// }
}

async function setupTestEnvironment() {
	try {
		// Get the project root directory (parent of e2e)
		const projectRoot = path.resolve(__dirname, "..");
		console.log(`📁 Project root: ${projectRoot}`);

		// Validate project structure
		const requiredPaths = [
			path.join(projectRoot, "go.mod"),
			path.join(projectRoot, "cmd/fusionaly/main.go"),
			path.join(projectRoot, "cmd/fnctl/main.go"),
			path.join(projectRoot, "Makefile"),
		];

		for (const requiredPath of requiredPaths) {
			if (!checkFileExists(requiredPath, `Required file ${path.basename(requiredPath)}`)) {
				throw new Error(`Missing required file: ${requiredPath}`);
			}
		}

		// Ensure storage directory exists
		const storageDir = path.join(projectRoot, "storage");
		if (!fs.existsSync(storageDir)) {
			console.log("📁 Creating storage directory...");
			fs.mkdirSync(storageDir, { recursive: true });
		}

		// Build database tools with timeout
		runCommand(
			"make db-build-tools",
			"Building database tools",
			{
				env: { ...process.env, FUSIONALY_ENV: "test" },
				cwd: projectRoot,
				timeout: 60000, // 60 second timeout
			}
		);

		// Verify fnctl was built
		const fnctlPath = path.join(projectRoot, "tmp/fnctl");
		if (!checkFileExists(fnctlPath, "fnctl binary")) {
			throw new Error("fnctl binary was not built successfully");
		}

		// Build web assets for e2e tests
		runCommand(
			"cd web && npm run build",
			"Building web assets for e2e tests",
			{
				env: { ...process.env, FUSIONALY_ENV: "test" },
				cwd: projectRoot,
				timeout: 120000, // 2 minute timeout for npm build
			}
		);

		// Clean the test database
		runCommand(
			"make db-drop",
			"Dropping test database",
			{
				env: { ...process.env, FUSIONALY_ENV: "test" },
				cwd: projectRoot,
			}
		);

		// Run migrations for test database
		runCommand(
			"make db-migrate",
			"Running database migrations",
			{
				env: { ...process.env, FUSIONALY_ENV: "test" },
				cwd: projectRoot,
			}
		);

		// Create localhost website for event ingestion tests
		// This is needed because the SDK validates the origin against registered websites
		const dbPath = path.join(projectRoot, "storage", "fusionaly-test.db");
		runCommand(
			`sqlite3 "${dbPath}" "INSERT OR IGNORE INTO websites (domain, created_at) VALUES ('localhost', datetime('now'));"`,
			"Creating localhost website for event tests",
			{
				env: { ...process.env, FUSIONALY_ENV: "test" },
				cwd: projectRoot,
			}
		);

		// Create test user with validation (skip for onboarding tests)
		// const skipUserCreation = process.env.SKIP_TEST_USER_CREATION === "true";
		// if (!skipUserCreation) {
		// 	runCommand(
		// 		`"${fnctlPath}" create-admin-user test@example.com password`,
		// 		"Creating test admin user",
		// 		{
		// 			env: { ...process.env, FUSIONALY_ENV: "test" },
		// 			cwd: projectRoot,
		// 		}
		// 	);

		// 	// Validate the setup
		// 	validateDatabase(projectRoot);
		// } else {
		// 	console.log("⚠️ Skipping test user creation for onboarding tests");
		// }

		console.log("\n🎉 E2E test environment setup completed successfully!");
		console.log("📊 Setup summary:");
		console.log("  - Database: ✅ Created and migrated");
		console.log("  - Tools: ✅ Built and validated");

	} catch (error) {
		console.error("\n💥 E2E test environment setup failed:");
		console.error(error.message);
		console.error("\n🔍 Troubleshooting tips:");
		console.error("  1. Ensure Go is installed and in PATH");
		console.error("  2. Run 'make install' to install dependencies");
		console.error("  3. Check if port 3000 is available");
		console.error("  4. Verify FUSIONALY_ENV=test is set");
		process.exit(1);
	}
}

// Run setup if this file is executed directly
if (require.main === module) {
	setupTestEnvironment();
}

module.exports = setupTestEnvironment;
