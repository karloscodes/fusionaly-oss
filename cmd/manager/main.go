// Fusionaly Manager - Installation and management tool for Fusionaly
package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"golang.org/x/term"

	"github.com/karloscodes/matcha"
)

var currentManagerVersion string = "dev"

const ossImage = "karloscodes/fusionaly:latest"

// updateCronPath is the auto-update cron installed by matcha (CronUpdates).
const updateCronPath = "/etc/cron.d/fusionaly-update"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	m := newMatcha()

	switch os.Args[1] {
	case "install":
		printInstallBanner()
		if err := m.Install(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	case "update":
		migrateCaddyToKamalProxy(m)
		repairUpdateCron(m)
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
	case "check":
		if err := matcha.Check(); err != nil {
			os.Exit(1)
		}
	case "migrate-to-oss":
		runMigrateToOSS(m)
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
		HealthPath:     "/_health",
		Volumes:        []string{"/app/storage", "/app/logs"},
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

func runMigrateToOSS(m *matcha.Matcha) {
	fmt.Println("Migrate to Fusionaly")
	fmt.Println("====================")
	fmt.Println()
	fmt.Println("This switches your installation from Fusionaly Pro to Fusionaly.")
	fmt.Println("Your data is preserved. All former Pro features (AI Lens, activity")
	fmt.Println("feed) remain available.")
	fmt.Println()
	fmt.Println("This will:")
	fmt.Println("  - Back up your current database")
	fmt.Println("  - Switch to the Fusionaly Docker image")
	fmt.Println("  - Restart containers")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Proceed with migration? [Y/n]: ")

	confirm, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	confirm = strings.TrimSpace(strings.ToLower(confirm))
	if confirm != "" && confirm != "yes" && confirm != "y" {
		fmt.Println("Migration cancelled.")
		os.Exit(0)
	}

	// Backup database
	fmt.Println("Backing up database...")
	if _, err := m.BackupDB(); err != nil {
		fmt.Printf("Warning: backup failed: %v\n", err)
		fmt.Println("Proceeding without backup...")
	}

	// Switch to OSS image and deploy
	fmt.Println("Switching to Fusionaly...")
	m.SetImage(ossImage)

	// Save image to .env BEFORE Deploy (Deploy calls loadConfig which reads .env)
	if err := m.SaveImage(); err != nil {
		fmt.Printf("Error: failed to save image config: %v\n", err)
		os.Exit(1)
	}

	if err := m.Deploy(); err != nil {
		fmt.Printf("Error: migration failed: %v\n", err)
		os.Exit(1)
	}

	// Old /opt/fusionaly Pro installs carry a stale auto-update cron that
	// redirects to a now-missing log dir; replace it with the self-healing one.
	repairUpdateCron(m)

	fmt.Println("Migration completed successfully!")

	if domain, err := m.GetDomain(); err == nil && domain != "" {
		fmt.Printf("Visit https://%s to confirm everything works\n", domain)
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

// buildUpdateCron returns the self-healing auto-update cron line. It creates
// the log directory inline (mkdir -p) so a missing dir can never make cron's
// redirect fail before fusionaly runs — the failure that silently stopped
// nightly updates on boxes carrying a stale legacy cron. Appends (>>) so the
// log keeps history instead of only the last run.
func buildUpdateCron(binPath, logDir string) string {
	return fmt.Sprintf("# Fusionaly auto-update (managed by fusionaly)\n"+
		"# Runs daily at 3 AM\n"+
		"0 3 * * * root mkdir -p %s && %s update >> %s/update.log 2>&1\n",
		logDir, binPath, logDir)
}

// repairCronFile writes the self-healing cron to path when it is missing or
// differs from the desired content. Returns whether it wrote.
func repairCronFile(path, binPath, logDir string) (bool, error) {
	desired := buildUpdateCron(binPath, logDir)
	if current, err := os.ReadFile(path); err == nil && string(current) == desired {
		return false, nil
	}
	if err := os.WriteFile(path, []byte(desired), 0644); err != nil {
		return false, err
	}
	return true, nil
}

// repairUpdateCron ensures the auto-update cron is the self-healing form.
// Old /opt/fusionaly installers wrote a cron that redirected to a log dir
// which no longer exists; cron's `>` then failed before fusionaly ran, so
// nightly updates silently stopped. Runs on `update` and after migrate-to-oss.
func repairUpdateCron(m *matcha.Matcha) {
	binPath := m.GetConfig().BinaryPath
	if binPath == "" {
		binPath = "/usr/local/bin/fusionaly"
	}
	logDir := m.DataDir() + "/logs"

	wrote, err := repairCronFile(updateCronPath, binPath, logDir)
	if err != nil {
		fmt.Printf("Warning: could not repair auto-update cron: %v\n", err)
		return
	}
	if wrote {
		fmt.Println("Repaired auto-update cron (now self-creates its log dir).")
	}
}

// migrateCaddyToKamalProxy removes legacy Caddy and blue-green containers
// from pre-v1.6.0 installs so kamal-proxy can bind ports 80/443.
func migrateCaddyToKamalProxy(m *matcha.Matcha) {
	legacy := []string{"fusionaly-caddy", m.AppContainerName() + "-1", m.AppContainerName() + "-2"}
	migrated := false
	for _, name := range legacy {
		if exec.Command("docker", "inspect", name).Run() == nil {
			fmt.Printf("Migrating: removing legacy container %s...\n", name)
			exec.Command("docker", "stop", name).Run()
			exec.Command("docker", "rm", name).Run()
			migrated = true
		}
	}
	if migrated {
		fmt.Println("Re-registering service with proxy...")
		m.Reload()
	}
}

// printInstallBanner warns the operator to avoid subdomain labels that
// Brave Shields, uBlock Origin, AdGuard and EasyPrivacy blocklists reject
// outright. If Fusionaly is served from e.g. analytics.example.com, the
// tracking script URL itself gets blocked and events never reach the app.
func printInstallBanner() {
	const (
		bold   = "\033[1m"
		yellow = "\033[33m"
		dim    = "\033[2m"
		reset  = "\033[0m"
	)
	fmt.Println()
	fmt.Println(bold + yellow + "!  Pick a subdomain ad blockers won't block" + reset)
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println("Fusionaly is served from the domain you enter next. Browsers")
	fmt.Println("running Brave Shields, uBlock Origin, AdGuard, or any")
	fmt.Println("EasyPrivacy-based blocklist drop requests to hosts that begin")
	fmt.Println("with these labels, which means the tracking script loaded by")
	fmt.Println("your visitors will fail:")
	fmt.Println()
	fmt.Println("  " + bold + "AVOID:" + reset + "  analytics.  tracking.  track.  stats.  metrics.")
	fmt.Println("          telemetry.  counter.  pixel.  beacon.  collect.")
	fmt.Println("          ads.  adserver.  tagmanager.  tags.")
	fmt.Println()
	fmt.Println("  " + bold + "SAFE:" + reset + "   your apex domain (example.com) or a neutral")
	fmt.Println("          subdomain like fs., data., api., app., hub., m.,")
	fmt.Println("          or your own brand prefix.")
	fmt.Println()
	fmt.Println(dim + "More: https://fusionaly.com/docs/adblockers" + reset)
	fmt.Println(strings.Repeat("-", 60))
	fmt.Println()
}

func printUsage() {
	fmt.Println("Usage: fusionaly [command] [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  install                     Install Fusionaly")
	fmt.Println("  update                      Update an existing installation")
	fmt.Println("  migrate-to-oss              Switch a Fusionaly Pro install to Fusionaly")
	fmt.Println("  reload                      Reload containers with latest .env config")
	fmt.Println("  restore-db                  Interactively restore database from a backup")
	fmt.Println("  change-admin-password       Change the admin user password")
	fmt.Println("  version                     Show version information")
	fmt.Println("  check                       Check server security")
	fmt.Println("  help                        Show this help message")
}

