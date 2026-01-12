package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"fusionaly/internal/manager/errors"
	"fusionaly/internal/manager/logging"
)

// FileOperationWithRetry performs a file operation with retry logic
func FileOperationWithRetry(logger *logging.Logger, operation string, filePath string, fn func() error) error {
	return errors.RetryWithBackoff(func() error {
		if err := fn(); err != nil {
			logger.Debug("File operation '%s' failed for %s: %v", operation, filePath, err)
			return errors.WrapWithContext(err, fmt.Sprintf("file operation '%s' on %s", operation, filePath))
		}
		return nil
	}, 3, 100*time.Millisecond)
}

// EnsureDirectoryExists creates a directory if it doesn't exist, with proper error handling
func EnsureDirectoryExists(logger *logging.Logger, dirPath string, perm os.FileMode) error {
	if dirPath == "" {
		return errors.NewValidationError("directory_path", dirPath, "directory path cannot be empty")
	}

	// Check if directory already exists
	if info, err := os.Stat(dirPath); err == nil {
		if !info.IsDir() {
			return fmt.Errorf("path exists but is not a directory: %s", dirPath)
		}
		logger.Debug("Directory already exists: %s", dirPath)
		return nil
	}

	// Create directory with all parent directories
	if err := os.MkdirAll(dirPath, perm); err != nil {
		return errors.WrapWithContext(err, fmt.Sprintf("failed to create directory %s", dirPath))
	}

	logger.Debug("Created directory: %s", dirPath)
	return nil
}

// SafeFileWrite writes content to a file safely with backup and atomic operations
func SafeFileWrite(logger *logging.Logger, filePath string, content []byte, perm os.FileMode) error {
	if filePath == "" {
		return errors.NewValidationError("file_path", filePath, "file path cannot be empty")
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(filePath)
	if err := EnsureDirectoryExists(logger, parentDir, 0755); err != nil {
		return errors.WrapWithContext(err, "failed to ensure parent directory exists")
	}

	// Create temporary file for atomic write
	tempFile := filePath + ".tmp"
	
	// Clean up temp file on error
	defer func() {
		if _, err := os.Stat(tempFile); err == nil {
			os.Remove(tempFile)
		}
	}()

	// Write to temporary file first
	if err := os.WriteFile(tempFile, content, perm); err != nil {
		return errors.WrapWithContext(err, fmt.Sprintf("failed to write temporary file %s", tempFile))
	}

	// Atomic move from temp to target
	if err := os.Rename(tempFile, filePath); err != nil {
		return errors.WrapWithContext(err, fmt.Sprintf("failed to move %s to %s", tempFile, filePath))
	}

	logger.Debug("Successfully wrote file: %s", filePath)
	return nil
}

// BackupFile creates a backup of a file before modification
func BackupFile(logger *logging.Logger, filePath string) (string, error) {
	if filePath == "" {
		return "", errors.NewValidationError("file_path", filePath, "file path cannot be empty")
	}

	// Check if source file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		logger.Debug("Source file does not exist, no backup needed: %s", filePath)
		return "", nil
	}

	// Create backup filename with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.backup.%s", filePath, timestamp)

	// Copy file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", errors.WrapWithContext(err, fmt.Sprintf("failed to read source file %s", filePath))
	}

	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		return "", errors.WrapWithContext(err, fmt.Sprintf("failed to write backup file %s", backupPath))
	}

	logger.Info("Created backup: %s", backupPath)
	return backupPath, nil
}

// CleanupWithRetry attempts to clean up resources with retry logic
func CleanupWithRetry(logger *logging.Logger, resourceName string, cleanupFn func() error) {
	err := errors.RetryWithBackoff(func() error {
		return cleanupFn()
	}, 3, 500*time.Millisecond)

	if err != nil {
		logger.Error("Failed to cleanup %s after retries: %v", resourceName, err)
	} else {
		logger.Debug("Successfully cleaned up %s", resourceName)
	}
}

// ValidateAndExecute validates inputs and executes an operation with proper error handling
func ValidateAndExecute(logger *logging.Logger, operation string, validators []func() error, executor func() error) error {
	// Run all validations first
	for i, validator := range validators {
		if err := validator(); err != nil {
			return errors.WrapWithContext(err, fmt.Sprintf("validation %d failed for operation '%s'", i+1, operation))
		}
	}

	logger.Debug("All validations passed for operation: %s", operation)

	// Execute the operation
	if err := executor(); err != nil {
		return errors.WrapWithContext(err, fmt.Sprintf("execution failed for operation '%s'", operation))
	}

	logger.Debug("Operation completed successfully: %s", operation)
	return nil
}

// LogAndWrapError logs an error and wraps it with context
func LogAndWrapError(logger *logging.Logger, err error, context string) error {
	if err == nil {
		return nil
	}

	logger.Error("%s: %v", context, err)
	return errors.WrapWithContext(err, context)
}

// HandlePanicAsError converts panics to errors for safer execution
func HandlePanicAsError(logger *logging.Logger, operation string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic in operation '%s': %v", operation, r)
			logger.Error("Panic recovered: %v", err)
		}
	}()
	return nil
}

// ConditionalError returns an error only if the condition is met
func ConditionalError(condition bool, err error) error {
	if condition {
		return err
	}
	return nil
}

// FirstError returns the first non-nil error from a list of errors
func FirstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// AggregateErrors combines multiple errors into a single error message
func AggregateErrors(operation string, errs ...error) error {
	var nonNilErrors []error
	for _, err := range errs {
		if err != nil {
			nonNilErrors = append(nonNilErrors, err)
		}
	}

	if len(nonNilErrors) == 0 {
		return nil
	}

	if len(nonNilErrors) == 1 {
		return errors.WrapWithContext(nonNilErrors[0], operation)
	}

	errMsg := fmt.Sprintf("%s: multiple errors occurred:", operation)
	for i, err := range nonNilErrors {
		errMsg += fmt.Sprintf(" [%d] %v", i+1, err)
	}

	return fmt.Errorf("%s", errMsg)
}

// TimeoutOperation executes an operation with a timeout
func TimeoutOperation(logger *logging.Logger, operation string, timeout time.Duration, fn func() error) error {
	done := make(chan error, 1)
	
	go func() {
		defer func() {
			if r := recover(); r != nil {
				done <- fmt.Errorf("panic in operation '%s': %v", operation, r)
			}
		}()
		done <- fn()
	}()

	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		logger.Warn("Operation '%s' timed out after %v", operation, timeout)
		return fmt.Errorf("operation '%s' timed out after %v", operation, timeout)
	}
}