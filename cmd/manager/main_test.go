package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateEmail(t *testing.T) {
	tests := []struct {
		name    string
		email   string
		wantErr bool
	}{
		{"valid email", "test@example.com", false},
		{"valid with subdomain", "user@mail.example.com", false},
		{"valid with plus", "user+tag@example.com", false},
		{"empty", "", true},
		{"no at sign", "testexample.com", true},
		{"no domain", "test@", true},
		{"no tld", "test@example", true},
		{"spaces", "test @example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEmail(tt.email)

			if (err != nil) != tt.wantErr {
				t.Errorf("validateEmail(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
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
		{"valid 8 chars", "12345678", false},
		{"valid longer", "securepassword123", false},
		{"too short", "1234567", true},
		{"empty", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validatePassword(tt.password)

			if (err != nil) != tt.wantErr {
				t.Errorf("validatePassword(%q) error = %v, wantErr %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestNewMatcha(t *testing.T) {
	m := newMatcha()

	if m == nil {
		t.Fatal("newMatcha() returned nil")
	}

	// Verify matcha is configured for fusionaly
	prefix := m.EnvPrefix()
	if prefix != "FUSIONALY" {
		t.Errorf("EnvPrefix() = %q, want 'FUSIONALY'", prefix)
	}
}

func TestBuildUpdateCron(t *testing.T) {
	cron := buildUpdateCron("/usr/local/bin/fusionaly", "/var/matcha/fusionaly/logs")

	// Creates its log dir inline so a missing dir can't make the redirect fail
	// before fusionaly runs — the bug that silently stopped nightly updates.
	if !strings.Contains(cron, "mkdir -p /var/matcha/fusionaly/logs &&") {
		t.Errorf("cron must create its log dir before running; got:\n%s", cron)
	}
	// Runs the updater daily at 3 AM as root.
	if !strings.Contains(cron, "0 3 * * * root mkdir -p /var/matcha/fusionaly/logs && /usr/local/bin/fusionaly update") {
		t.Errorf("cron must run the binary daily at 3 AM as root; got:\n%s", cron)
	}
	// Appends (keeps history) to update.log inside the log dir.
	if !strings.Contains(cron, ">> /var/matcha/fusionaly/logs/update.log 2>&1") {
		t.Errorf("cron must append to update.log; got:\n%s", cron)
	}
	// cron.d ignores a file whose last line lacks a trailing newline.
	if !strings.HasSuffix(cron, "\n") {
		t.Errorf("cron file must end with a newline; got: %q", cron)
	}
}

func TestRepairCronFile(t *testing.T) {
	binPath := "/usr/local/bin/fusionaly"
	logDir := "/var/matcha/fusionaly/logs"
	desired := buildUpdateCron(binPath, logDir)

	t.Run("writes when the file is missing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "fusionaly-update")

		wrote, err := repairCronFile(path, binPath, logDir)

		if err != nil {
			t.Fatalf("repairCronFile() error = %v", err)
		}
		if !wrote {
			t.Error("expected wrote = true for a missing file")
		}
		got, _ := os.ReadFile(path)
		if string(got) != desired {
			t.Errorf("written content =\n%s\nwant\n%s", got, desired)
		}
	})

	t.Run("replaces a stale legacy cron", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "fusionaly-update")
		legacy := "0 3 * * * root cd /opt/fusionaly && /usr/local/bin/fusionaly update > /opt/fusionaly/logs/updater.log 2>&1\n"
		os.WriteFile(path, []byte(legacy), 0644)

		wrote, err := repairCronFile(path, binPath, logDir)

		if err != nil {
			t.Fatalf("repairCronFile() error = %v", err)
		}
		if !wrote {
			t.Error("expected wrote = true when replacing legacy content")
		}
		got, _ := os.ReadFile(path)
		if string(got) != desired {
			t.Errorf("legacy cron not repaired; got:\n%s", got)
		}
	})

	t.Run("is idempotent when already healthy", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "fusionaly-update")
		os.WriteFile(path, []byte(desired), 0644)

		wrote, err := repairCronFile(path, binPath, logDir)

		if err != nil {
			t.Fatalf("repairCronFile() error = %v", err)
		}
		if wrote {
			t.Error("expected wrote = false when content already matches")
		}
	})
}
