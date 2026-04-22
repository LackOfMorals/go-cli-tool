package core

import (
	"errors"
	"fmt"
)

// Common error definitions
var (
	ErrToolNotFound      = errors.New("tool not found")
	ErrToolAlreadyExists = errors.New("tool already registered")
	ErrInvalidConfig     = errors.New("invalid configuration")
	ErrShellNotRunning   = errors.New("shell is not running")
	ErrInvalidInput      = errors.New("invalid input")
	ErrNotImplemented    = errors.New("not implemented")
)

// ToolError represents an error from a tool execution
type ToolError struct {
	ToolName string
	Code     string
	Message  string
	Err      error
}

func (e *ToolError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("[%s] %s: %v", e.ToolName, e.Message, e.Err)
	}
	return fmt.Sprintf("[%s] %s", e.ToolName, e.Message)
}

func (e *ToolError) Unwrap() error {
	return e.Err
}

// NewToolError creates a new ToolError
func NewToolError(toolName, code, message string, err error) *ToolError {
	return &ToolError{
		ToolName: toolName,
		Code:     code,
		Message:  message,
		Err:      err,
	}
}

// ConfigError represents a configuration-related error
type ConfigError struct {
	Key   string
	Value interface{}
	Err   error
}

func (e *ConfigError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("config error for key '%s': %v", e.Key, e.Err)
	}
	return fmt.Sprintf("config error for key '%s': invalid value %v", e.Key, e.Value)
}

func (e *ConfigError) Unwrap() error {
	return e.Err
}

// NewConfigError creates a new ConfigError
func NewConfigError(key string, value interface{}, err error) *ConfigError {
	return &ConfigError{
		Key:   key,
		Value: value,
		Err:   err,
	}
}

// ShellError represents a shell-related error
type ShellError struct {
	Command string
	Message string
	Err     error
}

func (e *ShellError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("shell error executing '%s': %v", e.Command, e.Err)
	}
	return fmt.Sprintf("shell error: %s", e.Message)
}

func (e *ShellError) Unwrap() error {
	return e.Err
}

// NewShellError creates a new ShellError
func NewShellError(command, message string, err error) *ShellError {
	return &ShellError{
		Command: command,
		Message: message,
		Err:     err,
	}
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string
	Message string
	Value   interface{}
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error for '%s': %s (got: %v)", e.Field, e.Message, e.Value)
}

// NewValidationError creates a new ValidationError
func NewValidationError(field, message string, value interface{}) *ValidationError {
	return &ValidationError{
		Field:   field,
		Message: message,
		Value:   value,
	}
}

// IsErrNotFound checks if error is a not found error
func IsErrNotFound(err error) bool {
	return errors.Is(err, ErrToolNotFound)
}

// IsErrAlreadyExists checks if error is an already exists error
func IsErrAlreadyExists(err error) bool {
	return errors.Is(err, ErrToolAlreadyExists)
}

// IsErrInvalidConfig checks if error is an invalid config error
func IsErrInvalidConfig(err error) bool {
	return errors.Is(err, ErrInvalidConfig)
}
