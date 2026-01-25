package docker

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"fusionaly/internal/manager/config"
	"fusionaly/internal/manager/database"
	"fusionaly/internal/manager/errors"
	"fusionaly/internal/manager/logging"
)

const (
	NetworkName      = "fusionaly-network"
	CaddyName        = "fusionaly-caddy"
	AppNamePrimary   = "fusionaly-app-1"
	AppNameSecondary = "fusionaly-app-2"
	MaxRetries       = 3
	HealthCheckTries = 5
)

//go:embed templates/Caddyfile.tmpl
var caddyfileTemplate string

type Docker struct {
	logger *logging.Logger
	db     *database.Database
}

func NewDocker(logger *logging.Logger, db *database.Database) *Docker {
	return &Docker{
		logger: logger,
		db:     db,
	}
}

func (d *Docker) RunCommand(args ...string) (string, error) {
	if len(args) == 0 {
		return "", errors.NewDockerError("", "", fmt.Errorf("no docker command provided"))
	}
	
	d.logger.Debug("Running docker %s", strings.Join(args, " "))
	var stdout, stderr bytes.Buffer
	cmd := exec.Command("docker", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	
	if err := cmd.Run(); err != nil {
		return "", errors.NewDockerError(args[0], "", fmt.Errorf("%w - %s", err, stderr.String()))
	}
	return stdout.String(), nil
}

func (d *Docker) EnsureInstalled() error {
	if version, err := d.RunCommand("version"); err == nil {
		d.logger.Success("Docker is installed (version: %s)", strings.TrimSpace(strings.Split(version, "\n")[0]))
		return nil
	}

	d.logger.Info("Docker not found, installing...")
	output, err := exec.Command("bash", "-c", "curl -fsSL https://get.docker.com | sh").CombinedOutput()
	if err != nil {
		d.logger.Error("Docker installation failed: %s", string(output))
		return fmt.Errorf("install failed: %w", err)
	}
	d.logger.Success("Docker installed successfully")

	for _, cmd := range [][]string{
		{"systemctl", "start", "docker"},
		{"systemctl", "enable", "docker"},
	} {
		if err := exec.Command(cmd[0], cmd[1:]...).Run(); err != nil {
			return fmt.Errorf("%s failed: %w", cmd[1], err)
		}
	}

	version, err := d.RunCommand("version")
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}
	d.logger.InfoWithTime("Docker version: %s", strings.TrimSpace(strings.Split(version, "\n")[0]))
	return nil
}

func (d *Docker) Deploy(conf *config.Config) error {
	data := conf.GetData()
	dataDir := data.InstallDir

	if d.IsRunning(CaddyName) && (d.IsRunning(AppNamePrimary) || d.IsRunning(AppNameSecondary)) {
		return nil
	}

	for _, dir := range []string{
		filepath.Join(dataDir, "storage"),
		filepath.Join(dataDir, "logs"),
		filepath.Join(dataDir, "caddy"),
		filepath.Join(dataDir, "caddy", "config"),
		filepath.Join(dataDir, "storage", "backups"),
	} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create dir %s: %w", dir, err)
		}
	}

	if _, err := d.RunCommand("network", "inspect", NetworkName); err != nil {
		if _, err := d.RunCommand("network", "create", NetworkName); err != nil {
			return fmt.Errorf("create network: %w", err)
		}
	}

	caddyFile := filepath.Join(dataDir, "Caddyfile")
	caddyContent, err := d.generateCaddyfile(data)
	if err != nil {
		return fmt.Errorf("generate Caddyfile: %w", err)
	}
	if err := os.WriteFile(caddyFile, []byte(caddyContent), 0o644); err != nil {
		return fmt.Errorf("write Caddyfile: %w", err)
	}

	for _, image := range []string{data.AppImage, data.CaddyImage} {
		for i := 0; i < MaxRetries; i++ {
			if _, err := d.RunCommand("pull", image); err == nil {
				d.logImageDigest(image)
				break
			} else if i == MaxRetries-1 {
				return fmt.Errorf("pull %s failed after %d retries: %w", image, MaxRetries, err)
			}
			d.logger.Warn("Pull %s failed, retrying (%d/%d)", image, i+1, MaxRetries)
			time.Sleep(time.Duration(i+1) * 2 * time.Second)
		}
	}

	// Deploy app first
	if err := d.DeployApp(data, AppNamePrimary); err != nil {
		d.logger.Error("Initial app deployment failed, running diagnostics...")
		if d.containerExists(AppNamePrimary) {
			d.DiagnoseContainerStartup(AppNamePrimary)
		}
		return fmt.Errorf("initial app deploy failed: %w", err)
	}

	if err := d.waitForAppHealth(AppNamePrimary); err != nil {
		d.logger.Error("Initial app health check failed, running diagnostics...")
		d.DiagnoseContainerStartup(AppNamePrimary)
		if cleanupErr := d.StopAndRemove(AppNamePrimary); cleanupErr != nil {
			d.logger.Error("Failed to cleanup unhealthy container %s: %v", AppNamePrimary, cleanupErr)
		}
		return errors.NewDockerError("health_check", AppNamePrimary, err)
	}

	if !d.IsRunning(CaddyName) {
		if err := d.deployCaddy(data, caddyFile); err != nil {
			return fmt.Errorf("deploy caddy: %w", err)
		}
	} else {
		if err := d.ensureNetworkConnected(CaddyName, NetworkName); err != nil {
			return fmt.Errorf("failed to ensure network for %s: %w", CaddyName, err)
		}
	}

	d.logCaddyVersion()
	return nil
}

func (d *Docker) Update(conf *config.Config) error {
	data := conf.GetData()
	dataDir := data.InstallDir

	if _, err := d.RunCommand("network", "inspect", NetworkName); err != nil {
		d.logger.Info("Creating Docker network %s", NetworkName)
		if _, err := d.RunCommand("network", "create", NetworkName); err != nil {
			return fmt.Errorf("create network: %w", err)
		}
		d.logger.Success("Network created")
	}

	// Pull new images using the unified DockerImages struct
	dockerImages := conf.GetDockerImages()
	for _, image := range []string{dockerImages.AppImage, dockerImages.CaddyImage} {
		// Check if we need to pull the image
		shouldPull, err := d.ShouldPullImage(image)
		if err != nil {
			d.logger.Warn("Error checking image status for %s: %v, will attempt to pull", image, err)
			shouldPull = true
		}

		if shouldPull {
			d.logger.Info("Pulling %s...", image)
			for i := 0; i < MaxRetries; i++ {
				if _, err := d.RunCommand("pull", image); err == nil {
					d.logger.Success("%s pulled successfully", image)
					d.logImageDigest(image)
					break
				} else if i == MaxRetries-1 {
					return fmt.Errorf("pull %s failed after %d retries: %w", image, MaxRetries, err)
				}
				d.logger.Warn("Pull %s failed, retrying (%d/%d)", image, i+1, MaxRetries)
				time.Sleep(time.Duration(i+1) * 2 * time.Second)
			}
		} else {
			d.logger.Success("Image %s is already up to date, skipping pull", image)
			// Still log the digest for consistency in logs
			d.logImageDigest(image)
		}
	}

	// Determine current and new app instances
	currentName := AppNamePrimary
	newName := AppNameSecondary
	if d.IsRunning(AppNameSecondary) && !d.IsRunning(AppNamePrimary) {
		currentName, newName = AppNameSecondary, AppNamePrimary
	}

	// Deploy the new app instance
	for i := 0; i < MaxRetries; i++ {
		if err := d.DeployApp(data, newName); err == nil {
			d.logger.Success("%s deployed", newName)
			break
		} else if i == MaxRetries-1 {
			d.logger.Error("Failed to deploy %s after %d retries", newName, MaxRetries)
			// If the container was created but failed to start properly, run comprehensive diagnostics
			if d.containerExists(newName) {
				d.DiagnoseContainerStartup(newName)
			} else {
				d.logger.Error("Container %s was never created - deployment failed at creation stage", newName)
			}
			if cleanupErr := d.StopAndRemove(newName); cleanupErr != nil {
				d.logger.Error("Failed to cleanup failed container %s: %v", newName, cleanupErr)
			}
			return errors.NewDockerError("deploy", newName, fmt.Errorf("failed after %d retries: %w", MaxRetries, err))
		}
		d.logger.Warn("Deploy %s failed, retrying (%d/%d)", newName, i+1, MaxRetries)
		if cleanupErr := d.StopAndRemove(newName); cleanupErr != nil {
			d.logger.Error("Failed to cleanup container %s before retry: %v", newName, cleanupErr)
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	if err := d.ensureNetworkConnected(newName, NetworkName); err != nil {
		if cleanupErr := d.StopAndRemove(newName); cleanupErr != nil {
			d.logger.Error("Failed to cleanup container %s after network error: %v", newName, cleanupErr)
		}
		return errors.NewDockerError("network_connect", newName, err)
	}

	if err := d.waitForAppHealth(newName); err != nil {
		d.logger.Error("Health check failed for %s, running diagnostics...", newName)
		d.DiagnoseContainerStartup(newName)
		if cleanupErr := d.StopAndRemove(newName); cleanupErr != nil {
			d.logger.Error("Failed to cleanup unhealthy container %s: %v", newName, cleanupErr)
		}
		return errors.NewDockerError("health_check", newName, err)
	}

	// Switch Caddy to point to the new container
	d.logger.Info("Switching Caddy to point to new container %s...", newName)
	caddyFile := filepath.Join(dataDir, "Caddyfile")
	caddyContent, err := d.generateCaddyfileForContainer(data, newName)
	if err != nil {
		return fmt.Errorf("generate Caddyfile: %w", err)
	}
	if err := os.WriteFile(caddyFile, []byte(caddyContent), 0o644); err != nil {
		return fmt.Errorf("write Caddyfile: %w", err)
	}
	d.logger.Info("Reloading Caddy configuration to point to %s...", newName)
	if _, err := d.RunCommand("exec", CaddyName, "caddy", "reload", "--config", "/etc/caddy/Caddyfile"); err != nil {
		d.logger.Warn("Caddy reload failed: %v. Attempting full Caddy redeploy as a fallback.", err)
		// Fallback to stop and redeploy if reload fails
		if cleanupErr := d.StopAndRemove(CaddyName); cleanupErr != nil {
			d.logger.Error("Failed to cleanup Caddy container during fallback: %v", cleanupErr)
		}
		if errRedeploy := d.deployCaddy(data, caddyFile); errRedeploy != nil {
			return fmt.Errorf("caddy reload failed and subsequent redeploy also failed: %w (reload error: %v)", errRedeploy, err)
		}
		d.logger.Info("Caddy successfully redeployed as a fallback.")
	} else {
		d.logger.Success("Caddy configuration reloaded successfully")
	}

	d.logCaddyVersion()
	d.logContainerImage(newName)

	// Clean up old app instance
	if cleanupErr := d.StopAndRemove(currentName); cleanupErr != nil {
		d.logger.Error("Failed to cleanup old container %s: %v", currentName, cleanupErr)
	}
	if _, err := d.RunCommand("image", "prune", "-f"); err != nil {
		d.logger.Warn("Failed to prune unused images: %v", err)
	}

	return nil
}

func (d *Docker) UpdateWithDebug(conf *config.Config) error {
	d.logger.Debug("Starting Docker update with debug logging")
	data := conf.GetData()
	dataDir := data.InstallDir

	d.logger.Debug("Install directory: %s", dataDir)

	if _, err := d.RunCommand("network", "inspect", NetworkName); err != nil {
		d.logger.Info("Creating Docker network %s", NetworkName)
		if _, err := d.RunCommand("network", "create", NetworkName); err != nil {
			return fmt.Errorf("create network: %w", err)
		}
		d.logger.Success("Network created")
	} else {
		d.logger.Debug("Docker network %s already exists", NetworkName)
	}

	// Show current running containers
	d.logger.Debug("Checking current container status...")
	d.ShowContainerStatus()

	// Pull new images using the unified DockerImages struct
	dockerImages := conf.GetDockerImages()
	d.logger.Debug("Images to check: App=%s, Caddy=%s", dockerImages.AppImage, dockerImages.CaddyImage)
	
	for _, image := range []string{dockerImages.AppImage, dockerImages.CaddyImage} {
		// Check if we need to pull the image
		shouldPull, err := d.ShouldPullImage(image)
		if err != nil {
			d.logger.Warn("Error checking image status for %s: %v, will attempt to pull", image, err)
			shouldPull = true
		}

		d.logger.Debug("Image %s should pull: %v", image, shouldPull)

		if shouldPull {
			d.logger.Info("Pulling %s...", image)
			for i := 0; i < MaxRetries; i++ {
				d.logger.Debug("Pull attempt %d/%d for %s", i+1, MaxRetries, image)
				if _, err := d.RunCommand("pull", image); err == nil {
					d.logger.Success("%s pulled successfully", image)
					d.logImageDigest(image)
					break
				} else if i == MaxRetries-1 {
					d.logger.Error("Pull failed after %d retries: %v", MaxRetries, err)
					return fmt.Errorf("pull %s failed after %d retries: %w", image, MaxRetries, err)
				}
				d.logger.Warn("Pull %s failed, retrying (%d/%d): %v", image, i+1, MaxRetries, err)
				time.Sleep(time.Duration(i+1) * 2 * time.Second)
			}
		} else {
			d.logger.Success("Image %s is already up to date, skipping pull", image)
			// Still log the digest for consistency in logs
			d.logImageDigest(image)
		}
	}

	// Determine current and new app instances
	currentName := AppNamePrimary
	newName := AppNameSecondary
	if d.IsRunning(AppNameSecondary) && !d.IsRunning(AppNamePrimary) {
		currentName, newName = AppNameSecondary, AppNamePrimary
	}

	d.logger.Debug("Current container: %s, New container: %s", currentName, newName)

	// Deploy the new app instance
	for i := 0; i < MaxRetries; i++ {
		d.logger.Debug("Deploying new container %s (attempt %d/%d)", newName, i+1, MaxRetries)
		if err := d.DeployApp(data, newName); err == nil {
			d.logger.Success("%s deployed successfully", newName)
			break
		} else if i == MaxRetries-1 {
			d.logger.Error("Failed to deploy %s after %d retries", newName, MaxRetries)
			// If the container was created but failed to start properly, try to get logs
			if d.containerExists(newName) {
				d.logger.Debug("Container %s exists but failed, fetching logs...", newName)
				d.ShowContainerLogs(newName, 100) // Show more logs in debug mode
			}
			if cleanupErr := d.StopAndRemove(newName); cleanupErr != nil {
				d.logger.Error("Failed to cleanup failed container %s: %v", newName, cleanupErr)
			}
			return errors.NewDockerError("deploy", newName, fmt.Errorf("failed after %d retries: %w", MaxRetries, err))
		}
		d.logger.Warn("Deploy %s failed, retrying (%d/%d)", newName, i+1, MaxRetries)
		if cleanupErr := d.StopAndRemove(newName); cleanupErr != nil {
			d.logger.Error("Failed to cleanup container %s before retry: %v", newName, cleanupErr)
		}
		time.Sleep(time.Duration(i+1) * time.Second)
	}

	d.logger.Debug("Ensuring network connectivity for %s", newName)
	if err := d.ensureNetworkConnected(newName, NetworkName); err != nil {
		if cleanupErr := d.StopAndRemove(newName); cleanupErr != nil {
			d.logger.Error("Failed to cleanup container %s after network error: %v", newName, cleanupErr)
		}
		return errors.NewDockerError("network_connect", newName, err)
	}

	d.logger.Debug("Waiting for %s to become healthy...", newName)
	if err := d.waitForAppHealth(newName); err != nil {
		d.logger.Debug("Health check failed for %s, getting detailed logs...", newName)
		d.ShowContainerLogs(newName, 100)
		if cleanupErr := d.StopAndRemove(newName); cleanupErr != nil {
			d.logger.Error("Failed to cleanup unhealthy container %s: %v", newName, cleanupErr)
		}
		return errors.NewDockerError("health_check", newName, err)
	}

	// Switch Caddy to point to the new container
	d.logger.Info("Switching Caddy to point to new container %s...", newName)
	d.logger.Debug("Generating new Caddyfile for container %s", newName)
	caddyFile := filepath.Join(dataDir, "Caddyfile")
	caddyContent, err := d.generateCaddyfileForContainer(data, newName)
	if err != nil {
		return fmt.Errorf("generate Caddyfile: %w", err)
	}
	if err := os.WriteFile(caddyFile, []byte(caddyContent), 0o644); err != nil {
		return fmt.Errorf("write Caddyfile: %w", err)
	}
	d.logger.Debug("Caddyfile written, reloading Caddy configuration...")
	d.logger.Info("Reloading Caddy configuration to point to %s...", newName)
	if _, err := d.RunCommand("exec", CaddyName, "caddy", "reload", "--config", "/etc/caddy/Caddyfile"); err != nil {
		d.logger.Warn("Caddy reload failed: %v. Attempting full Caddy redeploy as a fallback.", err)
		// Show Caddy logs before fallback
		d.logger.Debug("Showing Caddy logs before redeploy:")
		d.ShowContainerLogs(CaddyName, 50)
		
		// Fallback to stop and redeploy if reload fails
		if cleanupErr := d.StopAndRemove(CaddyName); cleanupErr != nil {
			d.logger.Error("Failed to cleanup Caddy container during fallback: %v", cleanupErr)
		}
		if errRedeploy := d.deployCaddy(data, caddyFile); errRedeploy != nil {
			return fmt.Errorf("caddy reload failed and subsequent redeploy also failed: %w (reload error: %v)", errRedeploy, err)
		}
		d.logger.Info("Caddy successfully redeployed as a fallback.")
	} else {
		d.logger.Success("Caddy configuration reloaded successfully")
	}

	d.logCaddyVersion()
	d.logContainerImage(newName)

	// Clean up old app instance
	d.logger.Debug("Cleaning up old container: %s", currentName)
	if cleanupErr := d.StopAndRemove(currentName); cleanupErr != nil {
		d.logger.Error("Failed to cleanup old container %s: %v", currentName, cleanupErr)
	} else {
		d.logger.Debug("Old container %s cleaned up successfully", currentName)
	}
	
	d.logger.Debug("Pruning unused Docker images...")
	if _, err := d.RunCommand("image", "prune", "-f"); err != nil {
		d.logger.Warn("Failed to prune unused images: %v", err)
	} else {
		d.logger.Debug("Unused images pruned successfully")
	}

	d.logger.Debug("Showing final container status:")
	d.ShowContainerStatus()

	return nil
}

func (d *Docker) Reload(conf *config.Config) error {
	data := conf.GetData()
	dataDir := data.InstallDir

	d.logger.Info("Starting container reload with latest environment variables")

	// Ensure network exists
	if _, err := d.RunCommand("network", "inspect", NetworkName); err != nil {
		d.logger.Info("Creating Docker network %s", NetworkName)
		if _, err := d.RunCommand("network", "create", NetworkName); err != nil {
			return fmt.Errorf("create network: %w", err)
		}
		d.logger.Success("Network created")
	}

	// Find which app container is running
	currentName := ""
	if d.IsRunning(AppNamePrimary) {
		currentName = AppNamePrimary
	} else if d.IsRunning(AppNameSecondary) {
		currentName = AppNameSecondary
	} else {
		d.logger.Warn("No app container running, will deploy primary")
		currentName = AppNamePrimary
	}

	d.logger.Info("Restarting app container: %s", currentName)
	if cleanupErr := d.StopAndRemove(currentName); cleanupErr != nil {
		d.logger.Error("Failed to stop current container %s: %v", currentName, cleanupErr)
	}

	// Deploy the app container
	if err := d.DeployApp(data, currentName); err != nil {
		return fmt.Errorf("failed to redeploy app container %s: %w", currentName, err)
	}

	if err := d.waitForAppHealth(currentName); err != nil {
		if cleanupErr := d.StopAndRemove(currentName); cleanupErr != nil {
			d.logger.Error("Failed to cleanup unhealthy container %s: %v", currentName, cleanupErr)
		}
		return errors.NewDockerError("health_check", currentName, err)
	}

	// Restart Caddy container
	if d.IsRunning(CaddyName) {
		d.logger.Info("Restarting Caddy container")

		caddyFile := filepath.Join(dataDir, "Caddyfile")
		caddyContent, err := d.generateCaddyfile(data)
		if err != nil {
			return fmt.Errorf("generate Caddyfile: %w", err)
		}

		// Write the Caddyfile
		if err := os.WriteFile(caddyFile, []byte(caddyContent), 0o644); err != nil {
			return fmt.Errorf("write Caddyfile: %w", err)
		}

		// Reload Caddy
		d.logger.Info("Reloading Caddy configuration with new environment variables...")
		if _, err := d.RunCommand("exec", CaddyName, "caddy", "reload", "--config", "/etc/caddy/Caddyfile"); err != nil {
			d.logger.Warn("Caddy reload failed: %v. Attempting full Caddy redeploy as a fallback.", err)
			// Fallback to stop and redeploy if reload fails
			if cleanupErr := d.StopAndRemove(CaddyName); cleanupErr != nil {
				d.logger.Error("Failed to cleanup Caddy container during fallback: %v", cleanupErr)
			}
			if errRedeploy := d.deployCaddy(data, caddyFile); errRedeploy != nil {
				return fmt.Errorf("caddy reload failed and subsequent redeploy also failed: %w (reload error: %v)", errRedeploy, err)
			}
			d.logger.Info("Caddy successfully redeployed as a fallback.")
		} else {
			d.logger.Success("Caddy configuration reloaded successfully")
		}
	}

	d.logger.Success("Containers reloaded successfully with new environment variables")
	return nil
}

func (d *Docker) deployCaddy(data config.ConfigData, caddyFile string) error {
	if cleanupErr := d.StopAndRemove(CaddyName); cleanupErr != nil {
		// Only log if it's not a "no such container" error
		if !strings.Contains(cleanupErr.Error(), "No such container") {
			d.logger.Warn("Failed to cleanup existing Caddy container: %v", cleanupErr)
		}
	}
	_, err := d.RunCommand("run", "-d",
		"--name", CaddyName,
		"--network", NetworkName,
		"--pull", "always",
		"-p", "80:80", "-p", "443:443", "-p", "443:443/udp",
		"-v", caddyFile+":/etc/caddy/Caddyfile:ro",
		"-v", filepath.Join(data.InstallDir, "caddy")+":/data",
		"-v", filepath.Join(data.InstallDir, "caddy", "config")+":/config",
		"-v", filepath.Join(data.InstallDir, "logs")+":/data/logs",
		"-e", "DOMAIN="+data.Domain,
		"--memory=256m",
		"--restart", "unless-stopped",
		data.CaddyImage,
	)
	if err != nil {
		return fmt.Errorf("start caddy: %w", err)
	}
	_, err = d.RunCommand("exec", CaddyName, "chmod", "-R", "755", "/data")
	if err != nil {
		return fmt.Errorf("failed to set permissions on /data directory in %s container: %w", CaddyName, err)
	}
	return nil
}

func (d *Docker) DeployApp(data config.ConfigData, name string) error {
	if cleanupErr := d.StopAndRemove(name); cleanupErr != nil {
		// Only log if it's not a "no such container" error
		if !strings.Contains(cleanupErr.Error(), "No such container") {
			d.logger.Warn("Failed to cleanup existing container %s: %v", name, cleanupErr)
		}
	}
	args := []string{"run", "-d",
		"--name", name,
		"--network", NetworkName,
		"--pull", "always",
		"-v", filepath.Join(data.InstallDir, "storage") + ":/app/storage",
		"-v", filepath.Join(data.InstallDir, "logs") + ":/app/logs",
		"-e", "FUSIONALY_LOG_LEVEL=info",
		"-e", "FUSIONALY_APP_PORT=8080",
		"-e", "FUSIONALY_DOMAIN=" + data.Domain,
		"-e", "FUSIONALY_PRIVATE_KEY=" + data.PrivateKey,
		"-e", "SERVER_INSTANCE_ID=" + name,
		"-e", "FUSIONALY_LICENSE_KEY=" + data.LicenseKey,
		"--memory=512m",
		"--restart", "unless-stopped",
		data.AppImage,
	}
	
	_, err := d.RunCommand(args...)
	if err != nil {
		return fmt.Errorf("deploy %s: %w", name, err)
	}
	return nil
}

func (d *Docker) StopAndRemove(name string) error {
	if name == "" {
		return errors.NewDockerError("stop_and_remove", name, fmt.Errorf("container name cannot be empty"))
	}
	
	var stopErr, removeErr error
	
	// Attempt to stop the container
	if _, err := d.RunCommand("stop", name); err != nil {
		// Only warn if it's not a "no such container" error
		if !strings.Contains(err.Error(), "No such container") {
			d.logger.Warn("Failed to stop container %s: %v", name, err)
		}
		stopErr = err
	}
	
	// Attempt to remove the container
	if _, err := d.RunCommand("rm", "-f", name); err != nil {
		// Only warn if it's not a "no such container" error
		if !strings.Contains(err.Error(), "No such container") {
			d.logger.Warn("Failed to remove container %s: %v", name, err)
		}
		removeErr = err
	}
	
	// Return error if remove failed (more critical than stop failure)
	if removeErr != nil {
		return errors.NewDockerError("remove", name, removeErr)
	}
	if stopErr != nil {
		return errors.NewDockerError("stop", name, stopErr)
	}
	
	return nil
}

func (d *Docker) IsRunning(name string) bool {
	out, err := d.RunCommand("ps", "-q", "-f", "name="+name)
	return err == nil && strings.TrimSpace(out) != ""
}

func (d *Docker) ExecuteCommand(command ...string) error {
	containerName := AppNamePrimary
	if !d.IsRunning(containerName) {
		containerName = AppNameSecondary
		if !d.IsRunning(containerName) {
			return fmt.Errorf("no running app container found")
		}
	}

	args := []string{"exec", containerName}
	args = append(args, command...)

	d.logger.Debug("Executing in app container %s: %s", containerName, strings.Join(command, " "))

	var stdout, stderr bytes.Buffer
	cmd := exec.Command("docker", args...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to execute in container %s: %w - %s", containerName, err, stderr.String())
	}

	if stdout.Len() > 0 {
		d.logger.Debug("Command output: %s", stdout.String())
	}

	return nil
}

func (d *Docker) ensureNetworkConnected(container, network string) error {
	output, err := d.RunCommand("network", "inspect", network, "--format", "{{range .Containers}}{{.Name}}{{end}}")
	if err != nil {
		d.logger.Error("Failed to inspect network %s: %v", network, err)
		return fmt.Errorf("failed to inspect network %s: %w", network, err)
	}

	if strings.Contains(output, container) {
		d.logger.Info("Container %s is already connected to network %s", container, network)
		return nil
	}

	d.logger.Info("Connecting container %s to network %s...", container, network)
	_, err = d.RunCommand("network", "connect", network, container)
	if err != nil {
		d.logger.Error("Failed to connect container %s to network %s. Error: %v", container, network, err)

		// Check container status to provide more context
		if d.containerExists(container) {
			status, statusErr := d.RunCommand("inspect", "--format", "{{.State.Status}}", container)
			if statusErr == nil {
				d.logger.Info("Container %s current status: %s", container, strings.TrimSpace(status))
			}
		} else {
			d.logger.Error("Container %s no longer exists, which may be why network connection failed", container)
		}

		return fmt.Errorf("failed to connect container %s to network %s: %w", container, network, err)
	}
	d.logger.Success("Container %s connected to network %s", container, network)
	return nil
}

func (d *Docker) generateCaddyfile(data config.ConfigData) (string, error) {
	// Determine which container is currently active
	activeContainer := d.getActiveContainer()
	return d.generateCaddyfileForContainer(data, activeContainer)
}

// generateCaddyfileForContainer generates Caddyfile for a specific container
func (d *Docker) generateCaddyfileForContainer(data config.ConfigData, containerName string) (string, error) {
	env := os.Getenv("ENV")
	var tlsConfig string
	if env == "test" {
		d.logger.Info("Using self-signed certificate for test environment")
		tlsConfig = "internal"
	} else {
		d.logger.Info("Using Let's Encrypt for production environment")
		// Use database user email if available, otherwise generate admin email for Let's Encrypt
		if data.User != "" {
			d.logger.Info("Using database admin user email for Let's Encrypt: %s", data.User)
			tlsConfig = data.User
		} else {
			d.logger.Info("No database user found, generating admin email for Let's Encrypt")
			tlsConfig = generateAdminEmail(data.Domain)
		}
	}

	tplData := struct {
		Domain          string
		TLSConfig       string
		ActiveContainer string
	}{
		Domain:          data.Domain,
		TLSConfig:       tlsConfig,
		ActiveContainer: containerName,
	}

	tmpl, err := template.New("caddyfile").Parse(caddyfileTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, tplData); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	d.logger.Debug("Generated Caddyfile with active container %s: %s", containerName, buf.String())
	return buf.String(), nil
}

// getActiveContainer determines which app container is currently running
func (d *Docker) getActiveContainer() string {
	if d.IsRunning(AppNamePrimary) {
		return AppNamePrimary
	}
	if d.IsRunning(AppNameSecondary) {
		return AppNameSecondary
	}
	// Default to primary if neither is running (initial deployment)
	return AppNamePrimary
}

func (d *Docker) waitForAppHealth(name string) error {
	d.logger.Info("Waiting for %s to become healthy...", name)
	for i := 0; i < HealthCheckTries; i++ {
		if _, err := d.RunCommand("exec", name, "curl", "-f", "http://localhost:8080/_health"); err == nil {
			d.logger.Success("%s is healthy", name)
			return nil
		}
		time.Sleep(2 * time.Second)
		if i == HealthCheckTries-1 {
			d.logger.Error("Container %s failed to become healthy after %d attempts", name, HealthCheckTries)
			d.logContainerLogs(name)
			return fmt.Errorf("app %s not healthy after %d attempts", name, HealthCheckTries)
		}
	}
	return nil
}

func (d *Docker) logContainerLogs(containerName string) {
	d.logger.Warn("Fetching logs from unhealthy container %s to diagnose issue:", containerName)

	logs, err := d.RunCommand("logs", "--tail", "50", containerName)
	if err != nil {
		d.logger.Error("Failed to fetch logs for container %s: %v", containerName, err)
		return
	}

	if logs == "" {
		d.logger.Warn("No logs available for container %s - checking container details...", containerName)
		
		// When no logs are available, provide more diagnostic info
		status, err := d.RunCommand("inspect", "--format", "{{.State.Status}}", containerName)
		if err == nil {
			d.logger.Info("Container %s current status: %s", containerName, strings.TrimSpace(status))
		}

		// Check exit code
		exitCode, err := d.RunCommand("inspect", "--format", "{{.State.ExitCode}}", containerName)
		if err == nil && strings.TrimSpace(exitCode) != "0" {
			d.logger.Error("Container exited with code: %s", strings.TrimSpace(exitCode))
		}

		// Check for error message
		errMsg, err := d.RunCommand("inspect", "--format", "{{.State.Error}}", containerName)
		if err == nil && strings.TrimSpace(errMsg) != "" && strings.TrimSpace(errMsg) != "<no value>" {
			d.logger.Error("Container error: %s", strings.TrimSpace(errMsg))
		}

		// Check restart count
		restarts, err := d.RunCommand("inspect", "--format", "{{.RestartCount}}", containerName)
		if err == nil && strings.TrimSpace(restarts) != "0" {
			d.logger.Warn("Container has restarted %s times", strings.TrimSpace(restarts))
		}

		// Check if it's still starting
		startedAt, err := d.RunCommand("inspect", "--format", "{{.State.StartedAt}}", containerName)
		if err == nil {
			d.logger.Info("Container started at: %s", strings.TrimSpace(startedAt))
		}

	} else {
		for _, line := range strings.Split(logs, "\n") {
			if line != "" {
				d.logger.Debug("[Container %s] %s", containerName, line)
			}
		}
	}

	status, err := d.RunCommand("inspect", "--format", "{{.State.Status}}", containerName)
	if err == nil {
		d.logger.Info("Container %s current status: %s", containerName, strings.TrimSpace(status))
	}

	errMsg, err := d.RunCommand("inspect", "--format", "{{.State.Error}}", containerName)
	if err == nil && strings.TrimSpace(errMsg) != "" && strings.TrimSpace(errMsg) != "<no value>" {
		d.logger.Error("Container %s error message: %s", containerName, strings.TrimSpace(errMsg))
	}
}

// DiagnoseContainerStartup provides comprehensive diagnostics for container startup issues
func (d *Docker) DiagnoseContainerStartup(containerName string) {
	d.logger.Error("=== CONTAINER STARTUP DIAGNOSTICS: %s ===", containerName)
	
	// Check if container exists
	if !d.containerExists(containerName) {
		d.logger.Error("Container %s does not exist - creation may have failed", containerName)
		return
	}

	// Get comprehensive container information
	d.logger.Info("Container exists, gathering diagnostic information...")
	
	// Get all logs with timestamps
	d.logger.Info("--- Full Container Logs ---")
	logs, err := d.RunCommand("logs", "--timestamps", containerName)
	if err != nil {
		d.logger.Error("Failed to get full logs: %v", err)
	} else if logs == "" {
		d.logger.Warn("No logs produced by container (possible immediate crash)")
	} else {
		lines := strings.Split(logs, "\n")
		for i, line := range lines {
			if line != "" {
				d.logger.Info("[LOG %d] %s", i+1, line)
			}
		}
	}
	
	// Get container state details
	d.logger.Info("--- Container State Details ---")
	stateJSON, err := d.RunCommand("inspect", "--format", "{{json .State}}", containerName)
	if err == nil {
		d.logger.Info("State JSON: %s", stateJSON)
	}

	// Check for common issues
	d.logger.Info("--- Common Issue Checks ---")
	
	// Check if image exists locally
	image, err := d.RunCommand("inspect", "--format", "{{.Config.Image}}", containerName)
	if err == nil {
		imageExists, imgErr := d.RunCommand("inspect", "--type=image", strings.TrimSpace(image))
		if imgErr != nil {
			d.logger.Error("Image %s may not exist locally: %v", strings.TrimSpace(image), imgErr)
		} else {
			d.logger.Info("Image %s exists locally", strings.TrimSpace(image))
			// Get image details
			d.logger.Debug("Image details: %s", strings.Split(imageExists, "\n")[0])
		}
	}

	// Check port conflicts
	ports, err := d.RunCommand("inspect", "--format", "{{range $p, $conf := .Config.ExposedPorts}}{{$p}} {{end}}", containerName)
	if err == nil && strings.TrimSpace(ports) != "" {
		d.logger.Info("Container exposes ports: %s", strings.TrimSpace(ports))
		
		// Check if ports are already in use
		for _, port := range strings.Fields(strings.TrimSpace(ports)) {
			portNum := strings.Split(port, "/")[0]
			if portNum != "" {
				// Check what's using this port
				d.logger.Debug("Checking port %s usage...", portNum)
				portCheck, portErr := d.RunCommand("ps", "-f", fmt.Sprintf("publish=%s", portNum))
				if portErr == nil && strings.TrimSpace(portCheck) != "" {
					d.logger.Warn("Port %s may be in use by other containers: %s", portNum, strings.TrimSpace(portCheck))
				}
			}
		}
	}

	// Check environment variables
	env, err := d.RunCommand("inspect", "--format", "{{range .Config.Env}}{{.}}\n{{end}}", containerName)
	if err == nil {
		d.logger.Info("--- Environment Variables ---")
		for _, envVar := range strings.Split(env, "\n") {
			if envVar != "" && !strings.Contains(envVar, "PASSWORD") && !strings.Contains(envVar, "SECRET") {
				d.logger.Debug("ENV: %s", envVar)
			}
		}
	}

	// Check mounts
	mounts, err := d.RunCommand("inspect", "--format", "{{range .Mounts}}{{.Source}}:{{.Destination}} ({{.Type}}) {{end}}", containerName)
	if err == nil && strings.TrimSpace(mounts) != "" {
		d.logger.Info("--- Volume Mounts ---")
		d.logger.Info("Mounts: %s", strings.TrimSpace(mounts))
	}

	// Check if there are recent similar containers that failed
	d.logger.Info("--- Recent Failed Containers ---")
	recent, err := d.RunCommand("ps", "-a", "--filter", fmt.Sprintf("name=%s", strings.TrimSuffix(containerName, "-1")+"-"), "--format", "table {{.Names}}\t{{.Status}}\t{{.CreatedAt}}")
	if err == nil {
		d.logger.Info("Recent containers with similar names:\n%s", recent)
	}

	d.logger.Error("=== END DIAGNOSTICS ===")
}

func (d *Docker) logCaddyVersion() {
	output, err := d.RunCommand("exec", CaddyName, "caddy", "version")
	if err == nil {
		d.logger.Info("Caddy version: %s", strings.TrimSpace(output))
	} else {
		d.logger.Warn("Failed to get Caddy version: %v", err)
	}
}

func (d *Docker) logContainerImage(containerName string) {
	output, err := d.RunCommand("inspect", containerName, "--format", "{{.Config.Image}}")
	if err == nil {
		d.logger.Info("%s is running image: %s", containerName, strings.TrimSpace(output))
	} else {
		d.logger.Warn("Failed to inspect %s image: %v", containerName, err)
	}
}

func (d *Docker) logImageDigest(image string) {
	output, err := d.RunCommand("inspect", image, "--format", "{{.Id}}")
	if err == nil {
		d.logger.Info("Image %s digest: %s", image, strings.TrimSpace(output))
	} else {
		d.logger.Warn("Failed to get digest for %s: %v", image, err)
	}
}

func (d *Docker) ContainerExists(name string) bool {
	// Check if the container exists, even if it's not running
	out, err := d.RunCommand("ps", "-a", "-q", "-f", "name="+name)
	return err == nil && strings.TrimSpace(out) != ""
}

func (d *Docker) containerExists(name string) bool {
	return d.ContainerExists(name)
}

// VerifyContainersRunning checks if the Fusionaly containers are running
func (d *Docker) VerifyContainersRunning() (bool, error) {
	// Check app container
	appRunning, err := d.isContainerRunning("fusionaly-app")
	if err != nil {
		return false, fmt.Errorf("failed to check app container: %w", err)
	}

	// Check Caddy container
	caddyRunning, err := d.isContainerRunning("fusionaly-caddy")
	if err != nil {
		return false, fmt.Errorf("failed to check Caddy container: %w", err)
	}

	return appRunning && caddyRunning, nil
}

// isContainerRunning checks if a specific container is running
func (d *Docker) isContainerRunning(containerName string) (bool, error) {
	cmd := exec.Command("docker", "ps", "--filter", "name="+containerName, "--format", "{{.Names}}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return false, fmt.Errorf("failed to check container status: %w", err)
	}

	return strings.Contains(string(output), containerName), nil
}

// generateAdminEmail generates the admin email for Let's Encrypt based on the domain
// Format: admin-fusionaly@{base_domain}
// Examples:
//   - "analytics.company.com" -> "admin-fusionaly@company.com"
//   - "t.example.org" -> "admin-fusionaly@example.org"
//   - "example.com" -> "admin-fusionaly@example.com"
func generateAdminEmail(domain string) string {
	baseDomain := extractBaseDomain(domain)
	return fmt.Sprintf("admin-fusionaly@%s", baseDomain)
}

// extractBaseDomain extracts the base domain from a subdomain
func extractBaseDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	
	// Handle localhost and IP addresses - return as-is
	localhostDomains := []string{
		"localhost", "127.0.0.1", "::1", "0.0.0.0", "localhost.localdomain",
	}
	for _, localhost := range localhostDomains {
		if domain == localhost {
			return domain
		}
	}
	
	// Check for localhost with port or subdomains
	if strings.HasPrefix(domain, "localhost:") || strings.HasSuffix(domain, ".localhost") {
		return domain
	}
	
	// Split by dots
	parts := strings.Split(domain, ".")
	if len(parts) <= 2 {
		// Already a base domain (e.g., "company.com" or single label)
		return domain
	}
	
	// For domains with more than 2 parts, take the last 2
	// This handles most cases correctly:
	// - "analytics.company.com" -> "company.com"
	// - "sub.domain.example.org" -> "example.org"
	return strings.Join(parts[len(parts)-2:], ".")
}

// ShowContainerStatus displays detailed status information for all relevant containers
func (d *Docker) ShowContainerStatus() {
	d.logger.Debug("=== Container Status ===")
	
	containers := []string{CaddyName, AppNamePrimary, AppNameSecondary}
	for _, container := range containers {
		if d.IsRunning(container) {
			status, err := d.RunCommand("inspect", "--format", "{{.State.Status}}", container)
			if err == nil {
				d.logger.Debug("Container %s: %s", container, strings.TrimSpace(status))
			}
			
			// Get image info
			image, err := d.RunCommand("inspect", "--format", "{{.Config.Image}}", container)
			if err == nil {
				d.logger.Debug("  Image: %s", strings.TrimSpace(image))
			}
			
			// Get ports info
			ports, err := d.RunCommand("port", container)
			if err == nil && strings.TrimSpace(ports) != "" {
				d.logger.Debug("  Ports: %s", strings.TrimSpace(ports))
			}
		} else {
			d.logger.Debug("Container %s: not running", container)
		}
	}
	
	d.logger.Debug("=====================")
}

// ShowContainerLogs displays the last N lines of logs for a specific container
func (d *Docker) ShowContainerLogs(containerName string, lines int) {
	d.logger.Debug("=== Logs for %s (last %d lines) ===", containerName, lines)
	
	if !d.containerExists(containerName) {
		d.logger.Debug("Container %s does not exist", containerName)
		return
	}
	
	logs, err := d.RunCommand("logs", "--tail", fmt.Sprintf("%d", lines), containerName)
	if err != nil {
		d.logger.Error("Failed to fetch logs for container %s: %v", containerName, err)
		return
	}
	
	if logs == "" {
		d.logger.Debug("No logs available for container %s", containerName)
	} else {
		for _, line := range strings.Split(logs, "\n") {
			if line != "" {
				d.logger.Debug("[%s] %s", containerName, line)
			}
		}
	}
	
	// Also show container status and error if any
	status, err := d.RunCommand("inspect", "--format", "{{.State.Status}}", containerName)
	if err == nil {
		d.logger.Debug("Container %s status: %s", containerName, strings.TrimSpace(status))
	}
	
	errMsg, err := d.RunCommand("inspect", "--format", "{{.State.Error}}", containerName)
	if err == nil && strings.TrimSpace(errMsg) != "" && strings.TrimSpace(errMsg) != "<no value>" {
		d.logger.Error("Container %s error: %s", containerName, strings.TrimSpace(errMsg))
	}
	
	d.logger.Debug("=== End logs for %s ===", containerName)
}
