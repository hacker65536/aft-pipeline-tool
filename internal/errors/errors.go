package errors

import (
	"fmt"
)

// ErrorType represents different types of errors
type ErrorType string

const (
	ErrorTypeValidation ErrorType = "validation"
	ErrorTypeAWS        ErrorType = "aws"
	ErrorTypeCache      ErrorType = "cache"
	ErrorTypeConfig     ErrorType = "config"
	ErrorTypeInternal   ErrorType = "internal"
)

// AppError represents an application-specific error
type AppError struct {
	Type    ErrorType
	Message string
	Cause   error
	Context map[string]interface{}
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// Unwrap returns the underlying error
func (e *AppError) Unwrap() error {
	return e.Cause
}

// NewValidationError creates a new validation error
func NewValidationError(message string, cause error) *AppError {
	return &AppError{
		Type:    ErrorTypeValidation,
		Message: message,
		Cause:   cause,
		Context: make(map[string]interface{}),
	}
}

// NewAWSError creates a new AWS-related error
func NewAWSError(message string, cause error) *AppError {
	return &AppError{
		Type:    ErrorTypeAWS,
		Message: message,
		Cause:   cause,
		Context: make(map[string]interface{}),
	}
}

// NewCacheError creates a new cache-related error
func NewCacheError(message string, cause error) *AppError {
	return &AppError{
		Type:    ErrorTypeCache,
		Message: message,
		Cause:   cause,
		Context: make(map[string]interface{}),
	}
}

// NewConfigError creates a new configuration error
func NewConfigError(message string, cause error) *AppError {
	return &AppError{
		Type:    ErrorTypeConfig,
		Message: message,
		Cause:   cause,
		Context: make(map[string]interface{}),
	}
}

// NewInternalError creates a new internal error
func NewInternalError(message string, cause error) *AppError {
	return &AppError{
		Type:    ErrorTypeInternal,
		Message: message,
		Cause:   cause,
		Context: make(map[string]interface{}),
	}
}

// WithContext adds context to an error
func (e *AppError) WithContext(key string, value interface{}) *AppError {
	e.Context[key] = value
	return e
}

// IsType checks if the error is of a specific type
func IsType(err error, errorType ErrorType) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == errorType
	}
	return false
}
