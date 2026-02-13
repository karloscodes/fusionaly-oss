package config

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/term"

	"fusionaly/internal/manager/errors"
	"fusionaly/internal/manager/logging"
	"fusionaly/internal/manager/validation"
)

// ConfigData holds the configuration
type ConfigData struct {
	Domain       string   // User-provided domain
	AppImage     string   // Docker image for Fusionaly app
	CaddyImage   string   // Docker image for Caddy reverse proxy
	InstallDir   string   // Installation directory
	BackupPath   string   // SQLite backup location
	PrivateKey   string   // Secure random key for FUSIONALY_PRIVATE_KEY
	Version      string   // Version of the fusionaly binary
	InstallerURL string   // URL to download new fusionaly binary
	DNSWarnings  []string // DNS configuration warnings
	DNSType      string   // Type of DNS issue: "not_found", "wrong_ip", or ""
	ServerIP     string   // This server's public IP address
	User         string   // Admin user email from users table
}

// Config manages configuration
type Config struct {
	logger *logging.Logger
	data   ConfigData
}

// NewConfig creates a Config with defaults
func NewConfig(logger *logging.Logger) *Config {
	return &Config{
		logger: logger,
		data: ConfigData{
			Domain:       "",
			AppImage:     "karloscodes/fusionaly:latest",
			CaddyImage:   "caddy:2.7-alpine",
			InstallDir:   "/opt/fusionaly",
			BackupPath:   "/opt/fusionaly/storage/backups",
			PrivateKey:   "",
			Version:      "latest",
			InstallerURL: "https://github.com/karloscodes/fusionaly-oss/releases/latest",
		},
	}
}

// Helper function to get the current server's primary public IP address
func getCurrentServerIP() (string, error) {
	// Try to get IPs from multiple external services for better reliability
	externalServices := []string{
		"https://api.ipify.org",
		"https://ifconfig.me/ip",
		"https://icanhazip.com",
	}

	var publicIPs []string

	// Try external services first
	for _, service := range externalServices {
		resp, err := http.Get(service)
		if err == nil {
			defer resp.Body.Close()
			ip, err := io.ReadAll(resp.Body)
			if err == nil && len(ip) > 0 {
				publicIP := strings.TrimSpace(string(ip))
				publicIPs = append(publicIPs, publicIP)
				break // We got a valid IP, no need to try other services
			}
		}
	}

	// Also collect all local interface IPs
	var localIPs []string

	ifaces, err := net.Interfaces()
	if err != nil {
		// If we have at least one public IP from external services, return that
		if len(publicIPs) > 0 {
			return publicIPs[0], nil
		}
		return "", err
	}

	for _, iface := range ifaces {
		// Skip loopback and interfaces that are down
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Skip loopback addresses
			if ip.IsLoopback() {
				continue
			}

			// Only consider IPv4 addresses for simplicity
			if ip4 := ip.To4(); ip4 != nil {
				localIPs = append(localIPs, ip4.String())
			}
		}
	}

	// Return results
	if len(publicIPs) > 0 {
		return publicIPs[0], nil
	}

	if len(localIPs) > 0 {
		return strings.Join(localIPs, ","), nil
	}

	return "", fmt.Errorf("unable to determine server IP")
}

// Helper function to check domain against multiple IPs
func checkDomainIPMatch(domain string, serverIPs string) (bool, string) {
	ips, err := net.LookupIP(domain)
	if err != nil || len(ips) == 0 {
		return false, ""
	}

	// Convert comma-separated IPs to slice
	serverIPList := strings.Split(serverIPs, ",")

	var domainIPStrings []string
	for _, ip := range ips {
		ipStr := ip.String()
		domainIPStrings = append(domainIPStrings, ipStr)

		// Check if this domain IP matches any server IP
		for _, serverIP := range serverIPList {
			if ipStr == serverIP {
				return true, ipStr
			}
		}
	}

	// No match found, return false and the domain IPs
	return false, strings.Join(domainIPStrings, ", ")
}

// CollectFromUser gets required user input upfront
func (c *Config) CollectFromUser(reader *bufio.Reader) error {
	// Check if we're in non-interactive mode
	if os.Getenv("NONINTERACTIVE") == "1" {
		return c.collectFromEnvironment()
	}

	// Initialize default values
	c.data.Domain = ""
	c.data.InstallDir = "/opt/fusionaly"

	// Collect domain
	fmt.Println("Configuration")
	fmt.Println()
	for {
		fmt.Print("  Domain (e.g., analytics.example.com): ")
		domain, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read domain: %w", err)
		}
		c.data.Domain = strings.TrimSpace(domain)
		if c.data.Domain == "" {
			fmt.Println("  Error: Domain cannot be empty.")
			continue
		}

		// Validate domain format immediately using the same validation that will be used during installation
		if err := validation.ValidateDomain(c.data.Domain); err != nil {
			fmt.Printf("  Error: %s\n", err.Error())
			continue
		}
		break
	}

	// Check DNS records and store warnings instead of blocking
	c.CheckDNSAndStoreWarnings(c.data.Domain)

	c.data.BackupPath = filepath.Join(c.data.InstallDir, "storage", "backups")

	// Show configuration summary and get confirmation
	for {
		fmt.Println("Summary")
		fmt.Println()
		fmt.Printf("  Domain:  %s\n", c.data.Domain)
		if c.HasDNSWarnings() {
			fmt.Printf("  DNS:     Not ready (installation will continue)\n")
		} else {
			fmt.Printf("  DNS:     Ready\n")
		}

		fmt.Println()
		fmt.Print("  Proceed? [Y/n] ")
		confirmStr, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("failed to read confirmation: %w", err)
		}

		confirmStr = strings.TrimSpace(strings.ToLower(confirmStr))
		if confirmStr == "y" || confirmStr == "yes" || confirmStr == "" {
			break
		}

		fmt.Println()
		fmt.Println("  Configuration declined. Let's start over.")
		fmt.Println()
		// Reset all values and start over
		c.data.Domain = ""
		return c.CollectFromUser(reader)
	}

	return nil
}

// collectFromEnvironment reads configuration from environment variables
func (c *Config) collectFromEnvironment() error {
	c.logger.Info("Running in non-interactive mode, reading configuration from environment variables")

	// Read domain from environment
	domain := os.Getenv("DOMAIN")
	if domain == "" {
		return fmt.Errorf("DOMAIN environment variable is required in non-interactive mode")
	}
	c.data.Domain = domain

	c.logger.Info("Configuration loaded from environment variables:")
	c.logger.Info("  Domain: %s", c.data.Domain)

	// Set default values for other fields
	c.data.InstallDir = "/opt/fusionaly"
	c.data.BackupPath = filepath.Join(c.data.InstallDir, "backups")
	c.data.AppImage = "karloscodes/fusionaly:latest"
	c.data.CaddyImage = "caddy:2.7-alpine"

	return nil
}

// Helper function to format IPs for display
func formatIPs(ips []net.IP) string {
	ipStrings := make([]string, len(ips))
	for i, ip := range ips {
		ipStrings[i] = ip.String()
	}
	return strings.Join(ipStrings, ", ")
}

// LoadFromFile loads local config from .env
func (c *Config) LoadFromFile(filename string) error {
	c.logger.Info("Loading from %s", filename)
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key, value := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		switch key {
		case "FUSIONALY_DOMAIN":
			c.data.Domain = value
		case "APP_IMAGE":
			c.data.AppImage = value
		case "CADDY_IMAGE":
			c.data.CaddyImage = value
		case "INSTALL_DIR":
			c.data.InstallDir = value
		case "BACKUP_PATH":
			c.data.BackupPath = value
		case "VERSION":
			c.data.Version = value
		case "INSTALLER_URL":
			c.data.InstallerURL = value
		case "FUSIONALY_PRIVATE_KEY":
			c.data.PrivateKey = value
		case "FUSIONALY_USER":
			c.data.User = value
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// If PrivateKey is missing, generate one and append to file
	if c.data.PrivateKey == "" {
		pk, err := generatePrivateKey()
		if err != nil {
			return err
		}
		c.data.PrivateKey = pk
		// Append to file
		f, ferr := os.OpenFile(filename, os.O_APPEND|os.O_WRONLY, 0)
		if ferr == nil {
			fmt.Fprintf(f, "FUSIONALY_PRIVATE_KEY=%s\n", pk)
			f.Close()
			c.logger.Info("Added missing FUSIONALY_PRIVATE_KEY to %s", filename)
		}
	}
	c.logger.Success("Configuration loaded from %s", filename)
	return nil
}

// SaveToFile saves local config to .env
func (c *Config) SaveToFile(filename string) error {
	c.logger.Info("Saving to %s", filename)

	// Ensure private key is set
	if c.data.PrivateKey == "" {
		pk, err := generatePrivateKey()
		if err != nil {
			return err
		}
		c.data.PrivateKey = pk
		c.logger.Info("Generated new FUSIONALY_PRIVATE_KEY")
	}

	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	fmt.Fprintf(file, "FUSIONALY_DOMAIN=%s\n", c.data.Domain)
	fmt.Fprintf(file, "APP_IMAGE=%s\n", c.data.AppImage)
	fmt.Fprintf(file, "CADDY_IMAGE=%s\n", c.data.CaddyImage)
	fmt.Fprintf(file, "INSTALL_DIR=%s\n", c.data.InstallDir)
	fmt.Fprintf(file, "BACKUP_PATH=%s\n", c.data.BackupPath)
	fmt.Fprintf(file, "VERSION=%s\n", c.data.Version)
	fmt.Fprintf(file, "INSTALLER_URL=%s\n", c.data.InstallerURL)
	fmt.Fprintf(file, "FUSIONALY_PRIVATE_KEY=%s\n", c.data.PrivateKey)
	if c.data.User != "" {
		fmt.Fprintf(file, "FUSIONALY_USER=%s\n", c.data.User)
	}

	c.logger.Info("Configuration saved to %s", filename)
	return nil
}

// GetData returns the config data
func (c *Config) GetData() ConfigData {
	return c.data
}

// SetData updates the config data
func (c *Config) SetData(data ConfigData) {
	c.data = data
}

// SetCaddyImage sets the CaddyImage field in ConfigData
func (c *Config) SetCaddyImage(image string) {
	c.data.CaddyImage = image
	c.logger.Info("CaddyImage updated to: %s", image)
}

// DockerImages contains both app and caddy image information
type DockerImages struct {
	AppImage   string
	CaddyImage string
}

// GetDockerImages returns both Docker images in a structured way
func (c *Config) GetDockerImages() DockerImages {
	return DockerImages{
		AppImage:   c.data.AppImage,
		CaddyImage: c.data.CaddyImage,
	}
}

// SetInstallDir sets the InstallDir field in ConfigData
func (c *Config) SetInstallDir(dir string) {
	c.data.InstallDir = dir
}

// SetInstallerURL sets the InstallerURL field in ConfigData
func (c *Config) SetInstallerURL(url string) {
	c.data.InstallerURL = url
}

// GetMainDBPath returns the main database path
func (c *Config) GetMainDBPath() string {
	return filepath.Join(c.data.InstallDir, "storage", "fusionaly-production.db")
}

// Validate checks required fields
func (c *Config) Validate() error {
	// Validate domain
	if err := validation.ValidateDomain(c.data.Domain); err != nil {
		return errors.NewConfigError("domain", c.data.Domain, err.Error())
	}

	// Validate app image
	if c.data.AppImage == "" {
		return errors.NewConfigError("app_image", "", "app image cannot be empty")
	}

	// Validate caddy image
	if c.data.CaddyImage == "" {
		return errors.NewConfigError("caddy_image", "", "caddy image cannot be empty")
	}

	// Validate install directory path
	if err := validation.ValidateFilePath(c.data.InstallDir); err != nil {
		return errors.NewConfigError("install_dir", c.data.InstallDir, err.Error())
	}

	// Validate backup path
	if err := validation.ValidateFilePath(c.data.BackupPath); err != nil {
		return errors.NewConfigError("backup_path", c.data.BackupPath, err.Error())
	}

	// Validate private key (basic check)
	if c.data.PrivateKey == "" {
		return errors.NewConfigError("private_key", "", "private key cannot be empty")
	}
	if len(c.data.PrivateKey) < 32 {
		return errors.NewConfigError("private_key", "", "private key too short (minimum 32 characters)")
	}

	// Validate version if provided
	if c.data.Version != "" {
		if err := validation.ValidateVersion(c.data.Version); err != nil {
			return errors.NewConfigError("version", c.data.Version, err.Error())
		}
	}

	// Validate installer URL if provided
	if c.data.InstallerURL != "" {
		if err := validation.ValidateURL(c.data.InstallerURL); err != nil {
			return errors.NewConfigError("installer_url", c.data.InstallerURL, err.Error())
		}
	}

	return nil
}

// CheckDNSAndStoreWarnings checks DNS configuration and stores warnings instead of blocking
func (c *Config) CheckDNSAndStoreWarnings(domain string) {
	// Clear any existing warnings
	c.data.DNSWarnings = []string{}
	c.data.DNSType = ""
	c.data.ServerIP = ""

	// Skip DNS checks for localhost - no DNS resolution needed
	if isLocalhostDomain(domain) {
		fmt.Println()
		fmt.Println("Checking DNS...")
		fmt.Println()
		fmt.Printf("  ✓ %s (localhost, skipping DNS check)\n", domain)
		fmt.Println()
		return
	}

	fmt.Println()
	fmt.Println("Checking DNS...")
	fmt.Println()

	// Get server IP first
	serverIP, err := getCurrentServerIP()
	if err != nil {
		c.data.ServerIP = "unknown"
	} else {
		c.data.ServerIP = serverIP
	}

	ips, err := net.LookupIP(domain)
	if err != nil || len(ips) == 0 {
		// DNS not found - show full instructions
		c.data.DNSType = "not_found"
		c.data.DNSWarnings = append(c.data.DNSWarnings, "No DNS record found")

		fmt.Printf("  ⚠️  No DNS record found for %s\n", domain)
		fmt.Println()
		fmt.Println("  ┌─ What to do ───────────────────────────────────────────────┐")
		fmt.Println("  │                                                            │")
		fmt.Println("  │  Add an A record at your DNS provider:                     │")
		fmt.Println("  │                                                            │")
		fmt.Println("  │    Name:   " + extractSubdomain(domain) + padRight("", 43-len(extractSubdomain(domain))) + "│")
		fmt.Println("  │    Type:   A                                               │")
		fmt.Printf("  │    Value:  %-14s  ← this server                    │\n", c.data.ServerIP)
		fmt.Println("  │                                                            │")
		fmt.Println("  │  SSL will activate automatically once DNS propagates.      │")
		fmt.Println("  │                                                            │")
		fmt.Println("  └────────────────────────────────────────────────────────────┘")
		fmt.Println()
		return
	}

	// Check if domain resolves to server IP
	match, matchedIP := checkDomainIPMatch(domain, c.data.ServerIP)
	if match {
		fmt.Printf("  ✓ %s → %s (this server)\n", domain, matchedIP)
		fmt.Println()
		return
	}

	// DNS points elsewhere - simple one-line message
	c.data.DNSType = "wrong_ip"
	domainIP := formatIPs(ips)
	c.data.DNSWarnings = append(c.data.DNSWarnings, fmt.Sprintf("Points to %s instead of this server", domainIP))

	fmt.Printf("  ⚠️  %s → %s (different server)\n", domain, domainIP)
	fmt.Println()
	fmt.Printf("  Update your A record: %s → %s\n", domain, c.data.ServerIP)
	fmt.Println("  SSL will activate automatically once DNS propagates.")
	fmt.Println()
}

// extractSubdomain extracts the subdomain part from a domain
// e.g., "analytics.example.com" -> "analytics", "example.com" -> "@"
func extractSubdomain(domain string) string {
	parts := strings.Split(domain, ".")
	if len(parts) <= 2 {
		return "@"
	}
	return parts[0]
}

// padRight pads a string with spaces to reach the desired length
func padRight(s string, length int) string {
	if len(s) >= length {
		return s
	}
	return s + strings.Repeat(" ", length-len(s))
}

// GetDNSWarnings returns the current DNS warnings
func (c *Config) GetDNSWarnings() []string {
	return c.data.DNSWarnings
}

// HasDNSWarnings returns true if there are DNS configuration warnings
func (c *Config) HasDNSWarnings() bool {
	return len(c.data.DNSWarnings) > 0
}

// GetServerIP returns the server's public IP address
func (c *Config) GetServerIP() string {
	return c.data.ServerIP
}

// GetDNSType returns the type of DNS issue: "not_found", "wrong_ip", or ""
func (c *Config) GetDNSType() string {
	return c.data.DNSType
}

// generatePrivateKey generates a secure random private key
func generatePrivateKey() (string, error) {
	key := make([]byte, 16)
	_, err := rand.Read(key)
	if err != nil {
		return "", fmt.Errorf("failed to generate private key: %w", err)
	}
	return hex.EncodeToString(key), nil
}

// readPassword reads a password from either terminal or stdin based on environment
func (c *Config) readPassword(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)

	var passwordBytes []byte
	var err error

	// In test environment, read password from stdin instead of terminal
	if os.Getenv("ENV") == "test" {
		password, readErr := reader.ReadString('\n')
		if readErr != nil {
			err = readErr
		} else {
			passwordBytes = []byte(strings.TrimSpace(password))
		}
	} else {
		passwordBytes, err = term.ReadPassword(int(syscall.Stdin))
	}

	if err != nil {
		return "", fmt.Errorf("failed to read password: %w", err)
	}

	if os.Getenv("ENV") != "test" {
		fmt.Println() // Only add newline for terminal mode
	}

	return strings.TrimSpace(string(passwordBytes)), nil
}

// isLocalhostDomain checks if the domain is localhost or a localhost variant
func isLocalhostDomain(domain string) bool {
	// Check for common localhost variants
	localhostDomains := []string{
		"localhost",
		"127.0.0.1",
		"::1",
		"0.0.0.0",
		"localhost.localdomain",
	}

	domain = strings.ToLower(strings.TrimSpace(domain))
	for _, localhost := range localhostDomains {
		if domain == localhost {
			return true
		}
	}

	// Check for localhost with port (e.g., localhost:8080)
	if strings.HasPrefix(domain, "localhost:") {
		return true
	}

	// Check for localhost subdomains (e.g., app.localhost, test.localhost)
	if strings.HasSuffix(domain, ".localhost") {
		return true
	}

	return false
}

