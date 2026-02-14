package snapshot

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Scenario defines a test scenario with inputs and expected behavior
type Scenario struct {
	Name   string
	Inputs []string // Lines to send as stdin
	Env    map[string]string
}

// GetScenarios returns all test scenarios for the installer
func GetScenarios() []Scenario {
	return []Scenario{
		{
			Name:   "happy_path_localhost",
			Inputs: []string{"localhost", "y"},
			Env:    map[string]string{"ENV": "test", "SKIP_PORT_CHECKING": "1"},
		},
		{
			Name:   "invalid_domain_spaces",
			Inputs: []string{"my domain.com", "localhost", "y"},
			Env:    map[string]string{"ENV": "test", "SKIP_PORT_CHECKING": "1"},
		},
		{
			Name:   "invalid_domain_http_prefix",
			Inputs: []string{"http://example.com", "localhost", "y"},
			Env:    map[string]string{"ENV": "test", "SKIP_PORT_CHECKING": "1"},
		},
		{
			Name:   "invalid_domain_https_prefix",
			Inputs: []string{"https://example.com", "localhost", "y"},
			Env:    map[string]string{"ENV": "test", "SKIP_PORT_CHECKING": "1"},
		},
		{
			Name:   "user_cancels",
			Inputs: []string{"localhost", "n"},
			Env:    map[string]string{"ENV": "test", "SKIP_PORT_CHECKING": "1"},
		},
		{
			Name:   "empty_domain_then_valid",
			Inputs: []string{"", "localhost", "y"},
			Env:    map[string]string{"ENV": "test", "SKIP_PORT_CHECKING": "1"},
		},
		{
			Name:   "subdomain_format",
			Inputs: []string{"analytics.example.com", "y"},
			Env:    map[string]string{"ENV": "test", "SKIP_PORT_CHECKING": "1"},
		},
	}
}

// NormalizeOutput removes dynamic content from output for comparison
func NormalizeOutput(output string) string {
	// Remove ANSI escape codes for spinner animations
	ansiPattern := regexp.MustCompile(`\x1b\[[0-9;]*[mK]|\r`)
	output = ansiPattern.ReplaceAllString(output, "")

	// Normalize IP addresses (replace with placeholder)
	ipPattern := regexp.MustCompile(`\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}`)
	output = ipPattern.ReplaceAllString(output, "X.X.X.X")

	// Normalize timestamps
	timePattern := regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}`)
	output = timePattern.ReplaceAllString(output, "TIMESTAMP")

	// Normalize duration patterns like "in 5s"
	durationPattern := regexp.MustCompile(`in \d+s`)
	output = durationPattern.ReplaceAllString(output, "in Xs")

	// Normalize private keys (64 char hex strings)
	keyPattern := regexp.MustCompile(`[a-f0-9]{32,64}`)
	output = keyPattern.ReplaceAllString(output, "PRIVATE_KEY")

	// Remove trailing whitespace from lines
	lines := strings.Split(output, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	output = strings.Join(lines, "\n")

	// Collapse multiple empty lines into one
	multipleNewlines := regexp.MustCompile(`\n{3,}`)
	output = multipleNewlines.ReplaceAllString(output, "\n\n")

	return strings.TrimSpace(output)
}

// RunScenario executes a scenario and captures output
func RunScenario(t *testing.T, scenario Scenario, binaryPath string) string {
	t.Helper()

	// Create stdin with inputs
	stdin := strings.Join(scenario.Inputs, "\n") + "\n"

	// Build command
	cmd := exec.Command(binaryPath, "install")
	cmd.Stdin = strings.NewReader(stdin)

	// Set environment
	cmd.Env = os.Environ()
	for k, v := range scenario.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	// Capture combined output
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	// Run (ignore exit code - we care about output)
	cmd.Run()

	return output.String()
}

// SnapshotDir returns the directory for snapshot files
func SnapshotDir() string {
	return filepath.Join("internal", "manager", "snapshot", "testdata")
}

// LoadSnapshot loads a saved snapshot
func LoadSnapshot(name string) (string, error) {
	path := filepath.Join(SnapshotDir(), name+".golden")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// SaveSnapshot saves a snapshot
func SaveSnapshot(name, content string) error {
	dir := SnapshotDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	path := filepath.Join(dir, name+".golden")
	return os.WriteFile(path, []byte(content), 0644)
}

// CompareSnapshots compares two snapshots and returns diff if different
func CompareSnapshots(expected, actual string) (bool, string) {
	if expected == actual {
		return true, ""
	}

	// Generate simple diff
	expectedLines := strings.Split(expected, "\n")
	actualLines := strings.Split(actual, "\n")

	var diff strings.Builder
	diff.WriteString("Snapshot mismatch:\n")
	diff.WriteString("==================\n")

	maxLines := len(expectedLines)
	if len(actualLines) > maxLines {
		maxLines = len(actualLines)
	}

	for i := 0; i < maxLines; i++ {
		var expLine, actLine string
		if i < len(expectedLines) {
			expLine = expectedLines[i]
		}
		if i < len(actualLines) {
			actLine = actualLines[i]
		}

		if expLine != actLine {
			diff.WriteString(fmt.Sprintf("Line %d:\n", i+1))
			diff.WriteString(fmt.Sprintf("  - %q\n", expLine))
			diff.WriteString(fmt.Sprintf("  + %q\n", actLine))
		}
	}

	return false, diff.String()
}

// TestSnapshots is the main test function that compares current output to golden files
func TestSnapshots(t *testing.T) {
	// Skip if not explicitly running snapshot tests
	if os.Getenv("RUN_SNAPSHOT_TESTS") != "1" {
		t.Skip("Skipping snapshot tests. Set RUN_SNAPSHOT_TESTS=1 to run.")
	}

	// Build the binary first
	binaryPath := filepath.Join(os.TempDir(), "fusionaly-snapshot-test")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/manager")
	buildCmd.Dir = findProjectRoot()
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to build binary: %v\n%s", err, out)
	}
	defer os.Remove(binaryPath)

	scenarios := GetScenarios()
	updateMode := os.Getenv("UPDATE_SNAPSHOTS") == "1"

	for _, scenario := range scenarios {
		t.Run(scenario.Name, func(t *testing.T) {
			output := RunScenario(t, scenario, binaryPath)
			normalized := NormalizeOutput(output)

			if updateMode {
				if err := SaveSnapshot(scenario.Name, normalized); err != nil {
					t.Fatalf("Failed to save snapshot: %v", err)
				}
				t.Logf("Updated snapshot for %s", scenario.Name)
				return
			}

			expected, err := LoadSnapshot(scenario.Name)
			if err != nil {
				t.Fatalf("No snapshot found for %s. Run with UPDATE_SNAPSHOTS=1 to create.", scenario.Name)
			}

			match, diff := CompareSnapshots(expected, normalized)
			if !match {
				t.Errorf("Snapshot mismatch for %s:\n%s", scenario.Name, diff)
			}
		})
	}
}

func findProjectRoot() string {
	// Walk up to find go.mod
	dir, _ := os.Getwd()
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "."
		}
		dir = parent
	}
}

// CaptureConfigFlow captures just the configuration/input phase output
// This is useful for comparing the interactive prompts
func CaptureConfigFlow(t *testing.T, scenario Scenario) string {
	t.Helper()

	// Use a mock that only runs the config collection
	// This is a simplified version for testing the UI
	var buf bytes.Buffer
	writer := io.MultiWriter(&buf, os.Stdout)
	_ = writer // Would need to inject into config

	return buf.String()
}
