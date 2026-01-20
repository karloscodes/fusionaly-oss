package upgrader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fusionaly/internal/manager/logging"
)

func TestNewUpgrader(t *testing.T) {
	logger := logging.NewLogger(logging.Config{Level: "error", Quiet: true})
	upg := NewUpgrader(logger)

	if upg == nil {
		t.Fatal("NewUpgrader returned nil")
	}
	if upg.config == nil {
		t.Fatal("config is nil")
	}
	if upg.docker == nil {
		t.Fatal("docker is nil")
	}
	if upg.database == nil {
		t.Fatal("database is nil")
	}
}

func TestProAppImageConstant(t *testing.T) {
	expected := "karloscodes/fusionaly-pro:latest"
	if ProAppImage != expected {
		t.Errorf("ProAppImage = %q, want %q", ProAppImage, expected)
	}
}

func TestUpgraderDetectsAlreadyPro(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	// Create .env with Pro image already set
	envContent := `FUSIONALY_DOMAIN=localhost
APP_IMAGE=karloscodes/fusionaly-pro:latest
CADDY_IMAGE=caddy:2.9-alpine
INSTALL_DIR=` + tmpDir + `
BACKUP_PATH=` + filepath.Join(tmpDir, "backups") + `
FUSIONALY_PRIVATE_KEY=12345678901234567890123456789012
`
	if err := os.WriteFile(envFile, []byte(envContent), 0o644); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	// Create required directories
	os.MkdirAll(filepath.Join(tmpDir, "storage"), 0o755)
	os.MkdirAll(filepath.Join(tmpDir, "backups"), 0o755)

	logger := logging.NewLogger(logging.Config{Level: "error", Quiet: true})
	upg := NewUpgrader(logger)

	// Override install dir to use temp dir
	upg.config.SetInstallDir(tmpDir)

	// Load config
	if err := upg.config.LoadFromFile(envFile); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Check that it detects already running Pro
	data := upg.config.GetData()
	if data.AppImage != ProAppImage {
		t.Errorf("AppImage = %q, want %q", data.AppImage, ProAppImage)
	}
}

func TestUpgraderConfigSwitch(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	// Create .env with OSS image
	envContent := `FUSIONALY_DOMAIN=test.example.com
APP_IMAGE=karloscodes/fusionaly-beta:latest
CADDY_IMAGE=caddy:2.9-alpine
INSTALL_DIR=` + tmpDir + `
BACKUP_PATH=` + filepath.Join(tmpDir, "backups") + `
FUSIONALY_PRIVATE_KEY=12345678901234567890123456789012
`
	if err := os.WriteFile(envFile, []byte(envContent), 0o644); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	logger := logging.NewLogger(logging.Config{Level: "error", Quiet: true})
	upg := NewUpgrader(logger)

	// Load config
	if err := upg.config.LoadFromFile(envFile); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// Verify initial state is OSS
	data := upg.config.GetData()
	if data.AppImage != "karloscodes/fusionaly-beta:latest" {
		t.Errorf("initial AppImage = %q, want OSS image", data.AppImage)
	}

	// Simulate the config switch (without actually running Docker commands)
	data.AppImage = ProAppImage
	upg.config.SetData(data)

	// Save config
	if err := upg.config.SaveToFile(envFile); err != nil {
		t.Fatalf("failed to save config: %v", err)
	}

	// Verify saved config has Pro image
	content, err := os.ReadFile(envFile)
	if err != nil {
		t.Fatalf("failed to read env file: %v", err)
	}

	if !strings.Contains(string(content), "APP_IMAGE=karloscodes/fusionaly-pro:latest") {
		t.Errorf("saved config does not contain Pro image, got:\n%s", content)
	}
}

func TestGetDomain(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")

	envContent := `FUSIONALY_DOMAIN=analytics.example.com
APP_IMAGE=karloscodes/fusionaly-beta:latest
CADDY_IMAGE=caddy:2.9-alpine
INSTALL_DIR=` + tmpDir + `
BACKUP_PATH=` + filepath.Join(tmpDir, "backups") + `
FUSIONALY_PRIVATE_KEY=12345678901234567890123456789012
`
	if err := os.WriteFile(envFile, []byte(envContent), 0o644); err != nil {
		t.Fatalf("failed to write env file: %v", err)
	}

	logger := logging.NewLogger(logging.Config{Level: "error", Quiet: true})
	upg := NewUpgrader(logger)

	if err := upg.config.LoadFromFile(envFile); err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	domain := upg.GetDomain()
	if domain != "analytics.example.com" {
		t.Errorf("GetDomain() = %q, want %q", domain, "analytics.example.com")
	}
}
