// Fusionaly Manager - Installation and management tool for Fusionaly
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"

	"fusionaly/internal/manager/admin"
	"fusionaly/internal/manager/errors"
	"fusionaly/internal/manager/installer"
	"fusionaly/internal/manager/logging"
	"fusionaly/internal/manager/updater"
	"fusionaly/internal/manager/upgrader"
	"fusionaly/internal/manager/validation"
)

var currentManagerVersion string = "dev"

func main() {
	// Detect the current working directory
	workingDirectory, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error: Failed to determine working directory: %v\n", err)
		os.Exit(1)
	}

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	// Initialize logging
	startTime := time.Now()
	logger := initLogging()
	logger.Debug("Manager version: %s", currentManagerVersion)
	logger.Debug("Working directory: %s", workingDirectory)

	inst := installer.NewInstaller(logger)

	// Update environment variables with current version
	os.Setenv("FUSIONALY_VERSION", currentManagerVersion)

	switch os.Args[1] {
	case "install":
		runInstall(inst, logger)
	case "update":
		runUpdate(inst, logger, startTime)
	case "reload":
		runReload(logger, startTime)
	case "restore-db":
		runRestoreDB(inst, logger, startTime)
	case "change-admin-password":
		if err := runAdminPasswordChange(logger); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "upgrade":
		runUpgrade(logger, startTime)
	case "version", "--version", "-v":
		printVersion()
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func initLogging() *logging.Logger {
	logLevel := "info"
	if envLevel := os.Getenv("LOG_LEVEL"); envLevel != "" {
		logLevel = envLevel
	}

	verbose := false
	if os.Getenv("VERBOSE") == "true" {
		verbose = true
	}

	quiet := false
	if os.Getenv("QUIET") == "true" {
		quiet = true
	}

	// Configure the main logger to log to stdout
	logger := logging.NewLogger(logging.Config{
		Level:   logLevel,
		Verbose: verbose,
		Quiet:   quiet,
	})

	return logger
}

func runInstall(inst *installer.Installer, logger *logging.Logger) {
	logger.Debug("Initializing installation environment")

	// Run the complete installation process
	if err := inst.RunCompleteInstallation(); err != nil {
		logger.Error("Installation failed: %v", err)
		os.Exit(1)
	}

	// Display final success message and access information
	inst.DisplayCompletionMessage()

	os.Stdout.Sync() // Force flush to ensure output is captured
}

func runUpdate(inst *installer.Installer, logger *logging.Logger, startTime time.Time) {
	logger.Debug("Initializing update environment")

	upd := updater.NewUpdater(logger)
	logger.Info("Running update...")
	err := upd.Run(currentManagerVersion)
	if err != nil {
		logger.Error("Update failed: %v", err)
		os.Exit(1)
	}

	elapsedTime := time.Since(startTime).Round(time.Second)
	logger.Success("Update completed in %s", elapsedTime)
}

func runRestoreDB(inst *installer.Installer, logger *logging.Logger, startTime time.Time) {
	logger.Info("Starting database restore...")

	backupDir := inst.GetBackupDir()
	mainDBPath := inst.GetMainDBPath()

	// List available backups
	backups, err := inst.ListBackups()
	if err != nil {
		logger.Error("Failed to list backups: %v", err)
		os.Exit(1)
	}

	if len(backups) == 0 {
		logger.Error("No backups found in %s", backupDir)
		os.Exit(1)
	}

	// Let user select a backup
	selectedBackup, err := inst.PromptBackupSelection(backups)
	if err != nil {
		logger.Error("Backup selection failed: %v", err)
		os.Exit(1)
	}

	// Validate the selected backup
	if err := inst.ValidateBackup(selectedBackup); err != nil {
		logger.Error("Backup validation failed: %v", err)
		os.Exit(1)
	}

	// Confirmation prompt
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("Warning: This will replace your current database with the selected backup.\n")
	fmt.Printf("   Current database: %s\n", mainDBPath)
	fmt.Printf("   Selected backup: %s\n", selectedBackup)
	fmt.Print("Are you sure you want to continue? (yes/no): ")

	confirmation, err := reader.ReadString('\n')
	if err != nil {
		logger.Error("Failed to read confirmation: %v", err)
		os.Exit(1)
	}

	confirmation = strings.TrimSpace(strings.ToLower(confirmation))
	if confirmation != "yes" && confirmation != "y" {
		logger.Info("Restore cancelled by user")
		os.Exit(0)
	}

	// Perform the restore
	err = inst.RestoreFromBackup(selectedBackup)
	if err != nil {
		logger.Error("Restore failed: %v", err)
		os.Exit(1)
	}

	elapsedTime := time.Since(startTime).Round(time.Second)
	logger.Success("Database restored successfully in %s", elapsedTime)
	logger.Info("Verify the installation by running: sudo docker ps | grep fusionaly")
}

func runReload(logger *logging.Logger, startTime time.Time) {
	fmt.Println("Reloading containers with latest configuration")
	logger.Debug("Initializing reload environment")

	reloader := updater.NewReloader(logger)
	logger.Info("Reloading containers...")
	err := reloader.Run()
	if err != nil {
		logger.Error("Reload failed: %v", err)
		os.Exit(1)
	}

	elapsedTime := time.Since(startTime).Round(time.Second)
	logger.Success("Reload completed in %s", elapsedTime)
}

func runAdminPasswordChange(logger *logging.Logger) error {
	startTime := time.Now()
	adminMgr := admin.NewManager(logger)
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter admin email: ")
	emailInput, err := reader.ReadString('\n')
	if err != nil {
		logger.Error("Failed to read email: %v", err)
		return err
	}
	email := strings.TrimSpace(emailInput)
	if err := validation.ValidateEmail(email); err != nil {
		logger.Error("Invalid email: %v", err)
		return errors.WrapWithContext(err, "email validation failed")
	}

	var password string
	for {
		fmt.Print("Enter new admin password (minimum 8 characters): ")
		passBytes, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			logger.Error("Failed to read password: %v", err)
			return err
		}
		fmt.Println()

		password = strings.TrimSpace(string(passBytes))
		if err := validation.ValidatePassword(password); err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Print("Confirm new admin password: ")
		confirmBytes, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			logger.Error("Failed to read confirmation password: %v", err)
			return err
		}
		fmt.Println()

		confirm := strings.TrimSpace(string(confirmBytes))
		if password != confirm {
			fmt.Println("Error: Passwords do not match. Please try again.")
			continue
		}
		break
	}

	if err := adminMgr.ChangeAdminPassword(email, password); err != nil {
		logger.Error("Failed to change admin password: %v", err)
		return err
	}

	elapsed := time.Since(startTime).Round(time.Second)
	logger.Success("Password changed in %s", elapsed)
	return nil
}

func runUpgrade(logger *logging.Logger, startTime time.Time) {
	fmt.Println("Upgrade to Fusionaly Pro")
	fmt.Println("========================")
	fmt.Println()
	fmt.Println("This will:")
	fmt.Println("  - Back up your current database")
	fmt.Println("  - Switch to the Pro Docker image")
	fmt.Println("  - Restart containers")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Proceed with upgrade? [Y/n]: ")

	confirmation, err := reader.ReadString('\n')
	if err != nil {
		logger.Error("Failed to read confirmation: %v", err)
		os.Exit(1)
	}

	confirmation = strings.TrimSpace(strings.ToLower(confirmation))
	if confirmation != "" && confirmation != "yes" && confirmation != "y" {
		logger.Info("Upgrade cancelled by user")
		os.Exit(0)
	}

	logger.Info("Upgrading to Fusionaly Pro...")

	upg := upgrader.NewUpgrader(logger)
	if err := upg.Run(); err != nil {
		logger.Error("Upgrade failed: %v", err)
		os.Exit(1)
	}

	elapsedTime := time.Since(startTime).Round(time.Second)
	logger.Success("Upgrade completed in %s", elapsedTime)

	domain := upg.GetDomain()
	if domain != "" {
		logger.Info("Visit https://%s to complete Pro setup", domain)
	}
}

func printVersion() {
	fmt.Println(currentManagerVersion)
}

func printUsage() {
	fmt.Println("Usage: fusionaly [command] [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  install                     Install Fusionaly")
	fmt.Println("  update                      Update an existing installation")
	fmt.Println("  upgrade                     Upgrade from OSS to Fusionaly Pro")
	fmt.Println("  reload                      Reload containers with latest .env config")
	fmt.Println("  restore-db                  Interactively restore database from a backup")
	fmt.Println("  change-admin-password       Change the admin user password")
	fmt.Println("  version                     Show version information")
	fmt.Println("  help                        Show this help message")
}
