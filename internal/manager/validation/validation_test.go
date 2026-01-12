package validation

import (
	"errors"
	"testing"

	customerrors "fusionaly/internal/manager/errors"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "test@example.com", false},
		{"valid email with subdomain", "user@mail.example.com", false},
		{"valid email with numbers", "user123@example123.com", false},
		{"valid email with special chars", "user.name+tag@example.com", false},
		{"empty email", "", true},
		{"missing @", "testexample.com", true},
		{"missing domain", "test@", true},
		{"missing local part", "@example.com", true},
		{"invalid domain", "test@", true},
		{"too long email", "a" + string(make([]byte, 250)) + "@example.com", true},
		{"too long local part", string(make([]byte, 65)) + "@example.com", true},
		{"multiple @", "test@@example.com", true},
		{"domain with consecutive dots", "test@example..com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEmail(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEmail() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err != nil {
				var validationErr *customerrors.ValidationError
				if !errors.As(err, &validationErr) {
					t.Errorf("Expected ValidationError, got %T", err)
				}
			}
		})
	}
}

func TestValidateDomain(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		wantErr bool
	}{
		{"valid domain", "example.com", false},
		{"valid subdomain", "sub.example.com", false},
		{"valid single label", "localhost", false},
		{"valid with numbers", "example123.com", false},
		{"valid with hyphens", "my-site.example.com", false},
		{"empty domain", "", true},
		{"too long domain", string(make([]byte, 254)), true},
		{"starts with dot", ".example.com", true},
		{"ends with dot", "example.com.", true},
		{"consecutive dots", "example..com", true},
		{"invalid characters", "example$.com", true},
		{"label too long", string(make([]byte, 64)) + ".com", true},
		{"starts with hyphen", "-example.com", true},
		{"ends with hyphen", "example-.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDomain(tt.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateDomain() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePort(t *testing.T) {
	tests := []struct {
		name    string
		port    string
		wantErr bool
	}{
		{"valid port 80", "80", false},
		{"valid port 443", "443", false},
		{"valid port 8080", "8080", false},
		{"valid port 65535", "65535", false},
		{"valid port 1", "1", false},
		{"empty port", "", true},
		{"port 0", "0", true},
		{"port too high", "65536", true},
		{"negative port", "-1", true},
		{"non-numeric port", "abc", true},
		{"decimal port", "80.5", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePort(tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePort() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid http URL", "http://example.com", false},
		{"valid https URL", "https://example.com", false},
		{"valid URL with path", "https://example.com/path", false},
		{"valid URL with query", "https://example.com?query=value", false},
		{"valid URL with port", "https://example.com:8080", false},
		{"empty URL", "", true},
		{"no scheme", "example.com", true},
		{"invalid scheme", "ftp://example.com", true},
		{"no host", "https://", true},
		{"malformed URL", "https:////example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateURL(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateURL() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateIPAddress(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		wantErr bool
	}{
		{"valid IPv4", "192.168.1.1", false},
		{"valid IPv4 localhost", "127.0.0.1", false},
		{"valid IPv6", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", false},
		{"valid IPv6 short", "::1", false},
		{"empty IP", "", true},
		{"invalid IPv4", "192.168.1.256", true},
		{"invalid IPv4 format", "192.168.1", true},
		{"invalid IPv6", "2001:0db8:85a3::8a2e::7334", true},
		{"not an IP", "example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIPAddress(tt.ip)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateIPAddress() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateLicenseKey(t *testing.T) {
	tests := []struct {
		name    string
		license string
		wantErr bool
	}{
		{"valid license", "ABC123DEF456", false},
		{"valid with hyphens", "ABC-123-DEF-456", false},
		{"valid with underscores", "ABC_123_DEF_456", false},
		{"valid with dots", "ABC.123.DEF.456", false},
		{"valid long license", "ABCDEFGHIJ1234567890ABCDEFGHIJ", false},
		{"empty license", "", true},
		{"too short", "ABC123", true},
		{"too long", string(make([]byte, 101)), true},
		{"invalid characters", "ABC@123#DEF", true},
		{"spaces", "ABC 123 DEF", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateLicenseKey(tt.license)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateLicenseKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"valid strong password", "StrongPass123!", false},
		{"valid with all types", "Aa1!Pass", false},
		{"valid minimum 8 chars", "Password1", false},
		{"valid simple numbers", "12345678", false},
		{"valid only lowercase", "password", false},
		{"valid only uppercase", "PASSWORD", false},
		{"valid only special", "!@#$%^&*", false},
		{"valid letters only", "PasswordOnly", false},
		{"valid letters and numbers", "password123", false},
		{"empty password", "", true},
		{"too short", "Pass1!", true},
		{"too long", string(make([]byte, 129)), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePassword(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePassword() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateContainerName(t *testing.T) {
	tests := []struct {
		name          string
		containerName string
		wantErr       bool
	}{
		{"valid name", "my-container", false},
		{"valid with numbers", "container123", false},
		{"valid with underscore", "my_container", false},
		{"valid with period", "my.container", false},
		{"valid single char", "a", false},
		{"empty name", "", true},
		{"too long", string(make([]byte, 64)), true},
		{"starts with hyphen", "-container", true},
		{"starts with period", ".container", true},
		{"starts with underscore", "_container", false}, // This should be valid
		{"invalid characters", "my@container", true},
		{"spaces", "my container", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateContainerName(tt.containerName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateContainerName() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		wantErr bool
	}{
		{"valid semantic version", "1.2.3", false},
		{"valid with v prefix", "v1.2.3", false},
		{"valid with prerelease", "1.2.3-alpha", false},
		{"valid with build metadata", "1.2.3+build.1", false},
		{"valid complex", "v1.2.3-alpha.1+build.123", false},
		{"valid major only", "1.0.0", false},
		{"empty version", "", true},
		{"invalid format", "1.2", true},
		{"invalid format", "1", true},
		{"invalid characters", "1.2.3a", true},
		{"double v prefix", "vv1.2.3", true},
		{"negative numbers", "-1.2.3", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateVersion(tt.version)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateFilePath(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		wantErr  bool
	}{
		{"valid unix path", "/usr/local/bin", false},
		{"valid relative path", "path/to/file", false},
		{"valid windows path", "C:\\Program Files\\App", false},
		{"valid with spaces", "/path with spaces/file", false},
		{"empty path", "", true},
		{"too long path", "/" + string(make([]byte, 4096)), true},
		{"invalid characters", "/path<>file", true},
		{"invalid pipe", "/path|file", true},
		{"invalid question", "/path?file", true},
		{"invalid asterisk", "/path*file", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateFilePath(tt.filePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilePath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidationErrorFields(t *testing.T) {
	err := ValidateEmail("invalid-email")
	if err == nil {
		t.Fatal("Expected validation error for invalid email")
	}

	var validationErr *customerrors.ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatal("Expected ValidationError type")
	}

	if validationErr.Field != "email" {
		t.Errorf("Expected field 'email', got '%s'", validationErr.Field)
	}

	if validationErr.Value != "invalid-email" {
		t.Errorf("Expected value 'invalid-email', got '%s'", validationErr.Value)
	}

	if validationErr.Message == "" {
		t.Error("Expected non-empty message")
	}
}

func TestEmailValidationFlow(t *testing.T) {
	t.Run("AcceptValidBusinessEmail", func(t *testing.T) {
		email := "admin@company.com"
		err := ValidateEmail(email)
		
		if err != nil {
			t.Errorf("Expected valid email to be accepted, got error: %v", err)
		}
	})

	t.Run("RejectMalformedEmail", func(t *testing.T) {
		email := "invalid-email"
		err := ValidateEmail(email)
		
		if err == nil {
			t.Error("Expected malformed email to be rejected")
		}
		
		var validationErr *customerrors.ValidationError
		if !errors.As(err, &validationErr) {
			t.Errorf("Expected ValidationError, got %T", err)
		}
	})

	t.Run("RejectEmptyEmail", func(t *testing.T) {
		email := ""
		err := ValidateEmail(email)
		
		if err == nil {
			t.Error("Expected empty email to be rejected")
		}
	})
}

func TestDomainValidationFlow(t *testing.T) {
	t.Run("AcceptValidDomain", func(t *testing.T) {
		domain := "metrics.company.com"
		err := ValidateDomain(domain)
		
		if err != nil {
			t.Errorf("Expected valid domain to be accepted, got error: %v", err)
		}
	})

	t.Run("RejectInvalidDomain", func(t *testing.T) {
		domain := "invalid..domain"
		err := ValidateDomain(domain)
		
		if err == nil {
			t.Error("Expected invalid domain to be rejected")
		}
	})
}

func TestPasswordValidationFlow(t *testing.T) {
	t.Run("AcceptStrongPassword", func(t *testing.T) {
		password := "SecurePassword123!"
		err := ValidatePassword(password)
		
		if err != nil {
			t.Errorf("Expected strong password to be accepted, got error: %v", err)
		}
	})

	t.Run("RejectWeakPassword", func(t *testing.T) {
		password := "123"
		err := ValidatePassword(password)
		
		if err == nil {
			t.Error("Expected weak password to be rejected for security")
		}
	})

	t.Run("RejectEmptyPassword", func(t *testing.T) {
		password := ""
		err := ValidatePassword(password)
		
		if err == nil {
			t.Error("Expected empty password to be rejected as required")
		}
	})
}