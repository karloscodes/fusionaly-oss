package main

import (
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
