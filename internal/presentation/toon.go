package presentation

// This file lands the github.com/toon-format/toon-go dependency for the
// upcoming TOON output format. The TOONFormatter type and its registration
// arrive in subsequent tasks; for now the blank import keeps the module in
// go.sum across `go mod tidy` runs so later imports compile cleanly.
//
// Planned usage (see tasks-add-toon-to-output-formats.yml task-004):
//   - Top-level encoder: toon.MarshalString(v) — returns the TOON document
//     as a string, mirroring how JSONFormatter calls json.Marshal.
//   - Options: library defaults (Core Profile, comma delimiter, two-space
//     indent). No non-default toon.WithIndent / toon.WithArrayDelimiter /
//     toon.WithLengthMarkers tuning is planned for v1.
//   - Errors are wrapped as fmt.Errorf("encode TOON: %w", err) for parity
//     with JSONFormatter's "marshal JSON: %w".

import (
	// Blank import: pins github.com/toon-format/toon-go in go.mod/go.sum so
	// subsequent tasks can import it without `go mod tidy` removing it. The
	// real import (and the TOONFormatter that uses toon.MarshalString) lands
	// in task-004.
	_ "github.com/toon-format/toon-go"
)
