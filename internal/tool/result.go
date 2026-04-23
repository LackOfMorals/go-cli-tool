package tool

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// Result holds the output of a tool execution.
//
// Construction patterns:
//
//   - Simple: return tool.SuccessResult("output"), nil
//     or: return tool.ErrorResult("usage hint"), fmt.Errorf("what went wrong")
//
//   - Rich (with structured data): use NewResult() then the With* builders.
//
// Errors are signalled through the function's error return, not through a
// field on Result. This keeps a single error channel and avoids the
// serialisation problems that come with storing an error interface.
type Result struct {
	Success  bool
	Output   string
	Data     map[string]interface{}
	Logs     []string
	Metadata map[string]interface{}
}

// NewResult creates an empty Result with all maps initialised, ready for
// the With* builder methods.
func NewResult() *Result {
	return &Result{
		Data:     make(map[string]interface{}),
		Logs:     []string{},
		Metadata: make(map[string]interface{}),
	}
}

// SuccessResult creates a successful result with the given output string.
func SuccessResult(output string) Result {
	return Result{Success: true, Output: output}
}

// ErrorResult creates a failed result with an output string (typically a
// usage hint). The accompanying error is returned separately via the
// function's error return value.
func ErrorResult(output string) Result {
	return Result{Success: false, Output: output}
}

// ---- Builder methods (pointer receivers for NewResult() path) -----------

// SetSuccess marks the result as successful and sets the output string.
func (r *Result) SetSuccess(output string) {
	r.Success = true
	r.Output = output
}

// WithData adds a key/value pair to the structured data map.
func (r *Result) WithData(key string, value interface{}) *Result {
	if r.Data == nil {
		r.Data = make(map[string]interface{})
	}
	r.Data[key] = value
	return r
}

// WithLog appends a log line to the result.
func (r *Result) WithLog(log string) *Result {
	r.Logs = append(r.Logs, log)
	return r
}

// WithMetadata adds a key/value pair to the metadata map.
func (r *Result) WithMetadata(key string, value interface{}) *Result {
	if r.Metadata == nil {
		r.Metadata = make(map[string]interface{})
	}
	r.Metadata[key] = value
	return r
}

// ---- Serialisation ------------------------------------------------------

// String implements fmt.Stringer and returns the output string.
func (r *Result) String() string { return r.Output }

// JSON returns the result serialised as indented JSON.
func (r *Result) JSON() (string, error) {
	data := map[string]interface{}{
		"success": r.Success,
		"output":  r.Output,
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
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal result: %w", err)
	}
	return string(b), nil
}

// ToMap converts the result to a plain map, suitable for further processing.
func (r *Result) ToMap() map[string]interface{} {
	data := map[string]interface{}{
		"success": r.Success,
		"output":  r.Output,
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

// ---- IOHandler ----------------------------------------------------------

// IOHandler defines the interface for reading input and writing output.
type IOHandler interface {
	Read() (string, error)
	Write(format string, args ...interface{})
	WriteError(err error)
	WriteLine(line string)
	WriteJSON(data interface{}) error
}

// DefaultIOHandler writes to stdout / stderr and reads from stdin.
type DefaultIOHandler struct{}

func NewDefaultIOHandler() *DefaultIOHandler { return &DefaultIOHandler{} }

func (h *DefaultIOHandler) Read() (string, error) {
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read input: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func (h *DefaultIOHandler) Write(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func (h *DefaultIOHandler) WriteError(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}
}

func (h *DefaultIOHandler) WriteLine(line string) { fmt.Println(line) }

func (h *DefaultIOHandler) WriteJSON(data interface{}) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	fmt.Println(string(b))
	return nil
}
