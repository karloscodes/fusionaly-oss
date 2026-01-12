package utils

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	customerrors "fusionaly/internal/manager/errors"
	"fusionaly/internal/manager/logging"
)

func createTestLogger() *logging.Logger {
	return logging.NewLogger(logging.Config{Level: "error"}) // Reduce noise in tests
}

func TestEnsureDirectoryExists(t *testing.T) {
	logger := createTestLogger()
	tempDir := t.TempDir()

	t.Run("create new directory", func(t *testing.T) {
		newDir := filepath.Join(tempDir, "new_dir")
		err := EnsureDirectoryExists(logger, newDir, 0755)
		if err != nil {
			t.Errorf("EnsureDirectoryExists() error = %v", err)
		}

		// Verify directory was created
		info, err := os.Stat(newDir)
		if err != nil {
			t.Errorf("Directory was not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("Created path is not a directory")
		}
	})

	t.Run("directory already exists", func(t *testing.T) {
		existingDir := filepath.Join(tempDir, "existing_dir")
		os.MkdirAll(existingDir, 0755)

		err := EnsureDirectoryExists(logger, existingDir, 0755)
		if err != nil {
			t.Errorf("EnsureDirectoryExists() should not error for existing directory: %v", err)
		}
	})

	t.Run("path exists but is file", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "existing_file")
		os.WriteFile(filePath, []byte("content"), 0644)

		err := EnsureDirectoryExists(logger, filePath, 0755)
		if err == nil {
			t.Error("EnsureDirectoryExists() should error when path is a file")
		}
	})

	t.Run("empty path", func(t *testing.T) {
		err := EnsureDirectoryExists(logger, "", 0755)
		if err == nil {
			t.Error("EnsureDirectoryExists() should error for empty path")
		}

		var validationErr *customerrors.ValidationError
		if !errors.As(err, &validationErr) {
			t.Errorf("Expected ValidationError, got %T", err)
		}
	})
}

func TestSafeFileWrite(t *testing.T) {
	logger := createTestLogger()
	tempDir := t.TempDir()

	t.Run("write new file", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "test_file.txt")
		content := []byte("test content")

		err := SafeFileWrite(logger, filePath, content, 0644)
		if err != nil {
			t.Errorf("SafeFileWrite() error = %v", err)
		}

		// Verify file was written
		readContent, err := os.ReadFile(filePath)
		if err != nil {
			t.Errorf("Failed to read written file: %v", err)
		}
		if string(readContent) != string(content) {
			t.Errorf("File content mismatch. Expected %s, got %s", content, readContent)
		}
	})

	t.Run("write to subdirectory", func(t *testing.T) {
		filePath := filepath.Join(tempDir, "subdir", "test_file.txt")
		content := []byte("test content")

		err := SafeFileWrite(logger, filePath, content, 0644)
		if err != nil {
			t.Errorf("SafeFileWrite() error = %v", err)
		}

		// Verify file and directory were created
		if _, err := os.Stat(filePath); err != nil {
			t.Errorf("File was not created: %v", err)
		}
	})

	t.Run("empty file path", func(t *testing.T) {
		err := SafeFileWrite(logger, "", []byte("content"), 0644)
		if err == nil {
			t.Error("SafeFileWrite() should error for empty path")
		}

		var validationErr *customerrors.ValidationError
		if !errors.As(err, &validationErr) {
			t.Errorf("Expected ValidationError, got %T", err)
		}
	})
}

func TestBackupFile(t *testing.T) {
	logger := createTestLogger()
	tempDir := t.TempDir()

	t.Run("backup existing file", func(t *testing.T) {
		originalFile := filepath.Join(tempDir, "original.txt")
		originalContent := []byte("original content")
		os.WriteFile(originalFile, originalContent, 0644)

		backupPath, err := BackupFile(logger, originalFile)
		if err != nil {
			t.Errorf("BackupFile() error = %v", err)
		}

		if backupPath == "" {
			t.Error("BackupFile() should return backup path")
		}

		// Verify backup file exists and has same content
		backupContent, err := os.ReadFile(backupPath)
		if err != nil {
			t.Errorf("Failed to read backup file: %v", err)
		}
		if string(backupContent) != string(originalContent) {
			t.Errorf("Backup content mismatch. Expected %s, got %s", originalContent, backupContent)
		}
	})

	t.Run("backup non-existent file", func(t *testing.T) {
		nonExistentFile := filepath.Join(tempDir, "non_existent.txt")

		backupPath, err := BackupFile(logger, nonExistentFile)
		if err != nil {
			t.Errorf("BackupFile() should not error for non-existent file: %v", err)
		}

		if backupPath != "" {
			t.Error("BackupFile() should return empty path for non-existent file")
		}
	})

	t.Run("empty file path", func(t *testing.T) {
		_, err := BackupFile(logger, "")
		if err == nil {
			t.Error("BackupFile() should error for empty path")
		}

		var validationErr *customerrors.ValidationError
		if !errors.As(err, &validationErr) {
			t.Errorf("Expected ValidationError, got %T", err)
		}
	})
}

func TestFileOperationWithRetry(t *testing.T) {
	logger := createTestLogger()

	t.Run("success on first try", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			return nil
		}

		err := FileOperationWithRetry(logger, "test", "/tmp/test", operation)
		if err != nil {
			t.Errorf("FileOperationWithRetry() should succeed: %v", err)
		}
		if attempts != 1 {
			t.Errorf("Expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("success after retries", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			if attempts < 3 {
				return fmt.Errorf("temporary error")
			}
			return nil
		}

		err := FileOperationWithRetry(logger, "test", "/tmp/test", operation)
		if err != nil {
			t.Errorf("FileOperationWithRetry() should succeed after retries: %v", err)
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})

	t.Run("failure after max retries", func(t *testing.T) {
		attempts := 0
		operation := func() error {
			attempts++
			return fmt.Errorf("persistent error")
		}

		err := FileOperationWithRetry(logger, "test", "/tmp/test", operation)
		if err == nil {
			t.Error("FileOperationWithRetry() should fail after max retries")
		}
		if attempts != 3 {
			t.Errorf("Expected 3 attempts, got %d", attempts)
		}
	})
}

func TestValidateAndExecute(t *testing.T) {
	logger := createTestLogger()

	t.Run("all validations pass and execution succeeds", func(t *testing.T) {
		validators := []func() error{
			func() error { return nil },
			func() error { return nil },
		}
		executor := func() error { return nil }

		err := ValidateAndExecute(logger, "test", validators, executor)
		if err != nil {
			t.Errorf("ValidateAndExecute() should succeed: %v", err)
		}
	})

	t.Run("validation fails", func(t *testing.T) {
		validators := []func() error{
			func() error { return nil },
			func() error { return fmt.Errorf("validation error") },
		}
		executor := func() error { return nil }

		err := ValidateAndExecute(logger, "test", validators, executor)
		if err == nil {
			t.Error("ValidateAndExecute() should fail when validation fails")
		}
	})

	t.Run("execution fails", func(t *testing.T) {
		validators := []func() error{
			func() error { return nil },
		}
		executor := func() error { return fmt.Errorf("execution error") }

		err := ValidateAndExecute(logger, "test", validators, executor)
		if err == nil {
			t.Error("ValidateAndExecute() should fail when execution fails")
		}
	})
}

func TestLogAndWrapError(t *testing.T) {
	logger := createTestLogger()

	t.Run("with error", func(t *testing.T) {
		originalErr := fmt.Errorf("original error")
		wrappedErr := LogAndWrapError(logger, originalErr, "operation failed")

		if wrappedErr == nil {
			t.Error("LogAndWrapError() should return error when input is error")
		}

		if wrappedErr.Error() != "operation failed: original error" {
			t.Errorf("Unexpected wrapped error message: %v", wrappedErr)
		}
	})

	t.Run("with nil error", func(t *testing.T) {
		wrappedErr := LogAndWrapError(logger, nil, "operation failed")
		if wrappedErr != nil {
			t.Error("LogAndWrapError() should return nil when input is nil")
		}
	})
}

func TestFirstError(t *testing.T) {
	err1 := fmt.Errorf("first error")
	err2 := fmt.Errorf("second error")

	tests := []struct {
		name     string
		errors   []error
		expected error
	}{
		{"no errors", []error{nil, nil}, nil},
		{"first error", []error{err1, err2}, err1},
		{"second error", []error{nil, err2}, err2},
		{"all errors", []error{err1, err2}, err1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FirstError(tt.errors...)
			if result != tt.expected {
				t.Errorf("FirstError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestAggregateErrors(t *testing.T) {
	err1 := fmt.Errorf("first error")
	err2 := fmt.Errorf("second error")

	tests := []struct {
		name      string
		operation string
		errors    []error
		wantNil   bool
	}{
		{"no errors", "test", []error{nil, nil}, true},
		{"single error", "test", []error{err1}, false},
		{"multiple errors", "test", []error{err1, err2}, false},
		{"mixed errors", "test", []error{nil, err1, nil, err2}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := AggregateErrors(tt.operation, tt.errors...)
			if (result == nil) != tt.wantNil {
				t.Errorf("AggregateErrors() = %v, wantNil %v", result, tt.wantNil)
			}
		})
	}
}

func TestTimeoutOperation(t *testing.T) {
	logger := createTestLogger()

	t.Run("operation completes before timeout", func(t *testing.T) {
		operation := func() error {
			time.Sleep(10 * time.Millisecond)
			return nil
		}

		err := TimeoutOperation(logger, "test", 100*time.Millisecond, operation)
		if err != nil {
			t.Errorf("TimeoutOperation() should succeed: %v", err)
		}
	})

	t.Run("operation times out", func(t *testing.T) {
		operation := func() error {
			time.Sleep(200 * time.Millisecond)
			return nil
		}

		err := TimeoutOperation(logger, "test", 50*time.Millisecond, operation)
		if err == nil {
			t.Error("TimeoutOperation() should timeout")
		}

		if !contains(err.Error(), "timed out") {
			t.Errorf("Expected timeout error, got: %v", err)
		}
	})

	t.Run("operation panics", func(t *testing.T) {
		operation := func() error {
			panic("test panic")
		}

		err := TimeoutOperation(logger, "test", 100*time.Millisecond, operation)
		if err == nil {
			t.Error("TimeoutOperation() should handle panic")
		}

		if !contains(err.Error(), "panic") {
			t.Errorf("Expected panic error, got: %v", err)
		}
	})
}

func TestConditionalError(t *testing.T) {
	testErr := fmt.Errorf("test error")

	tests := []struct {
		name      string
		condition bool
		err       error
		expected  error
	}{
		{"condition true", true, testErr, testErr},
		{"condition false", false, testErr, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ConditionalError(tt.condition, tt.err)
			if result != tt.expected {
				t.Errorf("ConditionalError() = %v, want %v", result, tt.expected)
			}
		})
	}
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && s[:len(substr)] == substr) ||
		(len(s) > len(substr) && s[len(s)-len(substr):] == substr) ||
		containsAt(s, substr))
}

func containsAt(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}