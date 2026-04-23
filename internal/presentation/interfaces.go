package presentation

// Service is the public interface for output formatting.
//
// Implementations are expected to be safe for concurrent use by multiple
// goroutines unless otherwise documented.
type Service interface {
	// Format renders data using the currently active output format.
	Format(data any) (string, error)

	// FormatAs renders data using the specified format regardless of what the
	// current default format is. Use this for per-query format overrides.
	FormatAs(data any, format OutputFormat) (string, error)

	// RegisterFormatter adds (or replaces) the formatter for a given format.
	// Returns an error if formatter is nil.
	RegisterFormatter(format OutputFormat, formatter OutputFormatter) error

	// SetFormat switches the active output format. The format must already
	// have a registered formatter.
	SetFormat(format OutputFormat) error
}

// OutputFormatter is implemented by every concrete formatter.
type OutputFormatter interface {
	Format(data any) (string, error)
}

// Tabular is implemented by data types that want first-class table rendering.
// Rows returns interface{} slices so that numeric, boolean, and complex values
// (nodes, relationships) can be rendered with proper type-aware formatting.
type Tabular interface {
	Columns() []string
	Rows() [][]interface{}
}
