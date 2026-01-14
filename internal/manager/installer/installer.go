package installer

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"fusionaly/internal/manager/config"
	"fusionaly/internal/manager/cron"
	"fusionaly/internal/manager/database"
	"fusionaly/internal/manager/docker"
	"fusionaly/internal/manager/logging"
	"fusionaly/internal/manager/requirements"
)

const (
	DefaultInstallDir   = "/opt/fusionaly"
	DefaultBinaryPath   = "/usr/local/bin/fusionaly-manager"
	DefaultCronFile     = "/etc/cron.d/fusionaly-update"
	DefaultCronSchedule = "0 3 * * *"
)

type Installer struct {
	logger       *logging.Logger
	config       *config.Config
	docker       *docker.Docker
	database     *database.Database
	binaryPath   string
	portWarnings []string
}

func NewInstaller(logger *logging.Logger) *Installer {
	db := database.NewDatabase(logger)
	d := docker.NewDocker(logger, db)
	return &Installer{
		logger:     logger,
		config:     config.NewConfig(logger),
		docker:     d,
		database:   db,
		binaryPath: DefaultBinaryPath,
	}
}

func (i *Installer) GetConfig() *config.Config {
	return i.config
}

func (i *Installer) GetMainDBPath() string {
	data := i.config.GetData()
	return filepath.Join(data.InstallDir, "storage", "fusionaly-production.db")
}

func (i *Installer) GetBackupDir() string {
	data := i.config.GetData()
	return filepath.Join(data.InstallDir, "storage", "backups")
}

func (i *Installer) RunWithConfig(cfg *config.Config) error {
	i.config = cfg
	return i.Run()
}

// RunCompleteInstallation runs the complete installation process with proper coordination
func (i *Installer) RunCompleteInstallation() error {
	totalSteps := 7

	// Step 1: Display welcome message and collect ALL user input upfront
	i.displayWelcomeMessage()
	fmt.Println("Please provide the required configuration details:")
	reader := bufio.NewReader(os.Stdin)
	i.config = config.NewConfig(i.logger)
	if err := i.config.CollectFromUser(reader); err != nil {
		return fmt.Errorf("failed to collect configuration: %w", err)
	}

	// Step 2: Validate system requirements (no system changes yet)
	i.logger.Info("Step 1/%d: Checking system requirements", totalSteps)
	checker := requirements.NewChecker(i.logger)
	if err := checker.CheckSystemRequirements(); err != nil {
		return fmt.Errorf("system requirements check failed: %w", err)
	}
	i.logger.Success("System requirements verified")

	// Step 3: Install SQLite
	i.logger.Info("Step 2/%d: Installing SQLite", totalSteps)
	if err := i.database.EnsureSQLiteInstalled(); err != nil {
		return fmt.Errorf("failed to install SQLite: %w", err)
	}
	i.logger.Success("SQLite installed")

	// Step 4: Install Docker
	i.logger.Info("Step 3/%d: Installing Docker", totalSteps)
	progressChan := make(chan int, 1)
	go i.showProgress(progressChan, "Docker installation")
	if err := i.docker.EnsureInstalled(); err != nil {
		close(progressChan)
		return fmt.Errorf("failed to install Docker: %w", err)
	}
	progressChan <- 100
	close(progressChan)
	i.logger.Success("Docker installed")

	// Step 5: Configure system
	i.logger.Info("Step 4/%d: Configuring system", totalSteps)
	if err := i.configureSystem(); err != nil {
		return fmt.Errorf("failed to configure system: %w", err)
	}
	i.logger.Success("System configured")

	// Step 6: Deploy application
	i.logger.Info("Step 5/%d: Deploying application", totalSteps)
	deployProgressChan := make(chan int, 1)
	go i.showProgress(deployProgressChan, "Application deployment")
	if err := i.docker.Deploy(i.config); err != nil {
		close(deployProgressChan)
		return fmt.Errorf("failed to deploy application: %w", err)
	}
	deployProgressChan <- 100
	close(deployProgressChan)
	i.logger.Success("Application deployed")

	// Step 7: Setup maintenance
	i.logger.Info("Step 6/%d: Setting up maintenance", totalSteps)
	if err := i.setupMaintenance(); err != nil {
		return fmt.Errorf("failed to setup maintenance: %w", err)
	}
	i.logger.Success("Maintenance configured")

	// Step 8: Verify installation
	i.logger.Info("Step 7/%d: Verifying installation", totalSteps)
	if _, err := i.VerifyInstallation(); err != nil {
		return fmt.Errorf("installation verification failed: %w", err)
	}
	i.logger.Success("Installation verified")

	return nil
}

// displayWelcomeMessage shows the initial welcome and requirements message
func (i *Installer) displayWelcomeMessage() {
	fmt.Println("🚀 Welcome to Fusionaly Installer!")
	fmt.Println()
	fmt.Println("📋 Requirements: Ports 80/443 available, root privileges, internet connection")
	fmt.Println("📋 DNS Configuration (Optional): A/AAAA records are optional but useful if set before install")
	fmt.Println("🔒 SSL certificates provided by Let's Encrypt with automatic renewal")
	fmt.Println()
}

// configureSystem handles all configuration-related tasks
func (i *Installer) configureSystem() error {
	data := i.config.GetData()
	
	// Create installation directory
	if err := i.createInstallDir(data.InstallDir); err != nil {
		return fmt.Errorf("failed to create install dir: %w", err)
	}
	
	// Handle .env file configuration
	envFile := filepath.Join(data.InstallDir, ".env")
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		// No existing .env file - save the user-provided configuration
		if err := i.config.SaveToFile(envFile); err != nil {
			return fmt.Errorf("failed to save config to %s: %w", envFile, err)
		}
	} else {
		// Existing .env file found - preserve only system-generated values
		if err := i.updateExistingConfig(envFile); err != nil {
			return fmt.Errorf("failed to update existing config: %w", err)
		}
	}
	
	// Fetch server configuration
	if err := i.config.FetchFromServer(""); err != nil {
		i.logger.Warn("Using defaults due to server config fetch failure: %v", err)
	} else {
		i.logger.Debug("Server configuration fetched")
	}
	
	// Validate final configuration
	if err := i.config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	
	return nil
}

// updateExistingConfig preserves system values but uses fresh user input
func (i *Installer) updateExistingConfig(envFile string) error {
	i.logger.InfoWithTime("Found existing .env file at %s", envFile)
	oldConfig := config.NewConfig(i.logger)
	if err := oldConfig.LoadFromFile(envFile); err != nil {
		return fmt.Errorf("failed to load existing config from %s: %w", envFile, err)
	}
	
	// Preserve only the private key from old config, use fresh user input for everything else
	oldData := oldConfig.GetData()
	currentData := i.config.GetData()
	if oldData.PrivateKey != "" {
		// Update config with preserved private key
		preservedData := currentData
		preservedData.PrivateKey = oldData.PrivateKey
		newConfig := config.NewConfig(i.logger)
		newConfig.SetData(preservedData)
		i.config = newConfig
	}
	
	// Save the updated configuration (fresh user input + preserved private key)
	if err := i.config.SaveToFile(envFile); err != nil {
		return fmt.Errorf("failed to save updated config to %s: %w", envFile, err)
	}
	i.logger.InfoWithTime("Updated configuration with fresh user input")
	
	return nil
}

// setupMaintenance handles maintenance setup (no admin user creation)
func (i *Installer) setupMaintenance() error {
	// Install binary for updates (non-critical)
	if err := i.installBinary(); err != nil {
		i.logger.Warn("Failed to install binary for updates: %v", err)
		// Continue anyway - this is not critical for basic functionality
	}
	
	// Setup cron job for automatic updates
	cronManager := cron.NewManager(i.logger)
	if err := cronManager.SetupCronJob(); err != nil {
		return fmt.Errorf("failed to setup cron: %w", err)
	}
	
	return nil
}

// DisplayCompletionMessage shows the final completion message with DNS warnings if needed
func (i *Installer) DisplayCompletionMessage() {
	// DNS warnings (if any)
	if i.config.HasDNSWarnings() {
		fmt.Println("\n\033[1m⚠️  DNS CONFIGURATION REQUIRED\033[0m")
		fmt.Println(strings.Repeat("-", 40))
		fmt.Println("The following DNS issues were detected during installation:")
		for _, warning := range i.config.GetDNSWarnings() {
			if strings.HasPrefix(warning, "Suggestion:") {
				fmt.Printf("   💡 %s\n", warning[11:])
			} else {
				fmt.Printf("   • %s\n", warning)
			}
		}
		fmt.Println("\n🛠️  NEXT STEPS:")
		data := i.config.GetData()
		fmt.Printf("   1. Configure DNS: Add A/AAAA record for %s pointing to this server\n", data.Domain)
		fmt.Println("   2. Wait for DNS propagation (up to 24 hours)")
		fmt.Printf("   3. Test access: https://%s\n", data.Domain)
		fmt.Println("   4. Monitor logs: sudo tail -f /opt/fusionaly/logs/caddy.log")
		fmt.Println("\n📋 Note: All components are installed. The system will work once DNS is configured.")
		fmt.Println("📋 SSL setup might not be immediate due to Let's Encrypt retries.")
	}

	// Final success message with dashboard access information
	fmt.Println()
	fmt.Println("🎉 Installation Complete!")
	fmt.Println("═══════════════════════════")
	data := i.config.GetData()
	fmt.Printf("🌐 Dashboard URL: https://%s\n", data.Domain)
	// Generate the admin email that will be used for Let's Encrypt
	baseDomain := extractBaseDomain(data.Domain)
	_ = fmt.Sprintf("admin-fusionaly@%s", baseDomain) // Keep for potential future use
	fmt.Println()
	fmt.Println("🚀 Your Fusionaly installation is ready!")
	fmt.Println("Thank you for choosing Fusionaly for your analytics needs.")
}

func (i *Installer) Run() error {
	totalSteps := 6

	i.logger.Info("Step 1/%d: Checking system privileges", totalSteps)
	// Step 1: Privilege check - already done in main, just confirm
	i.logger.Success("Root privileges confirmed")

	i.logger.Info("Step 2/%d: Setting up SQLite", totalSteps)
	// Step 2: SQLite
	i.logger.Info("Installing SQLite...")
	if err := i.database.EnsureSQLiteInstalled(); err != nil {
		i.logger.Error("SQLite installation failed: %v", err)
		return fmt.Errorf("failed to install SQLite: %w", err)
	}
	i.logger.Success("SQLite installed successfully")

	i.logger.Info("Step 3/%d: Setting up Docker", totalSteps)
	// Step 3: Docker
	i.logger.Info("Installing Docker...")
	// Show progress indicator for Docker installation
	progressChan := make(chan int, 1)
	go i.showProgress(progressChan, "Docker installation")
	if err := i.docker.EnsureInstalled(); err != nil {
		close(progressChan)
		i.logger.Error("Docker installation failed: %v", err)
		return fmt.Errorf("failed to install Docker: %w", err)
	}
	progressChan <- 100
	close(progressChan)
	i.logger.Success("Docker installed successfully")

	i.logger.Info("Step 4/%d: Configuring Fusionaly", totalSteps)
	// Step 4: Config
	data := i.config.GetData()
	if err := i.createInstallDir(data.InstallDir); err != nil {
		return fmt.Errorf("failed to create install dir: %w", err)
	}
	envFile := filepath.Join(data.InstallDir, ".env")
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		// No existing .env file - save the user-provided configuration
		if err := i.config.SaveToFile(envFile); err != nil {
			return fmt.Errorf("failed to save config to %s: %w", envFile, err)
		}
	} else {
		// Existing .env file found - preserve only system-generated values (like private key)
		// but use fresh user-provided values for domain, email, and license
		i.logger.InfoWithTime("Found existing .env file at %s", envFile)
		oldConfig := config.NewConfig(i.logger)
		if err := oldConfig.LoadFromFile(envFile); err != nil {
			return fmt.Errorf("failed to load existing config from %s: %w", envFile, err)
		}
		
		// Preserve only the private key from old config, use fresh user input for everything else
		oldData := oldConfig.GetData()
		currentData := i.config.GetData()
		if oldData.PrivateKey != "" {
			// Update config with preserved private key
			preservedData := currentData
			preservedData.PrivateKey = oldData.PrivateKey
			newConfig := config.NewConfig(i.logger)
			newConfig.SetData(preservedData)
			i.config = newConfig
		}
		
		// Save the updated configuration (fresh user input + preserved private key)
		if err := i.config.SaveToFile(envFile); err != nil {
			return fmt.Errorf("failed to save updated config to %s: %w", envFile, err)
		}
		i.logger.InfoWithTime("Updated configuration with fresh user input")
	}

	i.logger.Info("Fetching server configuration...")
	if err := i.config.FetchFromServer(""); err != nil {
		i.logger.Warn("Using defaults due to server config fetch failure: %v", err)
	} else {
		i.logger.Success("Server configuration fetched")
	}

	if err := i.config.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}
	i.logger.Success("Configuration validated and saved to %s", envFile)

	i.logger.Info("Step 5/%d: Deploying Fusionaly", totalSteps)
	// Step 5: Deploy
	i.logger.Info("Deploying Docker containers...")
	// Show progress indicator for deployment
	deployProgressChan := make(chan int, 1)
	go i.showProgress(deployProgressChan, "Deployment")
	if err := i.docker.Deploy(i.config); err != nil {
		close(deployProgressChan)
		i.logger.Error("Deployment failed: %v", err)
		return fmt.Errorf("failed to deploy: %w", err)
	}
	deployProgressChan <- 100
	close(deployProgressChan)
	i.logger.Success("Deployment completed")

	i.logger.Info("Step 6/%d: Setting up maintenance", totalSteps)
	// Step 6: Maintenance setup
	// Install the binary itself for updates and cron jobs
	if err := i.installBinary(); err != nil {
		i.logger.Warn("Failed to install binary for updates: %v", err)
		// Don't fail installation, just warn
	}

	i.logger.InfoWithTime("Setting up automated updates")
	cronManager := cron.NewManager(i.logger)
	if err := cronManager.SetupCronJob(); err != nil {
		return fmt.Errorf("failed to setup cron: %w", err)
	}
	i.logger.Success("Daily automatic updates configured for 3:00 AM")

	return nil
}

// ListBackups returns available database backups
func (i *Installer) ListBackups() ([]database.BackupFile, error) {
	backupDir := i.GetBackupDir()
	return i.database.ListBackups(backupDir)
}

// PromptBackupSelection allows user to select from available backups
func (i *Installer) PromptBackupSelection(backups []database.BackupFile) (string, error) {
	return i.database.PromptSelection(backups)
}

// ValidateBackup validates the selected backup file
func (i *Installer) ValidateBackup(backupPath string) error {
	return i.database.ValidateBackup(backupPath)
}

// RestoreFromBackup restores database from a specific backup file
func (i *Installer) RestoreFromBackup(backupPath string) error {
	mainDBPath := i.GetMainDBPath()
	
	i.logger.InfoWithTime("Restoring database from %s to %s", backupPath, mainDBPath)
	i.logger.Info("Restoring database...")

	// Show progress for restore operation
	progressChan := make(chan int, 1)
	go i.showProgress(progressChan, "Database restore")

	err := i.database.RestoreDatabase(mainDBPath, backupPath)
	if err != nil {
		close(progressChan)
		i.logger.Error("Restore failed: %v", err)
		return fmt.Errorf("failed to restore database: %w", err)
	}

	progressChan <- 100
	close(progressChan)

	i.logger.Success("Database restored successfully")
	return nil
}

func (i *Installer) createInstallDir(installDir string) error {
	i.logger.InfoWithTime("Creating installation directory: %s", installDir)
	if err := os.MkdirAll(installDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	i.logger.Success("Installation directory created")
	return nil
}


// VerifyInstallation provides a way to verify that the installation completed successfully
func (i *Installer) VerifyInstallation() ([]string, error) {
	var warnings []string
	// Check that Docker containers are running
	containersRunning, err := i.docker.VerifyContainersRunning()
	if err != nil {
		return warnings, fmt.Errorf("installation verification failed: %w", err)
	}
	if !containersRunning {
		return warnings, fmt.Errorf("Docker containers are not running properly")
	}

	// Skip database check in test environment
	if os.Getenv("ENV") != "test" {
		// Check that the database exists
		dbPath := i.GetMainDBPath()
		if _, err := os.Stat(dbPath); os.IsNotExist(err) {
			return warnings, fmt.Errorf("database file not found: %w", err)
		}
	}

	// Ports are now checked as hard requirements before installation
	return warnings, nil
}

// checkPort checks if a port is available
func checkPort(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// showProgress displays a progress indicator for long-running operations
func (i *Installer) showProgress(progressChan <-chan int, operationName string) {
	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

	progress := 0
	spinner := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	spinnerIdx := 0
	stages := []string{"Starting", "Preparing", "Downloading", "Installing", "Configuring", "Finalizing"}
	stageIdx := 0

	// Clear the line and move cursor to beginning
	clearLine := func() {
		fmt.Print("\r\033[K") // ANSI escape code to clear line
	}

	for {
		select {
		case p, ok := <-progressChan:
			if !ok {
				return
			}
			progress = p

			// Update stage based on progress
			if progress < 20 {
				stageIdx = 0
			} else if progress < 40 {
				stageIdx = 1
			} else if progress < 60 {
				stageIdx = 2
			} else if progress < 80 {
				stageIdx = 3
			} else if progress < 95 {
				stageIdx = 4
			} else {
				stageIdx = 5
			}

			if progress >= 100 {
				clearLine()
				fmt.Print("\n") // Add newline before success message
				// Use consistent success format without emoji
				i.logger.Success("%s completed", operationName)
				return
			}
		case <-ticker.C:
			if progress < 100 {
				clearLine()
				currentStage := stages[stageIdx]
				fmt.Printf("\r● %s: %s %s", operationName, currentStage, spinner[spinnerIdx])
				spinnerIdx = (spinnerIdx + 1) % len(spinner)

				// Simulate progress if actual progress is not being reported
				if progress < 95 {
					progress += 2
				}
			}
		}
	}
}

// installBinary copies the current executable to the system binary path for updates and cron jobs
func (i *Installer) installBinary() error {
	if os.Getenv("ENV") == "test" {
		i.logger.InfoWithTime("Skipping binary installation in test environment")
		return nil
	}

	// Get the current executable path
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get current executable path: %w", err)
	}

	i.logger.InfoWithTime("Installing binary from %s to %s", currentExe, i.binaryPath)

	// Read the current executable
	sourceData, err := os.ReadFile(currentExe)
	if err != nil {
		return fmt.Errorf("failed to read source binary: %w", err)
	}

	// Write to destination
	if err := os.WriteFile(i.binaryPath, sourceData, 0755); err != nil {
		return fmt.Errorf("failed to write binary to %s: %w", i.binaryPath, err)
	}

	i.logger.Success("Binary installed successfully at %s", i.binaryPath)
	return nil
}

// extractBaseDomain extracts the base domain from a subdomain
// Examples:
//   - "analytics.company.com" -> "company.com"
//   - "t.getfusionaly.com" -> "getfusionaly.com"
//   - "google.com" -> "google.com"
//   - "localhost" -> "localhost"
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
