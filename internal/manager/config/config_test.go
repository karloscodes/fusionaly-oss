package config

import (
	"os"
	"strings"
	"testing"

	"fusionaly/internal/manager/logging"
)

func testLogger(t *testing.T) *logging.Logger {
	dir := t.TempDir()
	logger := logging.NewLogger(logging.Config{LogDir: dir})
	return logger
}

func TestValidate_AllFieldsPresent(t *testing.T) {
	c := NewConfig(testLogger(t))
	c.SetInstallDir("/test/dir")
	c.SetCaddyImage("caddy:test")
	c.SetInstallerURL("http://example.com")
	c.data.Domain = "example.com"
	c.data.AppImage = "appimg"
	c.data.CaddyImage = "caddyimg"
	c.data.InstallDir = "/test/dir"
	c.data.BackupPath = "/backup"
	c.data.PrivateKey = "this-is-a-very-long-private-key-that-meets-minimum-requirements"
	c.data.Version = "v1.0.0"
	c.data.InstallerURL = "https://example.com/installer"
	// Test Validate
	err := c.Validate()
	if err != nil {
		t.Errorf("Validate() returned error with all fields present: %v", err)
	}
}

func TestValidate_MissingFields(t *testing.T) {
	fields := []struct {
		name    string
		modify  func(c *Config)
		wantErr string
	}{
		{"Domain", func(c *Config) { c.data.Domain = "" }, "config error for field 'domain': validation failed for field 'domain': domain cannot be empty"},
		{"AppImage", func(c *Config) { c.data.AppImage = "" }, "config error for field 'app_image': app image cannot be empty"},
		{"CaddyImage", func(c *Config) { c.data.CaddyImage = "" }, "config error for field 'caddy_image': caddy image cannot be empty"},
		{"InstallDir", func(c *Config) { c.data.InstallDir = "" }, "config error for field 'install_dir': validation failed for field 'file_path': file path cannot be empty"},
		{"BackupPath", func(c *Config) { c.data.BackupPath = "" }, "config error for field 'backup_path': validation failed for field 'file_path': file path cannot be empty"},
		{"PrivateKey", func(c *Config) { c.data.PrivateKey = "" }, "config error for field 'private_key': private key cannot be empty"},
	}
	for _, tc := range fields {
		t.Run(tc.name, func(t *testing.T) {
			c := NewConfig(testLogger(t))
			c.data.Domain = "example.com"
			c.data.AppImage = "appimg"
			c.data.CaddyImage = "caddyimg"
			c.data.InstallDir = "/test/dir"
			c.data.BackupPath = "/backup"
			c.data.PrivateKey = "this-is-a-very-long-private-key-that-meets-minimum-requirements"
			c.data.Version = "v1.0.0"
			c.data.InstallerURL = "https://example.com/installer"
			tc.modify(c)
			err := c.Validate()
			if err == nil || err.Error() != tc.wantErr {
				t.Errorf("Validate() missing %s: got %v, want %q", tc.name, err, tc.wantErr)
			}
		})
	}
}

func TestSettersAndGetters(t *testing.T) {
	c := NewConfig(testLogger(t))
	c.SetInstallDir("/foo/bar")
	if got := c.GetMainDBPath(); got != "/foo/bar/storage/fusionaly-production.db" {
		t.Errorf("GetMainDBPath() = %q, want %q", got, "/foo/bar/storage/fusionaly-production.db")
	}
	c.SetCaddyImage("caddy:custom")
	if c.data.CaddyImage != "caddy:custom" {
		t.Errorf("SetCaddyImage() did not update field")
	}
	c.SetInstallerURL("http://installer")
	if c.data.InstallerURL != "http://installer" {
		t.Errorf("SetInstallerURL() did not update field")
	}
}

func TestGeneratePrivateKey_Uniqueness(t *testing.T) {
	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key, err := generatePrivateKey()
		if err != nil {
			t.Fatalf("generatePrivateKey() error: %v", err)
		}
		if len(key) != 32 {
			t.Errorf("generatePrivateKey() length = %d, want 32", len(key))
		}
		if keys[key] {
			t.Errorf("generatePrivateKey() produced duplicate key: %s", key)
		}
		keys[key] = true
	}
}

func TestNewConfig_Defaults(t *testing.T) {
	c := NewConfig(testLogger(t))
	data := c.data
	if data.AppImage != "karloscodes/fusionaly:latest" {
		t.Errorf("AppImage default = %q, want %q", data.AppImage, "karloscodes/fusionaly:latest")
	}
	if data.CaddyImage != "caddy:2.7-alpine" {
		t.Errorf("CaddyImage default = %q, want %q", data.CaddyImage, "caddy:2.7-alpine")
	}
	if data.InstallDir != "/opt/fusionaly" {
		t.Errorf("InstallDir default = %q, want %q", data.InstallDir, "/opt/fusionaly")
	}
}

func TestGetData(t *testing.T) {
	c := NewConfig(testLogger(t))
	c.data.Domain = "test.example.com"

	data := c.GetData()
	if data.Domain != "test.example.com" {
		t.Errorf("GetData().Domain = %q, want %q", data.Domain, "test.example.com")
	}
}

func TestGetDockerImages(t *testing.T) {
	c := NewConfig(testLogger(t))
	c.data.AppImage = "custom/app:1.0"
	c.data.CaddyImage = "custom/caddy:2.0"

	images := c.GetDockerImages()
	if images.AppImage != "custom/app:1.0" {
		t.Errorf("GetDockerImages().AppImage = %q, want %q", images.AppImage, "custom/app:1.0")
	}
	if images.CaddyImage != "custom/caddy:2.0" {
		t.Errorf("GetDockerImages().CaddyImage = %q, want %q", images.CaddyImage, "custom/caddy:2.0")
	}
}

func TestLoadFromFile(t *testing.T) {
	// Test loading valid .env file
	t.Run("ValidFile", func(t *testing.T) {
		c := NewConfig(testLogger(t))

		// Create temp file with config data
		tmpFile := t.TempDir() + "/test.env"
		content := `FUSIONALY_DOMAIN=test.example.com
APP_IMAGE=test/app:latest
CADDY_IMAGE=test/caddy:latest
INSTALL_DIR=/custom/install
BACKUP_PATH=/custom/backup
VERSION=1.2.3
INSTALLER_URL=https://test.com/installer
FUSIONALY_PRIVATE_KEY=testprivatekey123
`
		if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		err := c.LoadFromFile(tmpFile)
		if err != nil {
			t.Errorf("LoadFromFile() error = %v", err)
		}

		// Verify loaded values
		if c.data.Domain != "test.example.com" {
			t.Errorf("Domain = %q, want %q", c.data.Domain, "test.example.com")
		}
		if c.data.AppImage != "test/app:latest" {
			t.Errorf("AppImage = %q, want %q", c.data.AppImage, "test/app:latest")
		}
		if c.data.CaddyImage != "test/caddy:latest" {
			t.Errorf("CaddyImage = %q, want %q", c.data.CaddyImage, "test/caddy:latest")
		}
		if c.data.InstallDir != "/custom/install" {
			t.Errorf("InstallDir = %q, want %q", c.data.InstallDir, "/custom/install")
		}
		if c.data.BackupPath != "/custom/backup" {
			t.Errorf("BackupPath = %q, want %q", c.data.BackupPath, "/custom/backup")
		}
		if c.data.Version != "1.2.3" {
			t.Errorf("Version = %q, want %q", c.data.Version, "1.2.3")
		}
		if c.data.InstallerURL != "https://test.com/installer" {
			t.Errorf("InstallerURL = %q, want %q", c.data.InstallerURL, "https://test.com/installer")
		}
		if c.data.PrivateKey != "testprivatekey123" {
			t.Errorf("PrivateKey = %q, want %q", c.data.PrivateKey, "testprivatekey123")
		}
	})

	// Test missing private key generation
	t.Run("MissingPrivateKey", func(t *testing.T) {
		c := NewConfig(testLogger(t))

		tmpFile := t.TempDir() + "/test.env"
		content := `FUSIONALY_DOMAIN=test.example.com
`
		if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}

		err := c.LoadFromFile(tmpFile)
		if err != nil {
			t.Errorf("LoadFromFile() error = %v", err)
		}

		// Should have generated a private key
		if c.data.PrivateKey == "" {
			t.Error("PrivateKey should be generated when missing")
		}
		if len(c.data.PrivateKey) != 32 {
			t.Errorf("Generated PrivateKey length = %d, want 32", len(c.data.PrivateKey))
		}
	})

	// Test nonexistent file
	t.Run("NonexistentFile", func(t *testing.T) {
		c := NewConfig(testLogger(t))
		err := c.LoadFromFile("/nonexistent/file.env")
		if err == nil {
			t.Error("LoadFromFile() should return error for nonexistent file")
		}
	})
}

func TestSaveToFile(t *testing.T) {
	c := NewConfig(testLogger(t))
	c.data.Domain = "save.example.com"
	c.data.AppImage = "save/app:latest"
	c.data.CaddyImage = "save/caddy:latest"
	c.data.InstallDir = "/save/install"
	c.data.BackupPath = "/save/backup"
	c.data.Version = "2.0.0"
	c.data.InstallerURL = "https://save.com/installer"

	tmpFile := t.TempDir() + "/save.env"

	err := c.SaveToFile(tmpFile)
	if err != nil {
		t.Errorf("SaveToFile() error = %v", err)
	}

	// Read the file back and verify content
	content, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	expected := []string{
		"FUSIONALY_DOMAIN=save.example.com",
		"APP_IMAGE=save/app:latest",
		"CADDY_IMAGE=save/caddy:latest",
		"INSTALL_DIR=/save/install",
		"BACKUP_PATH=/save/backup",
		"VERSION=2.0.0",
		"INSTALLER_URL=https://save.com/installer",
		"FUSIONALY_PRIVATE_KEY=", // Should be generated
	}

	contentStr := string(content)
	for _, expectedLine := range expected {
		if !strings.Contains(contentStr, expectedLine) {
			t.Errorf("SaveToFile() missing expected line: %s", expectedLine)
		}
	}

	// Verify private key was generated and saved
	if !strings.Contains(contentStr, "FUSIONALY_PRIVATE_KEY=") {
		t.Error("SaveToFile() should include FUSIONALY_PRIVATE_KEY")
	}
}

func TestDNSWarnings(t *testing.T) {
	c := NewConfig(testLogger(t))

	// Initially no warnings
	if c.HasDNSWarnings() {
		t.Error("HasDNSWarnings() should be false initially")
	}
	if len(c.GetDNSWarnings()) != 0 {
		t.Error("GetDNSWarnings() should be empty initially")
	}

	// Add some warnings manually for testing
	c.data.DNSWarnings = []string{"Warning 1", "Warning 2"}

	if !c.HasDNSWarnings() {
		t.Error("HasDNSWarnings() should be true after adding warnings")
	}

	warnings := c.GetDNSWarnings()
	if len(warnings) != 2 {
		t.Errorf("GetDNSWarnings() length = %d, want 2", len(warnings))
	}
	if warnings[0] != "Warning 1" {
		t.Errorf("GetDNSWarnings()[0] = %q, want %q", warnings[0], "Warning 1")
	}
	if warnings[1] != "Warning 2" {
		t.Errorf("GetDNSWarnings()[1] = %q, want %q", warnings[1], "Warning 2")
	}
}

func TestCheckDNSAndStoreWarnings(t *testing.T) {
	c := NewConfig(testLogger(t))

	// Test with invalid domain (should generate warnings)
	c.CheckDNSAndStoreWarnings("invalid-domain-that-does-not-exist.nonexistent")

	if !c.HasDNSWarnings() {
		t.Error("CheckDNSAndStoreWarnings() should generate warnings for invalid domain")
	}

	warnings := c.GetDNSWarnings()
	if len(warnings) == 0 {
		t.Error("CheckDNSAndStoreWarnings() should add warnings for invalid domain")
	}

	// Verify that at least one warning mentions DNS lookup failure
	found := false
	for _, warning := range warnings {
		if strings.Contains(warning, "DNS lookup failed") {
			found = true
			break
		}
	}
	if !found {
		t.Error("Expected DNS lookup failure warning not found")
	}
}

func TestCheckDNSAndStoreWarningsSkipsLocalhost(t *testing.T) {
	c := NewConfig(testLogger(t))

	// Test with localhost domain (should skip checks and have no warnings)
	c.CheckDNSAndStoreWarnings("localhost")

	if c.HasDNSWarnings() {
		t.Error("CheckDNSAndStoreWarnings() should not generate warnings for localhost")
	}

	warnings := c.GetDNSWarnings()
	if len(warnings) != 0 {
		t.Errorf("CheckDNSAndStoreWarnings() should not add warnings for localhost, got %d warnings: %v", len(warnings), warnings)
	}

	// Test with other localhost variants
	localhostVariants := []string{
		"LOCALHOST",
		"127.0.0.1",
		"::1",
		"localhost:8080",
		"app.localhost",
		"localhost.localdomain",
	}

	for _, variant := range localhostVariants {
		c.CheckDNSAndStoreWarnings(variant)
		if c.HasDNSWarnings() {
			t.Errorf("CheckDNSAndStoreWarnings() should not generate warnings for localhost variant: %s", variant)
		}
	}
}

func TestCollectFromEnvironment(t *testing.T) {
	c := NewConfig(testLogger(t))

	// Set environment variables
	originalEnv := make(map[string]string)
	envVars := map[string]string{
		"NONINTERACTIVE": "1",
		"DOMAIN":         "env.example.com",
	}

	// Backup and set environment variables
	for key, value := range envVars {
		originalEnv[key] = os.Getenv(key)
		os.Setenv(key, value)
	}

	// Restore environment after test
	defer func() {
		for key, original := range originalEnv {
			if original == "" {
				os.Unsetenv(key)
			} else {
				os.Setenv(key, original)
			}
		}
	}()

	// Test successful collection
	err := c.collectFromEnvironment()
	if err != nil {
		t.Errorf("collectFromEnvironment() error = %v", err)
	}

	if c.data.Domain != "env.example.com" {
		t.Errorf("Domain = %q, want %q", c.data.Domain, "env.example.com")
	}

	// Test missing required variable
	os.Unsetenv("DOMAIN")
	err = c.collectFromEnvironment()
	if err == nil {
		t.Error("collectFromEnvironment() should return error when DOMAIN is missing")
	}
}

func TestConfigurationValidation(t *testing.T) {
	t.Run("ValidateCompleteConfiguration", func(t *testing.T) {
		c := NewConfig(testLogger(t))
		c.data.Domain = "metrics.company.com"
		c.data.AppImage = "karloscodes/fusionaly:latest"
		c.data.CaddyImage = "caddy:2.7-alpine"
		c.data.InstallDir = "/opt/fusionaly"
		c.data.BackupPath = "/opt/fusionaly/backup"
		c.data.PrivateKey = "this-is-a-very-long-private-key-that-meets-minimum-requirements"
		c.data.Version = "v1.0.0"
		c.data.InstallerURL = "https://company.com/installer"
		
		err := c.Validate()
		
		if err != nil {
			t.Errorf("Expected complete configuration to be valid, got error: %v", err)
		}
	})

	t.Run("ValidateRejectsMissingDomain", func(t *testing.T) {
		c := NewConfig(testLogger(t))
		// Domain is intentionally missing
		
		err := c.Validate()
		
		if err == nil {
			t.Error("Expected validation to fail when domain is missing")
		}
		if err != nil && !strings.Contains(err.Error(), "domain") {
			t.Errorf("Expected error to mention domain, got: %v", err)
		}
	})

}

func TestConfigurationDefaults(t *testing.T) {
	t.Run("NewConfigurationSetsDefaults", func(t *testing.T) {
		c := NewConfig(testLogger(t))
		data := c.GetData()
		
		expectedDefaults := map[string]string{
			"AppImage":   "karloscodes/fusionaly:latest",
			"CaddyImage": "caddy:2.7-alpine",
			"InstallDir": "/opt/fusionaly",
		}
		
		if data.AppImage != expectedDefaults["AppImage"] {
			t.Errorf("Expected default AppImage %s, got %s", expectedDefaults["AppImage"], data.AppImage)
		}
		if data.CaddyImage != expectedDefaults["CaddyImage"] {
			t.Errorf("Expected default CaddyImage %s, got %s", expectedDefaults["CaddyImage"], data.CaddyImage)
		}
		if data.InstallDir != expectedDefaults["InstallDir"] {
			t.Errorf("Expected default InstallDir %s, got %s", expectedDefaults["InstallDir"], data.InstallDir)
		}
		
		// Private key is generated when needed, not by default
		t.Logf("Private key status: length=%d", len(data.PrivateKey))
	})
}

func TestEnvironmentConfigCollection(t *testing.T) {
	// Save original environment
	originalDomain := os.Getenv("DOMAIN")
	
	defer func() {
		// Restore original environment
		os.Setenv("DOMAIN", originalDomain)
	}()

	t.Run("PopulatesFromEnvironmentVariables", func(t *testing.T) {
		os.Setenv("DOMAIN", "env.company.com")
		
		c := NewConfig(testLogger(t))
		err := c.collectFromEnvironment()
		
		if err != nil {
			t.Errorf("Expected environment collection to succeed, got error: %v", err)
		}
		
		data := c.GetData()
		if data.Domain != "env.company.com" {
			t.Errorf("Expected domain from environment, got %s", data.Domain)
		}
	})

	t.Run("ReturnsErrorForMissingEnvironmentVars", func(t *testing.T) {
		os.Unsetenv("DOMAIN")
		
		c := NewConfig(testLogger(t))
		err := c.collectFromEnvironment()
		
		if err == nil {
			t.Error("Expected error when required environment variables are missing")
		}
	})
}
