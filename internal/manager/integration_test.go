// Package manager provides integration tests for fusionaly-manager.
// These tests run the manager binary directly to verify basic functionality.
package manager_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestManagerVersion verifies the version command works correctly.
func TestManagerVersion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	binaryPath := getBinaryPath(t)

	cmd := exec.Command(binaryPath, "version")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	require.NoError(t, err, "Version command should succeed")

	output := stdout.String()
	assert.NotEmpty(t, output, "Version output should not be empty")
	t.Logf("Version output: %s", strings.TrimSpace(output))
}

// TestManagerHelp verifies the help command works correctly.
func TestManagerHelp(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	binaryPath := getBinaryPath(t)

	cmd := exec.Command(binaryPath, "help")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	require.NoError(t, err, "Help command should succeed")

	output := stdout.String()
	assert.Contains(t, output, "install", "Help should mention install command")
	assert.Contains(t, output, "update", "Help should mention update command")
	assert.Contains(t, output, "version", "Help should mention version command")
	t.Logf("Help output:\n%s", output)
}

// TestManagerInvalidCommand verifies invalid commands are handled gracefully.
func TestManagerInvalidCommand(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	binaryPath := getBinaryPath(t)

	cmd := exec.Command(binaryPath, "nonexistent-command")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Should fail with non-zero exit code
	assert.Error(t, err, "Invalid command should return error")

	output := stdout.String()
	assert.Contains(t, output, "Unknown command", "Output should mention unknown command")
}

// TestManagerNoArgs verifies running without arguments shows usage.
func TestManagerNoArgs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	binaryPath := getBinaryPath(t)

	cmd := exec.Command(binaryPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Should fail with non-zero exit code
	assert.Error(t, err, "Running without args should return error")

	output := stdout.String()
	assert.Contains(t, output, "Usage:", "Output should show usage")
}

// getBinaryPath returns the path to the manager binary, building it if necessary.
func getBinaryPath(t *testing.T) string {
	t.Helper()

	projectRoot := findProjectRoot(t)
	binaryPath := filepath.Join(projectRoot, "tmp", "fusionaly-manager")

	// Always rebuild to ensure we're testing current code
	t.Log("Building manager binary...")
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "./cmd/manager")
	buildCmd.Dir = projectRoot
	output, err := buildCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to build manager binary: %v\nOutput: %s", err, output)
	}

	t.Logf("Built binary at: %s", binaryPath)
	return binaryPath
}

// findProjectRoot finds the fusionaly project root directory.
func findProjectRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	require.NoError(t, err)

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			content, err := os.ReadFile(filepath.Join(dir, "go.mod"))
			if err == nil && strings.Contains(string(content), "module fusionaly") {
				return dir
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("Could not find project root")
		}
		dir = parent
	}
}
