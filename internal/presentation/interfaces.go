package presentation

// Service is the public interface for output formatting.
//
// Implementations are expected to be safe for concurrent use by multiple
// goroutines unless otherwise documented.
type Service interface {
	// Format renders data using the currently active output format.
	Format(data any) (string, error)

	// RegisterFormatter adds (or replaces) the formatter for a given format.
	// Returns an error if formatter is nil.
	RegisterFormatter(format OutputFormat, formatter OutputFormatter) error

	// SetFormat switches the active output format. The format must already
	// have a registered formatter.
	SetFormat(format OutputFormat) error
}

// OutputFormatter is implemented by every concrete formatter
// (TextFormatter, JSONFormatter, TableFormatter, ...).
type OutputFormatter interface {
	Format(data any) (string, error)
}

// Tabular is an optional interface that data types can implement to enable
// table-aware rendering in TextFormatter and TableFormatter.
type Tabular interface {
	Columns() []string
	Rows() [][]string
}
