//go:build matomo_fixtures

package user_agent_test

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"

	"fusionaly/internal/pkg/user_agent"
)

// TestFixture represents a test case from Matomo's fixtures
type TestFixture struct {
	UserAgent string `yaml:"user_agent"`
	OS        struct {
		Name     string `yaml:"name"`
		Version  string `yaml:"version"`
		Platform string `yaml:"platform"`
	} `yaml:"os"`
	Client struct {
		Type          string `yaml:"type"`
		Name          string `yaml:"name"`
		Version       string `yaml:"version"`
		Engine        string `yaml:"engine"`
		EngineVersion string `yaml:"engine_version"`
	} `yaml:"client"`
	Device struct {
		Type  string `yaml:"type"`
		Brand string `yaml:"brand"`
		Model string `yaml:"model"`
	} `yaml:"device"`
	OSFamily      string `yaml:"os_family"`
	BrowserFamily string `yaml:"browser_family"`
}

func loadFixtures(t *testing.T, filename string) []TestFixture {
	// Try to load from fixtures/user_agent directory
	possiblePaths := []string{
		"../../../../fixtures/user_agent/" + filename,
		"../../../fixtures/user_agent/" + filename,
	}

	var data []byte
	var err error

	for _, path := range possiblePaths {
		if data, err = os.ReadFile(path); err == nil {
			break
		}
	}

	if err != nil {
		t.Skipf("Could not load fixture file %s: %v", filename, err)
		return nil
	}

	var fixtures []TestFixture
	if err := yaml.Unmarshal(data, &fixtures); err != nil {
		t.Fatalf("Failed to parse fixture file %s: %v", filename, err)
	}

	return fixtures
}

func TestMatomoFixturesDesktop(t *testing.T) {
	fixtures := loadFixtures(t, "desktop.yml")
	if fixtures == nil {
		return
	}

	passed := 0
	failed := 0

	for i, fixture := range fixtures {
		if i >= 10 { // Limit to first 10 tests to avoid overwhelming output
			break
		}

		t.Run(fixture.UserAgent[:min(50, len(fixture.UserAgent))], func(t *testing.T) {
			result := user_agent.ParseUserAgent(fixture.UserAgent)

			t.Logf("Testing: %s", fixture.UserAgent)
			t.Logf("Expected - OS: %s, Browser: %s, Device: %s", fixture.OS.Name, fixture.Client.Name, fixture.Device.Type)
			t.Logf("Got      - OS: %s, Browser: %s, Device: %s", result.OS, result.Browser, result.Device)

			// Check OS
			if result.OS != fixture.OS.Name {
				t.Errorf("OS mismatch: expected %s, got %s", fixture.OS.Name, result.OS)
				failed++
			}

			// Check Browser
			if result.Browser != fixture.Client.Name {
				t.Errorf("Browser mismatch: expected %s, got %s", fixture.Client.Name, result.Browser)
				failed++
			}

			// Check Device Type
			expectedMobile := fixture.Device.Type == "smartphone" || fixture.Device.Type == "feature phone"
			expectedTablet := fixture.Device.Type == "tablet"
			expectedDesktop := fixture.Device.Type == "desktop"

			if result.Mobile != expectedMobile {
				t.Errorf("Mobile mismatch: expected %v, got %v", expectedMobile, result.Mobile)
				failed++
			}

			if result.Tablet != expectedTablet {
				t.Errorf("Tablet mismatch: expected %v, got %v", expectedTablet, result.Tablet)
				failed++
			}

			if result.Desktop != expectedDesktop {
				t.Errorf("Desktop mismatch: expected %v, got %v", expectedDesktop, result.Desktop)
				failed++
			}

			if failed == 0 {
				passed++
			}
		})
	}

	t.Logf("Desktop fixtures: %d passed, %d failed", passed, failed)
}

func TestMatomoFixturesSmartphone(t *testing.T) {
	fixtures := loadFixtures(t, "smartphone.yml")
	if fixtures == nil {
		return
	}

	passed := 0
	failed := 0

	for i, fixture := range fixtures {
		if i >= 10 { // Limit to first 10 tests
			break
		}

		t.Run(fixture.UserAgent[:min(50, len(fixture.UserAgent))], func(t *testing.T) {
			result := user_agent.ParseUserAgent(fixture.UserAgent)

			t.Logf("Testing: %s", fixture.UserAgent)
			t.Logf("Expected - OS: %s, Browser: %s, Device: %s", fixture.OS.Name, fixture.Client.Name, fixture.Device.Type)
			t.Logf("Got      - OS: %s, Browser: %s, Device: %s", result.OS, result.Browser, result.Device)

			// Check OS
			if result.OS != fixture.OS.Name {
				t.Errorf("OS mismatch: expected %s, got %s", fixture.OS.Name, result.OS)
				failed++
			}

			// Check Browser
			if result.Browser != fixture.Client.Name {
				t.Errorf("Browser mismatch: expected %s, got %s", fixture.Client.Name, result.Browser)
				failed++
			}

			// Check that it's detected as mobile
			if !result.Mobile {
				t.Errorf("Should be detected as mobile device")
				failed++
			}

			if failed == 0 {
				passed++
			}
		})
	}

	t.Logf("Smartphone fixtures: %d passed, %d failed", passed, failed)
}

func TestMatomoFixturesTablet(t *testing.T) {
	fixtures := loadFixtures(t, "tablet.yml")
	if fixtures == nil {
		return
	}

	passed := 0
	failed := 0

	for i, fixture := range fixtures {
		if i >= 10 { // Limit to first 10 tests
			break
		}

		t.Run(fixture.UserAgent[:min(50, len(fixture.UserAgent))], func(t *testing.T) {
			result := user_agent.ParseUserAgent(fixture.UserAgent)

			t.Logf("Testing: %s", fixture.UserAgent)
			t.Logf("Expected - OS: %s, Browser: %s, Device: %s", fixture.OS.Name, fixture.Client.Name, fixture.Device.Type)
			t.Logf("Got      - OS: %s, Browser: %s, Device: %s", result.OS, result.Browser, result.Device)

			// Check OS
			if result.OS != fixture.OS.Name {
				t.Errorf("OS mismatch: expected %s, got %s", fixture.OS.Name, result.OS)
				failed++
			}

			// Check Browser
			if result.Browser != fixture.Client.Name {
				t.Errorf("Browser mismatch: expected %s, got %s", fixture.Client.Name, result.Browser)
				failed++
			}

			// Check that it's detected as tablet
			if !result.Tablet {
				t.Errorf("Should be detected as tablet device")
				failed++
			}

			if failed == 0 {
				passed++
			}
		})
	}

	t.Logf("Tablet fixtures: %d passed, %d failed", passed, failed)
}

// Simple test to debug pattern matching
func TestSimplePatterns(t *testing.T) {
	testCases := []struct {
		name      string
		userAgent string
		desc      string
	}{
		{
			name:      "Chrome",
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
			desc:      "Standard Chrome on Windows",
		},
		{
			name:      "Safari",
			userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 14_6 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/14.0 Mobile/15E148 Safari/604.1",
			desc:      "Safari on iPhone",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := user_agent.ParseUserAgent(tc.userAgent)

			t.Logf("Testing: %s", tc.desc)
			t.Logf("User Agent: %s", tc.userAgent)
			t.Logf("Result: Browser=%s, OS=%s, Device=%s, Mobile=%v, Tablet=%v, Desktop=%v",
				result.Browser, result.OS, result.Device, result.Mobile, result.Tablet, result.Desktop)
		})
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
