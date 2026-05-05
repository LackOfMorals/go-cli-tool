package shell

import (
	"testing"

	"github.com/cli/go-cli-tool/internal/config"
)

// TestIsUnconfigured covers the package-level isUnconfigured helper defined in
// interactive.go. It lives in package shell (not shell_test) because
// isUnconfigured is unexported.
func TestIsUnconfigured(t *testing.T) {
	tests := []struct {
		name string
		cfg  *config.Config
		want bool
	}{
		{
			name: "nil config is unconfigured",
			cfg:  nil,
			want: true,
		},
		{
			name: "empty URI and empty password is unconfigured",
			cfg:  &config.Config{},
			want: true,
		},
		{
			name: "default sentinel URI and empty password is unconfigured",
			cfg: &config.Config{
				Neo4j: config.Neo4jConfig{URI: defaultNeo4jURI},
			},
			want: true,
		},
		{
			name: "custom URI without password is configured",
			cfg: &config.Config{
				Neo4j: config.Neo4jConfig{URI: "bolt://myhost:7687"},
			},
			want: false,
		},
		{
			name: "empty URI with password set is configured",
			cfg: &config.Config{
				Neo4j: config.Neo4jConfig{Password: "secret"},
			},
			want: false,
		},
		{
			name: "default sentinel URI with password set is configured",
			cfg: &config.Config{
				Neo4j: config.Neo4jConfig{URI: defaultNeo4jURI, Password: "secret"},
			},
			want: false,
		},
		{
			name: "custom URI with password set is configured",
			cfg: &config.Config{
				Neo4j: config.Neo4jConfig{URI: "bolt://prod:7687", Password: "hunter2"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isUnconfigured(tt.cfg)
			if got != tt.want {
				t.Errorf("isUnconfigured(%v) = %v, want %v", tt.cfg, got, tt.want)
			}
		})
	}
}
