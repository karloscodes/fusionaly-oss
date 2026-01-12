package updater

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fusionaly/internal/manager/logging"
)

// TestPrivateKeyGeneration ensures that updater Run saves private key when missing.
func TestPrivateKeyGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	envFile := filepath.Join(tmpDir, ".env")
	os.WriteFile(envFile, []byte("FUSIONALY_DOMAIN=localhost\n"), 0o644)

	logger := logging.NewLogger(logging.Config{Level: "error"})
	u := NewUpdater(logger)

	// Directly use config load/save logic which generates the key when missing
	cfg := u.config
	if err := cfg.LoadFromFile(envFile); err != nil {
		t.Fatalf("load err: %v", err)
	}
	if cfg.GetData().PrivateKey == "" {
		t.Fatalf("expected private key to be generated on load")
	}

	content, _ := os.ReadFile(envFile)
	if !strings.Contains(string(content), "FUSIONALY_PRIVATE_KEY") {
		t.Fatalf("env file missing generated key")
	}

	// Call SaveToFile and ensure key persists
	if err := cfg.SaveToFile(envFile); err != nil {
		t.Fatalf("save err: %v", err)
	}
	content2, _ := os.ReadFile(envFile)
	if !strings.Contains(string(content2), cfg.GetData().PrivateKey) {
		t.Fatalf("saved key not found in env file")
	}
}

func TestCompareVersions(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		exp  int
	}{
		{"minor lt", "1.0.0", "1.2.0", -1},
		{"major gt", "2.0.0", "1.9.9", 1},
		{"equal", "1.0.0", "1.0.0", 0},
		{"different segment lengths eq", "1.0", "1.0.0", 0},
		{"patch gt", "1.0.10", "1.0.2", 1},
		{"numeric compare not lexicographic", "1.10.1", "1.9.9", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := compareVersions(c.a, c.b); got != c.exp {
				t.Fatalf("compareVersions(%s,%s)=%d want %d", c.a, c.b, got, c.exp)
			}
		})
	}
}

func TestExtractVersionFromURL(t *testing.T) {
	tests := map[string]string{
		// New naming pattern
		"https://github.com/karloscodes/fusionaly-installer/releases/download/v1.2.3/fusionaly-installer-v1.2.3-amd64": "1.2.3",
		// Old naming pattern (backwards compatibility)
		"https://github.com/karloscodes/fusionaly-installer/releases/download/v1.2.3/fusionaly-v1.2.3-amd64": "1.2.3",
		"https://no-version.com/asset": "",
	}
	for url, want := range tests {
		if got := extractVersionFromURL(url); got != want {
			t.Errorf("extractVersionFromURL(%s)=%s want %s", url, got, want)
		}
	}
}
