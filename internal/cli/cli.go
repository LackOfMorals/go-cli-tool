// Package cli previously held the Neo4jCLI facade struct.
// Its responsibilities have been inlined directly into the App struct in
// cmd/neo4-cli/app.go, which is the only consumer and the right place for
// startup wiring. This file is kept to avoid breaking the directory; the
// package exports nothing and should be removed in a future cleanup.
package cli
