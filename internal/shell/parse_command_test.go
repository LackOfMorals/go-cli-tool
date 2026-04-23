package shell

import (
	"testing"
)

func TestParseCommand(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
		{
			name:  "whitespace only",
			input: "   ",
			want:  nil,
		},
		{
			name:  "single token",
			input: "help",
			want:  []string{"help"},
		},
		{
			name:  "multiple tokens",
			input: "cloud instances list",
			want:  []string{"cloud", "instances", "list"},
		},
		{
			name:  "extra spaces between tokens",
			input: "cloud   instances   list",
			want:  []string{"cloud", "instances", "list"},
		},
		{
			name:  "leading and trailing spaces",
			input: "  exit  ",
			want:  []string{"exit"},
		},
		{
			name:  "tab-separated tokens",
			input: "cloud\tinstances\tlist",
			want:  []string{"cloud", "instances", "list"},
		},
		{
			name:  "double-quoted argument with spaces",
			input: `cypher "MATCH (n) RETURN n"`,
			want:  []string{"cypher", "MATCH (n) RETURN n"},
		},
		{
			name:  "single-quoted argument with spaces",
			input: `cypher 'MATCH (n) RETURN n'`,
			want:  []string{"cypher", "MATCH (n) RETURN n"},
		},
		{
			name:  "multiple quoted arguments",
			input: `cmd "arg one" "arg two"`,
			want:  []string{"cmd", "arg one", "arg two"},
		},
		{
			name:  "mixed quoted and unquoted",
			input: `set prompt "neo4j> "`,
			want:  []string{"set", "prompt", "neo4j> "},
		},
		{
			name:  "quoted empty string produces empty token",
			input: `cmd ""`,
			want:  []string{"cmd", ""},
		},
		{
			name:  "unclosed quote falls back to whitespace split",
			input: `cypher "MATCH (n`,
			want:  []string{"cypher", `"MATCH`, "(n"},
		},
		{
			name:  "cypher with colon and braces",
			input: `cypher MATCH (n:Person {name: "Alice"}) RETURN n`,
			// shlex (and the old parser) treat "Alice"}) as a single token: the
			// closing quote is consumed, then } and ) attach to the same token
			// because there is no whitespace between them.
			want: []string{"cypher", "MATCH", "(n:Person", "{name:", "Alice})", "RETURN", "n"},
		},
		{
			name:  "quoted cypher preserves internal spaces",
			input: `query "MATCH (n:Person) RETURN n LIMIT 5"`,
			want:  []string{"query", "MATCH (n:Person) RETURN n LIMIT 5"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCommand(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("parseCommand(%q)\n  got  %v (len %d)\n  want %v (len %d)",
					tt.input, got, len(got), tt.want, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("token[%d]: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}
