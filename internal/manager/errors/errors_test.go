package errors

import (
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestValidationError(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		value    string
		message  string
		expected string
	}{
		{
			name:     "with value",
			field:    "email",
			value:    "invalid-email",
			message:  "invalid format",
			expected: "validation failed for field 'email' with value 'invalid-email': invalid format",
		},
		{
			name:     "without value",
			field:    "password",
			value:    "",
			message:  "too short",
			expected: "validation failed for field 'password': too short",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ValidationError{
				Field:   tt.field,
				Value:   tt.value,
				Message: tt.message,
			}
			if got := err.Error(); got != tt.expected {
				t.Errorf("ValidationError.Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestValidationErrorIs(t *testing.T) {
	validationErr := &ValidationError{
		Field:   "test",
		Value:   "value",
		Message: "message",
	}

	if !validationErr.Is(ErrInvalidInput) {
		t.Error("ValidationError should be identified as ErrInvalidInput")
	}

	if validationErr.Is(ErrTimeout) {
		t.Error("ValidationError should not be identified as ErrTimeout")
	}
}

func TestNetworkError(t *testing.T) {
	originalErr := fmt.Errorf("connection refused")
	netErr := &NetworkError{
		Operation: "GET",
		URL:       "https://example.com",
		Err:       originalErr,
	}

	expected := "network operation 'GET' failed for URL 'https://example.com': connection refused"
	if got := netErr.Error(); got != expected {
		t.Errorf("NetworkError.Error() = %v, want %v", got, expected)
	}

	if unwrapped := netErr.Unwrap(); unwrapped != originalErr {
		t.Errorf("NetworkError.Unwrap() = %v, want %v", unwrapped, originalErr)
	}
}

func TestDockerError(t *testing.T) {
	originalErr := fmt.Errorf("container not found")
	
	tests := []struct {
		name      string
		operation string
		container string
		err       error
		expected  string
	}{
		{
			name:      "with container",
			operation: "start",
			container: "my-app",
			err:       originalErr,
			expected:  "docker operation 'start' failed for container 'my-app': container not found",
		},
		{
			name:      "without container",
			operation: "pull",
			container: "",
			err:       originalErr,
			expected:  "docker operation 'pull' failed: container not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dockerErr := &DockerError{
				Operation: tt.operation,
				Container: tt.container,
				Err:       tt.err,
			}

			if got := dockerErr.Error(); got != tt.expected {
				t.Errorf("DockerError.Error() = %v, want %v", got, tt.expected)
			}

			if unwrapped := dockerErr.Unwrap(); unwrapped != originalErr {
				t.Errorf("DockerError.Unwrap() = %v, want %v", unwrapped, originalErr)
			}
		})
	}
}

func TestConfigError(t *testing.T) {
	tests := []struct {
		name     string
		field    string
		value    string
		message  string
		expected string
	}{
		{
			name:     "with value",
			field:    "domain",
			value:    "invalid.domain",
			message:  "invalid format",
			expected: "config error for field 'domain' with value 'invalid.domain': invalid format",
		},
		{
			name:     "without value",
			field:    "password",
			value:    "",
			message:  "cannot be empty",
			expected: "config error for field 'password': cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := &ConfigError{
				Field:   tt.field,
				Value:   tt.value,
				Message: tt.message,
			}
			if got := err.Error(); got != tt.expected {
				t.Errorf("ConfigError.Error() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestInstallationError(t *testing.T) {
	originalErr := fmt.Errorf("permission denied")
	installErr := &InstallationError{
		Component: "docker",
		Step:      "install",
		Err:       originalErr,
	}

	expected := "installation failed for component 'docker' at step 'install': permission denied"
	if got := installErr.Error(); got != expected {
		t.Errorf("InstallationError.Error() = %v, want %v", got, expected)
	}

	if unwrapped := installErr.Unwrap(); unwrapped != originalErr {
		t.Errorf("InstallationError.Unwrap() = %v, want %v", unwrapped, originalErr)
	}
}

func TestWrapWithContext(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		context  string
		expected string
	}{
		{
			name:     "with error",
			err:      fmt.Errorf("original error"),
			context:  "operation failed",
			expected: "operation failed: original error",
		},
		{
			name:     "with nil error",
			err:      nil,
			context:  "operation failed",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := WrapWithContext(tt.err, tt.context)
			if tt.err == nil {
				if result != nil {
					t.Errorf("WrapWithContext() with nil error should return nil, got %v", result)
				}
				return
			}

			if got := result.Error(); got != tt.expected {
				t.Errorf("WrapWithContext() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestRetryWithBackoff(t *testing.T) {
	t.Run("success on first try", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			return nil
		}

		err := RetryWithBackoff(operation, 3, 10*time.Millisecond)
		if err != nil {
			t.Errorf("RetryWithBackoff() should succeed on first try, got error: %v", err)
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("success on second try", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			if attempts == 1 {
				return fmt.Errorf("temporary error")
			}
			return nil
		}

		err := RetryWithBackoff(operation, 3, 10*time.Millisecond)
		if err != nil {
			t.Errorf("RetryWithBackoff() should succeed on second try, got error: %v", err)
		}
		if attempts != 2 {
			t.Errorf("Expected 2 attempts, got %d", attempts)
		}
	})

	t.Run("failure after max retries", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			return fmt.Errorf("persistent error")
		}

		err := RetryWithBackoff(operation, 3, 10*time.Millisecond)
		if err == nil {
			t.Error("RetryWithBackoff() should fail after max retries")
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}

		expectedMsg := "operation failed after 3 retries: persistent error"
		if got := err.Error(); got != expectedMsg {
			t.Errorf("Error message = %v, want %v", got, expectedMsg)
		}
	})

	t.Run("invalid max retries", func(t *testing.T) {
		operation := func() error {
			return nil
		}

		err := RetryWithBackoff(operation, 0, 10*time.Millisecond)
		if err == nil {
			t.Error("RetryWithBackoff() should fail with invalid maxRetries")
		}

		expectedMsg := "maxRetries must be greater than 0"
		if got := err.Error(); got != expectedMsg {
			t.Errorf("Error message = %v, want %v", got, expectedMsg)
		}
	})
}

func TestSafeExecute(t *testing.T) {
	t.Run("successful execution", func(t *testing.T) {
		fn := func() error {
			return nil
		}

		err := SafeExecute(fn)
		if err != nil {
			t.Errorf("SafeExecute() should succeed, got error: %v", err)
		}
	})

	t.Run("function returns error", func(t *testing.T) {
		expectedErr := fmt.Errorf("function error")
		fn := func() error {
			return expectedErr
		}

		err := SafeExecute(fn)
		if err != expectedErr {
			t.Errorf("SafeExecute() should return function error, got: %v", err)
		}
	})

	t.Run("function panics", func(t *testing.T) {
		fn := func() error {
			panic("test panic")
		}

		err := SafeExecute(fn)
		if err == nil {
			t.Error("SafeExecute() should capture panic")
		}

		expectedMsg := "panic recovered: test panic"
		if got := err.Error(); got != expectedMsg {
			t.Errorf("Error message = %v, want %v", got, expectedMsg)
		}
	})
}

func TestIsTemporary(t *testing.T) {
	t.Run("network error is temporary", func(t *testing.T) {
		netErr := &NetworkError{
			Operation: "GET",
			URL:       "http://example.com",
			Err:       fmt.Errorf("connection refused"),
		}

		if !IsTemporary(netErr) {
			t.Error("NetworkError should be considered temporary")
		}
	})

	t.Run("regular error is not temporary", func(t *testing.T) {
		err := fmt.Errorf("regular error")

		if IsTemporary(err) {
			t.Error("Regular error should not be considered temporary")
		}
	})
}

func TestErrorConstructors(t *testing.T) {
	t.Run("NewValidationError", func(t *testing.T) {
		err := NewValidationError("email", "test@example", "invalid format")
		if err.Field != "email" || err.Value != "test@example" || err.Message != "invalid format" {
			t.Error("NewValidationError did not set fields correctly")
		}
	})

	t.Run("NewNetworkError", func(t *testing.T) {
		originalErr := fmt.Errorf("connection error")
		err := NewNetworkError("POST", "http://api.com", originalErr)
		if err.Operation != "POST" || err.URL != "http://api.com" || err.Err != originalErr {
			t.Error("NewNetworkError did not set fields correctly")
		}
	})

	t.Run("NewDockerError", func(t *testing.T) {
		originalErr := fmt.Errorf("docker error")
		err := NewDockerError("run", "my-container", originalErr)
		if err.Operation != "run" || err.Container != "my-container" || err.Err != originalErr {
			t.Error("NewDockerError did not set fields correctly")
		}
	})

	t.Run("NewConfigError", func(t *testing.T) {
		err := NewConfigError("domain", "example.com", "invalid")
		if err.Field != "domain" || err.Value != "example.com" || err.Message != "invalid" {
			t.Error("NewConfigError did not set fields correctly")
		}
	})

	t.Run("NewInstallationError", func(t *testing.T) {
		originalErr := fmt.Errorf("install error")
		err := NewInstallationError("docker", "download", originalErr)
		if err.Component != "docker" || err.Step != "download" || err.Err != originalErr {
			t.Error("NewInstallationError did not set fields correctly")
		}
	})
}

func TestErrorsAs(t *testing.T) {
	validationErr := NewValidationError("test", "value", "invalid")
	
	var target *ValidationError
	if !errors.As(validationErr, &target) {
		t.Error("ValidationError should be unwrappable with errors.As")
	}
	
	if target.Field != "test" {
		t.Errorf("Unwrapped error field = %v, want %v", target.Field, "test")
	}
}