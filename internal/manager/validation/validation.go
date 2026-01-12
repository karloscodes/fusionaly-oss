package validation

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"fusionaly/internal/manager/errors"
)

var (
	emailRegex  = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	domainRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?)*$`)
)

// ValidateEmail validates email format and returns appropriate error
func ValidateEmail(email string) error {
	if email == "" {
		return errors.NewValidationError("email", email, "email cannot be empty")
	}

	if len(email) > 254 {
		return errors.NewValidationError("email", email, "email too long (max 254 characters)")
	}

	if !emailRegex.MatchString(email) {
		return errors.NewValidationError("email", email, "invalid email format")
	}

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return errors.NewValidationError("email", email, "email must contain exactly one @ symbol")
	}

	localPart, domain := parts[0], parts[1]
	if len(localPart) > 64 {
		return errors.NewValidationError("email", email, "local part too long (max 64 characters)")
	}

	if err := ValidateDomain(domain); err != nil {
		return errors.NewValidationError("email", email, fmt.Sprintf("invalid domain in email: %v", err))
	}

	return nil
}

// ValidateDomain validates domain name format
func ValidateDomain(domain string) error {
	if domain == "" {
		return errors.NewValidationError("domain", domain, "domain cannot be empty")
	}

	if len(domain) > 253 {
		return errors.NewValidationError("domain", domain, "domain too long (max 253 characters)")
	}

	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return errors.NewValidationError("domain", domain, "domain cannot start or end with a dot")
	}

	if strings.Contains(domain, "..") {
		return errors.NewValidationError("domain", domain, "domain cannot contain consecutive dots")
	}

	if !domainRegex.MatchString(domain) {
		return errors.NewValidationError("domain", domain, "invalid domain format")
	}

	parts := strings.Split(domain, ".")
	for _, part := range parts {
		if len(part) == 0 {
			return errors.NewValidationError("domain", domain, "domain cannot contain empty labels")
		}
		if len(part) > 63 {
			return errors.NewValidationError("domain", domain, "domain label too long (max 63 characters)")
		}
	}

	return nil
}

// ValidatePort validates port number
func ValidatePort(port string) error {
	if port == "" {
		return errors.NewValidationError("port", port, "port cannot be empty")
	}

	portNum, err := strconv.Atoi(port)
	if err != nil {
		return errors.NewValidationError("port", port, "port must be a valid integer")
	}

	if portNum < 1 || portNum > 65535 {
		return errors.NewValidationError("port", port, "port must be between 1 and 65535")
	}

	return nil
}

// ValidateURL validates URL format
func ValidateURL(rawURL string) error {
	if rawURL == "" {
		return errors.NewValidationError("url", rawURL, "URL cannot be empty")
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return errors.NewValidationError("url", rawURL, fmt.Sprintf("invalid URL format: %v", err))
	}

	if parsedURL.Scheme == "" {
		return errors.NewValidationError("url", rawURL, "URL must include a scheme (http/https)")
	}

	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return errors.NewValidationError("url", rawURL, "URL scheme must be http or https")
	}

	if parsedURL.Host == "" {
		return errors.NewValidationError("url", rawURL, "URL must include a host")
	}

	return nil
}

// ValidateIPAddress validates IP address format
func ValidateIPAddress(ip string) error {
	if ip == "" {
		return errors.NewValidationError("ip", ip, "IP address cannot be empty")
	}

	if net.ParseIP(ip) == nil {
		return errors.NewValidationError("ip", ip, "invalid IP address format")
	}

	return nil
}

// ValidateLicenseKey validates license key format (basic validation)
func ValidateLicenseKey(license string) error {
	if license == "" {
		return errors.NewValidationError("license", license, "license key cannot be empty")
	}

	if len(license) < 10 {
		return errors.NewValidationError("license", license, "license key too short (minimum 10 characters)")
	}

	if len(license) > 100 {
		return errors.NewValidationError("license", license, "license key too long (maximum 100 characters)")
	}

	// Basic format validation - alphanumeric and common separators
	validChars := regexp.MustCompile(`^[a-zA-Z0-9\-_\.]+$`)
	if !validChars.MatchString(license) {
		return errors.NewValidationError("license", license, "license key contains invalid characters")
	}

	return nil
}

// ValidatePassword validates password strength
func ValidatePassword(password string) error {
	if password == "" {
		return errors.NewValidationError("password", "", "password cannot be empty")
	}

	if len(password) < 8 {
		return errors.NewValidationError("password", "", "password must be at least 8 characters long")
	}

	if len(password) > 128 {
		return errors.NewValidationError("password", "", "password too long (maximum 128 characters)")
	}

	return nil
}

// ValidateContainerName validates Docker container name
func ValidateContainerName(name string) error {
	if name == "" {
		return errors.NewValidationError("container_name", name, "container name cannot be empty")
	}

	if len(name) > 63 {
		return errors.NewValidationError("container_name", name, "container name too long (max 63 characters)")
	}

	// Docker container name validation - must start with alphanumeric or underscore
	validName := regexp.MustCompile(`^[a-zA-Z0-9_][a-zA-Z0-9_.-]*$`)
	if !validName.MatchString(name) {
		return errors.NewValidationError("container_name", name, "container name must start with alphanumeric or underscore and contain only alphanumeric, underscore, period, or hyphen")
	}

	return nil
}

// ValidateVersion validates semantic version format
func ValidateVersion(version string) error {
	if version == "" {
		return errors.NewValidationError("version", version, "version cannot be empty")
	}

	// Allow "latest" as a special case
	if version == "latest" {
		return nil
	}

	// Basic semantic version validation (major.minor.patch)
	versionRegex := regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?(?:\+([0-9A-Za-z-]+(?:\.[0-9A-Za-z-]+)*))?$`)
	if !versionRegex.MatchString(version) {
		return errors.NewValidationError("version", version, "version must follow semantic versioning format (e.g., v1.2.3)")
	}

	return nil
}

// ValidateFilePath validates file path format
func ValidateFilePath(path string) error {
	if path == "" {
		return errors.NewValidationError("file_path", path, "file path cannot be empty")
	}

	if len(path) > 4096 {
		return errors.NewValidationError("file_path", path, "file path too long (max 4096 characters)")
	}

	// Check for invalid characters in file path (excluding Windows drive letters)
	// Allow colons only for Windows drive letters (e.g., C:)
	if len(path) >= 2 && path[1] == ':' {
		// Windows path - check remaining characters
		remainingPath := path[2:]
		invalidChars := regexp.MustCompile(`[<>"|?*]`)
		if invalidChars.MatchString(remainingPath) {
			return errors.NewValidationError("file_path", path, "file path contains invalid characters")
		}
	} else {
		// Unix path - check all characters
		invalidChars := regexp.MustCompile(`[<>:"|?*]`)
		if invalidChars.MatchString(path) {
			return errors.NewValidationError("file_path", path, "file path contains invalid characters")
		}
	}

	return nil
}
