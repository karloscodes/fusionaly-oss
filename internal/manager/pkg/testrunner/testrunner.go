package testrunner

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Environment represents where tests are running
type Environment string

const (
	// LocalEnvironment represents tests running locally
	LocalEnvironment Environment = "local"

	// CIEnvironment represents tests running in GitHub Actions
	CIEnvironment Environment = "ci"
)

// Config holds configuration for the test runner
type Config struct {
	BinaryPath     string
	Timeout        time.Duration
	StdinInput     string
	EnvVars        map[string]string
	Args           []string
	WorkingDir     string
	Debug          bool
	VMName         string   // Only used for local environment
	MultipassFlags []string // Only used for local environment
	DirectRun      bool     // If true, run directly without VM even in local environment
}

// DefaultConfig returns a Config with default values
func DefaultConfig() Config {
	return Config{
		Timeout: 10 * time.Minute, // Increased timeout to 10 minutes
		EnvVars: make(map[string]string),
		VMName:  "fusionaly-test-vm",
		MultipassFlags: []string{
			"--memory", "2G",
			"--disk", "10G",
			"--cpus", "2",
		},
		Debug: os.Getenv("DEBUG") == "1",
	}
}

// TestRunner handles running commands in different environments
type TestRunner struct {
	Config Config
	env    Environment
	stdout bytes.Buffer
	stderr bytes.Buffer
	Logger io.Writer
}

// NewTestRunner creates a new TestRunner
func NewTestRunner(config Config) *TestRunner {
	env := LocalEnvironment
	if os.Getenv("GITHUB_ACTIONS") == "true" || os.Getenv("GITHUB_RUN_NUMBER") != "" {
		env = CIEnvironment
	}

	return &TestRunner{
		Config: config,
		env:    env,
		Logger: os.Stdout,
	}
}

// Run executes the command in the appropriate environment.
// - In CI, runs the binary directly on the host (see runInCI).
// - Otherwise, runs the binary inside a fresh Multipass VM (see runInVM).
func (r *TestRunner) Run() error {
	r.logf("Starting test in %s environment", r.env)
	r.logf("Binary path: %s", r.Config.BinaryPath)
	r.logf("Environment variables: %v", r.Config.EnvVars)

	if r.env == CIEnvironment {
		return r.runInCI()
	} else {
		return r.runInVM()
	}
}

// runInCI runs the command directly in CI
func (r *TestRunner) runInCI() error {
	r.logf("Running directly in CI environment")

	cmd := exec.Command(r.Config.BinaryPath, r.Config.Args...)
	cmd.Stdin = strings.NewReader(r.Config.StdinInput)
	cmd.Stdout = io.MultiWriter(&r.stdout, r.Logger)
	cmd.Stderr = io.MultiWriter(&r.stderr, r.Logger)

	cmd.Env = os.Environ()
	for k, v := range r.Config.EnvVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	if r.Config.WorkingDir != "" {
		cmd.Dir = r.Config.WorkingDir
	}

	r.logf("Executing command: %s %s", cmd.Path, strings.Join(cmd.Args[1:], " "))

	return r.runWithTimeout(cmd)
}

// runInVM provisions a fresh Multipass VM, copies the binary, and runs the installer inside the VM.
// This simulates a real user/server environment and tests the full install process, including system-level effects.
func (r *TestRunner) runInVM() error {
	r.logf("Running in local environment with Multipass VM")

	// Check for multipass availability
	if _, err := exec.LookPath("multipass"); err != nil {
		return fmt.Errorf("multipass not found, please install it first: %w", err)
	}

	// Clean up existing VM if needed
	r.logf("Cleaning up any existing VM: %s", r.Config.VMName)
	cleanCmd := exec.Command("multipass", "delete", r.Config.VMName, "--purge")
	cleanCmd.Run() // Ignore errors

	// Prepare cloud-init with SSH key for VM access
	sshKeyPath := os.Getenv("SSH_KEY_PATH")
	if sshKeyPath == "" {
		sshKeyPath = filepath.Join(os.Getenv("HOME"), ".ssh", "id_ed25519.pub")
	}
	// If the SSH public key does not exist, generate a new Ed25519 key pair
	if _, err := os.Stat(sshKeyPath); os.IsNotExist(err) {
		privateKeyPath := strings.TrimSuffix(sshKeyPath, ".pub")
		r.logf("SSH public key not found at %s, generating new Ed25519 key pair...", sshKeyPath)
		cmd := exec.Command("ssh-keygen", "-t", "ed25519", "-f", privateKeyPath, "-N", "")
		cmd.Stdout = r.Logger
		cmd.Stderr = r.Logger
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to generate SSH key pair: %w", err)
		}
	}
	pubKeyBytes, err := os.ReadFile(sshKeyPath)
	if err != nil {
		return fmt.Errorf("failed to read SSH public key (%s): %w", sshKeyPath, err)
	}
	cloudInit := fmt.Sprintf(`#cloud-config\nusers:\n  - default\n  - name: ubuntu\n    ssh_authorized_keys:\n      - %s\n`, strings.TrimSpace(string(pubKeyBytes)))
	cloudInitPath := filepath.Join(os.TempDir(), "cloud-init.yaml")
	if err := os.WriteFile(cloudInitPath, []byte(cloudInit), 0o644); err != nil {
		return fmt.Errorf("failed to write cloud-init file: %w", err)
	}

	// Create new VM with cloud-init and any extra flags
	r.logf("Creating new VM: %s with cloud-init", r.Config.VMName)
	args := []string{"launch", "22.04", "--name", r.Config.VMName, "--cpus", "2", "--memory", "2G", "--disk", "10G", "--cloud-init", cloudInitPath}
	args = append(args, r.Config.MultipassFlags...)

	launchCmd := exec.Command("multipass", args...)
	launchOutput := &bytes.Buffer{}
	launchCmd.Stdout = io.MultiWriter(launchOutput, r.Logger)
	launchCmd.Stderr = io.MultiWriter(launchOutput, r.Logger)

	if err := launchCmd.Run(); err != nil {
		r.logf("VM launch output: %s", launchOutput.String())
		return fmt.Errorf("failed to launch VM: %w", err)
	}

	// Wait for the VM to be ready for SSH/SCP
	r.logf("Waiting for VM to be ready")
	vmReady := false
	for i := 0; i < 60; i++ {
		cmd := exec.Command("multipass", "info", r.Config.VMName)
		output, err := cmd.CombinedOutput()
		outputStr := string(output)
		r.logf("VM status check attempt %d/60: %v", i+1, err)

		if err == nil {
			r.logf("VM info output: %s", outputStr)
			if strings.Contains(outputStr, "State:") && strings.Contains(outputStr, "Running") {
				vmReady = true
				r.logf("VM is ready after %d seconds", i+1)
				break
			}
		}
		time.Sleep(1 * time.Second)
	}

	if !vmReady {
		infoCmd := exec.Command("multipass", "info", r.Config.VMName, "--format", "yaml")
		infoOutput, _ := infoCmd.CombinedOutput()
		r.logf("Detailed VM info: \n%s", string(infoOutput))

		finalCheckCmd := exec.Command("multipass", "list")
		finalOutput, _ := finalCheckCmd.CombinedOutput()
		if strings.Contains(string(finalOutput), r.Config.VMName) && strings.Contains(string(finalOutput), "Running") {
			r.logf("VM appears to be ready based on 'multipass list' output")
			vmReady = true
		} else {
			return fmt.Errorf("timeout waiting for VM to be ready")
		}
	}

	// Set ENV variable in the VM
	envSetCmd := exec.Command("multipass", "exec", r.Config.VMName, "--", "sudo", "sh", "-c", "echo 'export ENV=test' >> /etc/environment")
	envSetCmd.Run()

	// Copy binary to VM
	r.logf("Copying binary to VM")
	copyCmd := exec.Command("multipass", "transfer", r.Config.BinaryPath, fmt.Sprintf("%s:/home/ubuntu/fusionaly", r.Config.VMName))
	copyCmd.Stdout = r.Logger
	copyCmd.Stderr = r.Logger

	if err := copyCmd.Run(); err != nil {
		return fmt.Errorf("failed to copy binary to VM: %w", err)
	}

	// Make binary executable
	r.logf("Making binary executable")
	execCmd := exec.Command("multipass", "exec", r.Config.VMName, "--", "chmod", "+x", "/home/ubuntu/fusionaly")
	execCmd.Stdout = r.Logger
	execCmd.Stderr = r.Logger

	if err := execCmd.Run(); err != nil {
		return fmt.Errorf("failed to make binary executable: %w", err)
	}

	r.logf("Moving binary to system location")
	moveCmd := exec.Command("multipass", "exec", r.Config.VMName, "--", "sudo", "mv", "/home/ubuntu/fusionaly", "/usr/local/bin/fusionaly")
	moveCmd.Stdout = r.Logger
	moveCmd.Stderr = r.Logger

	if err := moveCmd.Run(); err != nil {
		return fmt.Errorf("failed to move binary: %w", err)
	}

	// Set environment variables in the VM before running the command
	for k, v := range r.Config.EnvVars {
		r.logf("Setting environment variable in VM: %s=%s", k, v)
		envCmd := exec.Command("multipass", "exec", r.Config.VMName, "--", "sudo", "sh", "-c", fmt.Sprintf("echo 'export %s=%s' >> /etc/environment", k, v))
		envOutput, err := envCmd.CombinedOutput()
		if err != nil {
			r.logf("Failed to set environment variable %s: %v\nOutput: %s", k, err, string(envOutput))
		}
	}

	// Create the command to run with stdin input
	cmdParts := []string{"exec", r.Config.VMName, "--", "sudo", "/usr/local/bin/fusionaly"}
	cmdParts = append(cmdParts, r.Config.Args...)

	cmd := exec.Command("multipass", cmdParts...)
	cmd.Stdin = strings.NewReader(r.Config.StdinInput)
	cmd.Stdout = io.MultiWriter(&r.stdout, r.Logger)
	cmd.Stderr = io.MultiWriter(&r.stderr, r.Logger)

	cmd.Env = os.Environ()
	for k, v := range r.Config.EnvVars {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}

	err = r.runWithTimeout(cmd)

	if os.Getenv("KEEP_VM") != "1" {
		r.logf("Cleaning up VM: %s", r.Config.VMName)
		cleanupCmd := exec.Command("multipass", "delete", r.Config.VMName, "--purge")
		cleanupCmd.Run() // Ignore errors
	} else {
		r.logf("Keeping VM for inspection: %s", r.Config.VMName)
	}

	return err
}

// GetVMIP returns the IPv4 address of the Multipass VM
func (r *TestRunner) GetVMIP() (string, error) {
	cmd := exec.Command("multipass", "info", r.Config.VMName)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.Contains(line, "IPv4") {
			parts := strings.Fields(line)
			if len(parts) > 1 {
				return parts[1], nil
			}
		}
	}
	return "", fmt.Errorf("IP not found for VM %s", r.Config.VMName)
}

// RunMultipassCommand runs a shell command in the VM via multipass exec
func (r *TestRunner) RunMultipassCommand(command string, useSudo bool) (string, error) {
	args := []string{"exec", r.Config.VMName, "--"}
	if useSudo {
		args = append(args, "sudo")
	}
	args = append(args, "sh", "-c", command)
	cmd := exec.Command("multipass", args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// CopyFileToVM copies a file to the VM using multipass transfer
func (r *TestRunner) CopyFileToVM(localPath, remotePath string) error {
	cmd := exec.Command("multipass", "transfer", localPath, fmt.Sprintf("%s:%s", r.Config.VMName, remotePath))
	return cmd.Run()
}

// Deprecated: Use CopyFileToVM for VM file copy in local mode
func (r *TestRunner) CopyFileToVMOverSSH(localPath, remotePath string) error {
	return fmt.Errorf("CopyFileToVMOverSSH is deprecated; use CopyFileToVM with multipass instead")
}

// Deprecated: Use RunMultipassCommand for VM command execution in local mode
func (r *TestRunner) RunSSHCommand(command string) (string, error) {
	return "", fmt.Errorf("RunSSHCommand is deprecated; use RunMultipassCommand with multipass instead")
}

// CheckServiceAvailability checks if the service is available at the given URL.
// In VM mode, it uses multipass exec to curl from inside the VM. In direct mode, it curls locally.
func (r *TestRunner) CheckServiceAvailability(url string, attempts int, t interface {
	Logf(string, ...interface{})
	Errorf(string, ...interface{})
},
) (success bool, is302 bool, finalOutput string) {
	for i := 0; i < attempts; i++ {
		var output string
		var err error
		if r.env == LocalEnvironment && !r.Config.DirectRun {
			// VM mode: curl via multipass exec
			cmd := exec.Command("multipass", "exec", r.Config.VMName, "--", "curl", "-k", "-s", "-o", "/dev/null", "-w", "%{http_code}", url)
			out, e := cmd.CombinedOutput()
			output = strings.TrimSpace(string(out))
			err = e
		} else {
			// Direct mode: curl locally
			cmd := exec.Command("curl", "-k", "-s", "-o", "/dev/null", "-w", "%{http_code}", url)
			out, e := cmd.CombinedOutput()
			output = strings.TrimSpace(string(out))
			err = e
		}
		outputStr := strings.TrimSpace(output)
		finalOutput = outputStr
		t.Logf("Service check attempt %d/%d, result: %s, error: %v", i+1, attempts, outputStr, err)
		if err == nil && outputStr == "302" {
			success = true
			is302 = true
			break
		} else if err == nil && outputStr == "200" {
			success = true
		}
		time.Sleep(5 * time.Second)
	}
	return
}

// runWithTimeout runs a command with the configured timeout
func (r *TestRunner) runWithTimeout(cmd *exec.Cmd) error {
	err := cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(r.Config.Timeout):
		cmd.Process.Kill()
		return fmt.Errorf("command timed out after %v", r.Config.Timeout)
	}
}

// Stdout returns the stdout output
func (r *TestRunner) Stdout() string {
	return r.stdout.String()
}

// Stderr returns the stderr output
func (r *TestRunner) Stderr() string {
	return r.stderr.String()
}

// logf logs a message to the logger if debug is enabled
func (r *TestRunner) logf(format string, args ...interface{}) {
	if r.Config.Debug {
		fmt.Fprintf(r.Logger, "[TestRunner] "+format+"\n", args...)
	}
}
