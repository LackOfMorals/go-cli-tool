package presentation

// TableData is a generic Tabular implementation for result sets
// (query results, list commands, etc.).
type TableData struct {
	columns []string
	rows    [][]interface{}
}

// NewTableData creates a TableData. columns defines the header; each element
// of rows must have the same length as columns.
func NewTableData(columns []string, rows [][]interface{}) *TableData {
	return &TableData{columns: columns, rows: rows}
}

func (d *TableData) Columns() []string        { return d.columns }
func (d *TableData) Rows() [][]interface{}    { return d.rows }
func (d *TableData) RowCount() int            { return len(d.rows) }

// DetailData is a key-value list used for "get" / detail commands.
// The TableFormatter renders it as a two-column table (no header).
// The JSONFormatter renders it as a flat JSON object.
type DetailData struct {
	Title  string        // optional section heading
	Fields []DetailField
}

// DetailField is a single labelled value in a DetailData.
type DetailField struct {
	Label string
	Value string
}

// NewDetailData is a convenience constructor.
func NewDetailData(title string, fields []DetailField) *DetailData {
	return &DetailData{Title: title, Fields: fields}
}
