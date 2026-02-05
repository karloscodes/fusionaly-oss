package updater

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"fusionaly/internal/manager/config"
	"fusionaly/internal/manager/cron"
	"fusionaly/internal/manager/database"
	"fusionaly/internal/manager/docker"
	"fusionaly/internal/manager/logging"
)

const (
	GitHubRepo        = "karloscodes/fusionaly-oss"
	GitHubAPIURL      = "https://api.github.com/repos/" + GitHubRepo + "/releases/latest"
	BinaryInstallPath = "/usr/local/bin/fusionaly" // Keep simple name for backward compatibility
)

type Updater struct {
	logger   *logging.Logger
	config   *config.Config
	docker   *docker.Docker
	database *database.Database
}

func NewUpdater(logger *logging.Logger) *Updater {
	fileLogger := logging.NewFileLogger(logging.Config{
		Level:   logger.Level.String(),
		Verbose: logger.GetVerbose(),
		Quiet:   logger.GetQuiet(),
		LogDir:  "/opt/fusionaly/logs",
		LogFile: "fusionaly-updater.log",
	})

	db := database.NewDatabase(fileLogger)
	return &Updater{
		logger:   fileLogger,
		config:   config.NewConfig(fileLogger),
		docker:   docker.NewDocker(fileLogger, db),
		database: db,
	}
}

func (u *Updater) Run(currentVersion string) error {
	data := u.config.GetData()
	envFile := filepath.Join(data.InstallDir, ".env")

	u.logger.Info("Loading configuration")
	if err := u.config.LoadFromFile(envFile); err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	dockerImages := u.config.GetDockerImages()
	u.logger.Info("Using images from .env:")
	u.logger.Info("  - App image: %s", dockerImages.AppImage)
	u.logger.Info("  - Caddy image: %s", dockerImages.CaddyImage)

	// Fetch the latest version from GitHub
	latestVersion, binaryURL, checksumsURL, err := u.getLatestVersionAndBinaryURL()
	if err != nil {
		u.logger.Warn("Failed to fetch latest version from GitHub: %v", err)
		latestVersion = extractVersionFromURL(u.config.GetData().InstallerURL)
		if latestVersion == "" {
			u.logger.Warn("Could not determine latest version from URL: %s", u.config.GetData().InstallerURL)
		}
	}

	// Compare versions and update binary if necessary
	if latestVersion != "" {
		if compareVersions(currentVersion, latestVersion) < 0 {
			u.logger.Info("Local version %s is older than latest %s, updating binary...", currentVersion, latestVersion)
			arch := runtime.GOARCH
			if arch != "amd64" && arch != "arm64" {
				return fmt.Errorf("unsupported architecture: %s", arch)
			}

			downloadURL := binaryURL
			if downloadURL == "" {
				// Use fusionaly naming pattern (matches GoReleaser output)
				downloadURL = fmt.Sprintf("https://github.com/%s/releases/download/v%s/fusionaly-linux-%s", GitHubRepo, latestVersion, arch)
				u.logger.Info("Using binary URL: %s", downloadURL)
			}
			if checksumsURL == "" {
				checksumsURL = fmt.Sprintf("https://github.com/%s/releases/download/v%s/checksums.txt", GitHubRepo, latestVersion)
			}

			if err := u.updateBinary(downloadURL, checksumsURL, BinaryInstallPath); err != nil {
				u.logger.Warn("Failed to update binary: %v", err)
			} else {
				u.logger.Success("Binary updated to version %s", latestVersion)
				u.logger.Info("Restarting with new binary...")
				args := os.Args
				err = syscall.Exec(BinaryInstallPath, args, os.Environ())
				if err != nil {
					return fmt.Errorf("failed to exec new binary: %w", err)
				}
				return nil
			}
		} else {
			u.logger.Info("Current version %s matches or is newer than latest %s, no binary update needed", currentVersion, latestVersion)
		}
	}

	if err := u.update(); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	if err := u.config.SaveToFile(envFile); err != nil {
		return fmt.Errorf("save config: %w", err)
	}

	u.logger.Success("Update completed")
	return nil
}

func (u *Updater) getLatestVersionAndBinaryURL() (string, string, string, error) {
	u.logger.Info("Fetching latest release from GitHub: %s", GitHubAPIURL)

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Get(GitHubAPIURL)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to fetch latest release: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("failed to fetch latest release, status: %s", resp.Status)
	}

	var release struct {
		TagName string `json:"tag_name"`
		Assets  []struct {
			Name       string `json:"name"`
			BrowserURL string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", "", fmt.Errorf("failed to parse release JSON: %w", err)
	}

	latestVersion := strings.TrimPrefix(release.TagName, "v")
	if latestVersion == "" {
		return "", "", "", fmt.Errorf("invalid version in release tag: %s", release.TagName)
	}

	arch := runtime.GOARCH
	// Binary naming pattern (matches GoReleaser output)
	expectedAsset := fmt.Sprintf("fusionaly-linux-%s", arch)

	var binaryURL, checksumsURL string

	for _, asset := range release.Assets {
		if asset.Name == expectedAsset {
			binaryURL = asset.BrowserURL
		}
		if asset.Name == "checksums.txt" {
			checksumsURL = asset.BrowserURL
		}
	}

	if binaryURL == "" {
		return latestVersion, "", "", fmt.Errorf("no binary found for architecture %s in release v%s (expected %s)", arch, latestVersion, expectedAsset)
	}

	u.logger.Info("Found binary: %s", binaryURL)
	return latestVersion, binaryURL, checksumsURL, nil
}

func (u *Updater) update() error {
	totalSteps := 3

	u.logger.Info("Step 1/%d: Loading configuration", totalSteps)
	data := u.config.GetData()
	envFile := filepath.Join(data.InstallDir, ".env")
	if err := u.config.LoadFromFile(envFile); err != nil {
		return fmt.Errorf("failed to load config from %s: %w", envFile, err)
	}

	u.logger.Info("Step 2/%d: Applying updates", totalSteps)

	mainDBPath := u.config.GetMainDBPath()
	backupDir := u.config.GetData().BackupPath
	// Always backup database before update
	if _, err := u.database.BackupDatabase(mainDBPath, backupDir); err != nil {
		u.logger.Warn("Failed to backup database before update: %v", err)
		u.logger.Warn("Proceeding with update without backup")
	} else {
		u.logger.Success("Database backup created successfully")
	}

	// Read admin user from database and update config
	if adminUser, err := u.database.GetAdminUser(mainDBPath); err != nil {
		u.logger.Warn("Failed to read admin user from database: %v", err)
	} else if adminUser != "" {
		data := u.config.GetData()
		data.User = adminUser
		u.config.SetData(data)
		u.logger.Info("Updated configuration with admin user: %s", adminUser)
	}

	if err := u.docker.Update(u.config); err != nil {
		return fmt.Errorf("failed to update Docker containers: %w", err)
	}

	u.logger.Info("Step 3/%d: Updating cron job", totalSteps)
	cronManager := cron.NewManager(u.logger)
	if err := cronManager.SetupCronJob(); err != nil {
		u.logger.Warn("Failed to update cron job: %v", err)
	} else {
		u.logger.Success("Cron job updated successfully")
	}

	if err := u.config.SaveToFile(envFile); err != nil {
		return fmt.Errorf("failed to save config to %s: %w", envFile, err)
	}

	u.logger.Success("Update completed successfully")
	return nil
}

func (u *Updater) updateBinary(url, checksumsURL, binaryPath string) error {
	u.logger.InfoWithTime("Downloading new installer binary from %s", url)
	u.logger.Info("Current user: uid=%d, gid=%d", os.Getuid(), os.Getgid())
	u.logger.Info("Destination binary path: %s", binaryPath)

	client := &http.Client{
		Timeout: 60 * time.Second,
	}

	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed, status: %s", resp.Status)
	}

	newBinary := filepath.Join("/tmp", "fusionaly.new")

	if _, err := os.Stat(newBinary); err == nil {
		os.Remove(newBinary)
	}

	out, err := os.Create(newBinary)
	if err != nil {
		return fmt.Errorf("create new binary: %w", err)
	}
	defer out.Close()

	written, err := io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("write new binary: %w", err)
	}
	u.logger.Info("Downloaded %d bytes", written)
	out.Close()

	// Verify checksum if checksums URL is available
	if checksumsURL != "" {
		binaryName := filepath.Base(url)
		if err := u.verifyChecksum(client, checksumsURL, newBinary, binaryName); err != nil {
			os.Remove(newBinary)
			return fmt.Errorf("checksum verification failed: %w", err)
		}
		u.logger.Success("SHA256 checksum verified")
	} else {
		u.logger.Warn("No checksums URL available, skipping checksum verification")
	}

	if err := os.Chmod(newBinary, 0o755); err != nil {
		return fmt.Errorf("chmod new binary: %w", err)
	}

	if err := os.Rename(newBinary, binaryPath); err != nil {
		return fmt.Errorf("replace binary: %w", err)
	}

	u.logger.Success("Binary updated successfully")
	return nil
}

// verifyChecksum downloads checksums.txt and verifies the binary's SHA256 hash.
func (u *Updater) verifyChecksum(client *http.Client, checksumsURL, filePath, binaryName string) error {
	u.logger.Info("Downloading checksums from %s", checksumsURL)

	resp, err := client.Get(checksumsURL)
	if err != nil {
		return fmt.Errorf("failed to download checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download checksums, status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read checksums: %w", err)
	}

	// Parse checksums.txt — format: "<sha256>  <filename>"
	var expectedHash string
	for _, line := range strings.Split(string(body), "\n") {
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == binaryName {
			expectedHash = parts[0]
			break
		}
	}

	if expectedHash == "" {
		return fmt.Errorf("no checksum found for %s in checksums.txt", binaryName)
	}

	// Hash the downloaded file
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file for hashing: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("failed to hash file: %w", err)
	}

	actualHash := hex.EncodeToString(h.Sum(nil))

	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

func compareVersions(v1, v2 string) int {
	// Strip 'v' prefix if present
	v1 = strings.TrimPrefix(v1, "v")
	v2 = strings.TrimPrefix(v2, "v")
	
	v1Parts := strings.Split(v1, ".")
	v2Parts := strings.Split(v2, ".")

	maxParts := len(v1Parts)
	if len(v2Parts) > maxParts {
		maxParts = len(v2Parts)
	}
	for i := len(v1Parts); i < maxParts; i++ {
		v1Parts = append(v1Parts, "0")
	}
	for i := len(v2Parts); i < maxParts; i++ {
		v2Parts = append(v2Parts, "0")
	}

	for i := 0; i < maxParts; i++ {
		v1Num, err1 := strconv.Atoi(strings.TrimSpace(v1Parts[i]))
		if err1 != nil {
			// Invalid version part, treat as 0
			v1Num = 0
		}
		
		v2Num, err2 := strconv.Atoi(strings.TrimSpace(v2Parts[i]))
		if err2 != nil {
			// Invalid version part, treat as 0
			v2Num = 0
		}

		if v1Num < v2Num {
			return -1
		} else if v1Num > v2Num {
			return 1
		}
	}
	return 0
}

func extractVersionFromURL(url string) string {
	// Extract version from release URL path like:
	// https://github.com/karloscodes/fusionaly-oss/releases/download/v1.0.0/fusionaly-manager-linux-amd64
	parts := strings.Split(url, "/")
	for _, part := range parts {
		if strings.HasPrefix(part, "v") && len(part) > 1 {
			// Check if this looks like a version tag (v1.0.0, v1.2.3, etc.)
			version := strings.TrimPrefix(part, "v")
			if len(version) > 0 && (version[0] >= '0' && version[0] <= '9') {
				return version
			}
		}
	}
	return ""
}
