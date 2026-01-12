package errors

import (
	"errors"
	"fmt"
	"math"
	"time"
)

// Common error types
var (
	ErrInvalidInput    = errors.New("invalid input")
	ErrOperationFailed = errors.New("operation failed")
	ErrTimeout         = errors.New("operation timed out")
	ErrNotFound        = errors.New("resource not found")
	ErrAlreadyExists   = errors.New("resource already exists")
)

// ValidationError represents validation-specific errors
type ValidationError struct {
	Field   string
	Value   string
	Message string
}

func (e *ValidationError) Error() string {
	if e.Value != "" {
		return fmt.Sprintf("validation failed for field '%s' with value '%s': %s", e.Field, e.Value, e.Message)
	}
	return fmt.Sprintf("validation failed for field '%s': %s", e.Field, e.Message)
}

func (e *ValidationError) Is(target error) bool {
	return target == ErrInvalidInput
}

// NetworkError represents network-related errors
type NetworkError struct {
	Operation string
	URL       string
	Err       error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("network operation '%s' failed for URL '%s': %v", e.Operation, e.URL, e.Err)
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

// DockerError represents Docker-specific errors
type DockerError struct {
	Operation string
	Container string
	Err       error
}

func (e *DockerError) Error() string {
	if e.Container != "" {
		return fmt.Sprintf("docker operation '%s' failed for container '%s': %v", e.Operation, e.Container, e.Err)
	}
	return fmt.Sprintf("docker operation '%s' failed: %v", e.Operation, e.Err)
}

func (e *DockerError) Unwrap() error {
	return e.Err
}

// ConfigError represents configuration-related errors
type ConfigError struct {
	Field   string
	Value   string
	Message string
}

func (e *ConfigError) Error() string {
	if e.Value != "" {
		return fmt.Sprintf("config error for field '%s' with value '%s': %s", e.Field, e.Value, e.Message)
	}
	return fmt.Sprintf("config error for field '%s': %s", e.Field, e.Message)
}

// InstallationError represents installation-specific errors
type InstallationError struct {
	Component string
	Step      string
	Err       error
}

func (e *InstallationError) Error() string {
	return fmt.Sprintf("installation failed for component '%s' at step '%s': %v", e.Component, e.Step, e.Err)
}

func (e *InstallationError) Unwrap() error {
	return e.Err
}

// WrapWithContext wraps an error with additional context
func WrapWithContext(err error, context string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", context, err)
}

// RetryWithBackoff executes an operation with exponential backoff retry logic
func RetryWithBackoff(operation func() error, maxRetries int, baseDelay time.Duration) error {
	if maxRetries <= 0 {
		return fmt.Errorf("maxRetries must be greater than 0")
	}

	var lastErr error
	for i := 0; i < maxRetries; i++ {
		if err := operation(); err == nil {
			return nil
		} else {
			lastErr = err
			if i == maxRetries-1 {
				break
			}
			delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(i)))
			time.Sleep(delay)
		}
	}
	return fmt.Errorf("operation failed after %d retries: %w", maxRetries, lastErr)
}

// SafeExecute executes a function and returns an error if it panics
func SafeExecute(fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic recovered: %v", r)
		}
	}()
	return fn()
}

// IsTemporary checks if an error is temporary/retryable
func IsTemporary(err error) bool {
	type temporary interface {
		Temporary() bool
	}
	
	if temp, ok := err.(temporary); ok {
		return temp.Temporary()
	}
	
	// Check for specific error types that are usually temporary
	var netErr *NetworkError
	if errors.As(err, &netErr) {
		return true
	}
	
	return false
}

// NewValidationError creates a new validation error
func NewValidationError(field, value, message string) *ValidationError {
	return &ValidationError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// NewNetworkError creates a new network error
func NewNetworkError(operation, url string, err error) *NetworkError {
	return &NetworkError{
		Operation: operation,
		URL:       url,
		Err:       err,
	}
}

// NewDockerError creates a new Docker error
func NewDockerError(operation, container string, err error) *DockerError {
	return &DockerError{
		Operation: operation,
		Container: container,
		Err:       err,
	}
}

// NewConfigError creates a new configuration error
func NewConfigError(field, value, message string) *ConfigError {
	return &ConfigError{
		Field:   field,
		Value:   value,
		Message: message,
	}
}

// NewInstallationError creates a new installation error
func NewInstallationError(component, step string, err error) *InstallationError {
	return &InstallationError{
		Component: component,
		Step:      step,
		Err:       err,
	}
}