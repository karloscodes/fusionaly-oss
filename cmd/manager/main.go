// Fusionaly Manager - Installation and management tool for Fusionaly
package main

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/karloscodes/matcha"
)

var currentManagerVersion string = "dev"

const proImage = "karloscodes/fusionaly-pro:latest"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	m := newMatcha()

	switch os.Args[1] {
	case "install":
		if err := m.Install(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "update":
		if err := m.Update(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "reload":
		if err := m.Reload(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "restore-db":
		if err := m.RestoreDB(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "change-admin-password":
		if err := runAdminPasswordChange(m); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "upgrade":
		runUpgrade(m)
	case "version", "--version", "-v":
		fmt.Println(currentManagerVersion)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func newMatcha() *matcha.Matcha {
	return matcha.New(matcha.Config{
		Name:           "fusionaly",
		AppImage:       "karloscodes/fusionaly:latest",
		CaddyImage:     "caddy:2.9-alpine",
		BlueGreen:      true,
		CronUpdates:    true,
		Backups:        true,
		ManagerRepo:    "karloscodes/fusionaly-oss",
		ManagerVersion: currentManagerVersion,
	})
}

func runAdminPasswordChange(m *matcha.Matcha) error {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter admin email: ")
	email, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to read email: %w", err)
	}
	email = strings.TrimSpace(email)
	if err := validateEmail(email); err != nil {
		return err
	}

	var password string
	for {
		fmt.Print("Enter new admin password (minimum 8 characters): ")
		passBytes, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		fmt.Println()

		password = strings.TrimSpace(string(passBytes))
		if err := validatePassword(password); err != nil {
			fmt.Printf("Error: %v\n", err)
			continue
		}

		fmt.Print("Confirm new admin password: ")
		confirmBytes, err := term.ReadPassword(int(syscall.Stdin))
		if err != nil {
			return fmt.Errorf("failed to read password: %w", err)
		}
		fmt.Println()

		if password != strings.TrimSpace(string(confirmBytes)) {
			fmt.Println("Error: Passwords do not match. Please try again.")
			continue
		}
		break
	}

	fmt.Println("Changing password...")
	if err := m.Exec("/app/fnctl", "change-admin-password", email, password); err != nil {
		return fmt.Errorf("failed to change password: %w", err)
	}

	fmt.Println("Password changed successfully.")
	return nil
}

func runUpgrade(m *matcha.Matcha) {
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

	confirm, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	confirm = strings.TrimSpace(strings.ToLower(confirm))
	if confirm != "" && confirm != "yes" && confirm != "y" {
		fmt.Println("Upgrade cancelled.")
		os.Exit(0)
	}

	// Backup database
	fmt.Println("Backing up database...")
	if _, err := m.BackupDB(); err != nil {
		fmt.Printf("Warning: backup failed: %v\n", err)
		fmt.Println("Proceeding without backup...")
	}

	// Switch to Pro image and deploy
	fmt.Println("Switching to Fusionaly Pro...")
	m.SetImage(proImage)

	// Save image to .env BEFORE Deploy (Deploy calls loadConfig which reads .env)
	if err := m.SaveImage(); err != nil {
		fmt.Printf("Error: failed to save image config: %v\n", err)
		os.Exit(1)
	}

	if err := m.Deploy(); err != nil {
		fmt.Printf("Error: upgrade failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Upgrade completed successfully!")

	if domain, err := m.GetDomain(); err == nil && domain != "" {
		fmt.Printf("Visit https://%s to complete Pro setup\n", domain)
	}
}

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

func validateEmail(email string) error {
	if email == "" {
		return fmt.Errorf("email cannot be empty")
	}
	if !emailRegex.MatchString(email) {
		return fmt.Errorf("invalid email format")
	}
	return nil
}

func validatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}
	return nil
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

