package tool

import (
	"encoding/json"
	"fmt"
	"os"
)

// Result represents the output of a tool execution
type Result struct {
	// Success indicates if the execution was successful
	Success bool

	// Output contains the main output string
	Output string

	// Error contains any error that occurred
	Error error

	// Data contains structured data output
	Data map[string]interface{}

	// Logs contains log messages from the execution
	Logs []string

	// Metadata contains additional metadata about the execution
	Metadata map[string]interface{}
}

// NewResult creates a new Result instance
func NewResult() *Result {
	return &Result{
		Data:     make(map[string]interface{}),
		Logs:    []string{},
		Metadata: make(map[string]interface{}),
	}
}

// SuccessResult creates a successful result
func SuccessResult(output string) Result {
	return Result{
		Success: true,
		Output:  output,
	}
}

// ErrorResult creates an error result
func ErrorResult(output string, err error) Result {
	return Result{
		Success: false,
		Output:  output,
		Error:   err,
	}
}

// WithData adds data to the result
func (r *Result) WithData(key string, value interface{}) *Result {
	r.Data[key] = value
	return r
}

// WithLog adds a log message to the result
func (r *Result) WithLog(log string) *Result {
	r.Logs = append(r.Logs, log)
	return r
}

// WithMetadata adds metadata to the result
func (r *Result) WithMetadata(key string, value interface{}) *Result {
	r.Metadata[key] = value
	return r
}

// SetSuccess marks the result as successful
func (r *Result) SetSuccess(output string) {
	r.Success = true
	r.Output = output
}

// SetError marks the result as an error
func (r *Result) SetError(output string, err error) {
	r.Success = false
	r.Output = output
	r.Error = err
}

// String returns the output string
func (r *Result) String() string {
	return r.Output
}

// JSON returns the result as JSON
func (r *Result) JSON() (string, error) {
	data := map[string]interface{}{
		"success": r.Success,
		"output":  r.Output,
	}

	if r.Error != nil {
		data["error"] = r.Error.Error()
	}

	if len(r.Data) > 0 {
		data["data"] = r.Data
	}

	if len(r.Logs) > 0 {
		data["logs"] = r.Logs
	}

	if len(r.Metadata) > 0 {
		data["metadata"] = r.Metadata
	}

	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal result: %w", err)
	}
	return string(bytes), nil
}

// ToMap converts the result to a map
func (r *Result) ToMap() map[string]interface{} {
	data := map[string]interface{}{
		"success": r.Success,
		"output":  r.Output,
	}

	if r.Error != nil {
		data["error"] = r.Error.Error()
	}

	if len(r.Data) > 0 {
		data["data"] = r.Data
	}

	if len(r.Logs) > 0 {
		data["logs"] = r.Logs
	}

	if len(r.Metadata) > 0 {
		data["metadata"] = r.Metadata
	}

	return data
}

// IOHandler defines the interface for input/output operations
type IOHandler interface {
	// Read reads input from the user
	Read() (string, error)

	// Write writes formatted output
	Write(format string, args ...interface{})

	// WriteError writes an error message
	WriteError(err error)

	// WriteLine writes a line of output
	WriteLine(line string)

	// WriteJSON writes JSON formatted data
	WriteJSON(data interface{}) error
}

// DefaultIOHandler provides a default implementation of IOHandler
type DefaultIOHandler struct {
	// Add any dependencies here
}

// NewDefaultIOHandler creates a new DefaultIOHandler
func NewDefaultIOHandler() *DefaultIOHandler {
	return &DefaultIOHandler{}
}

// Read reads input from stdin
func (h *DefaultIOHandler) Read() (string, error) {
	var input string
	fmt.Scanln(&input)
	return input, nil
}

// Write writes formatted output to stdout
func (h *DefaultIOHandler) Write(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

// WriteError writes an error to stderr
func (h *DefaultIOHandler) WriteError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}

// WriteLine writes a line to stdout
func (h *DefaultIOHandler) WriteLine(line string) {
	fmt.Println(line)
}

// WriteJSON writes JSON to stdout
func (h *DefaultIOHandler) WriteJSON(data interface{}) error {
	bytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}
	fmt.Println(string(bytes))
	return nil
}
