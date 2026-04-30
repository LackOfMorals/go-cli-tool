package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	tests := []struct {
		name   string
		input  string
		xdg    string
		setXDG bool
		want   string
		wantOK bool
	}{
		{name: "tilde slash", input: "~/foo/bar", want: filepath.Join(tmp, "foo/bar"), wantOK: true},
		{name: "bare tilde", input: "~", want: tmp, wantOK: true},
		{name: "absolute path passthrough", input: "/etc/foo", want: "/etc/foo", wantOK: true},
		{name: "xdg unset falls back to ~/.config", input: "$XDG_CONFIG_HOME/opencode", want: filepath.Join(tmp, ".config", "opencode"), wantOK: true},
		{name: "xdg set", input: "$XDG_CONFIG_HOME/opencode", xdg: "/custom/cfg", setXDG: true, want: "/custom/cfg/opencode", wantOK: true},
		{name: "xdg set empty falls back", input: "$XDG_CONFIG_HOME/opencode", xdg: "", setXDG: true, want: filepath.Join(tmp, ".config", "opencode"), wantOK: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setXDG {
				t.Setenv("XDG_CONFIG_HOME", tc.xdg)
			} else {
				if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
					t.Fatalf("unset XDG_CONFIG_HOME: %v", err)
				}
			}
			got, ok := expandPath(tc.input)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExpandPathNoHome(t *testing.T) {
	if err := os.Unsetenv("HOME"); err != nil {
		t.Fatalf("unset HOME: %v", err)
	}
	// re-set after the test so subsequent tests are unaffected
	t.Cleanup(func() { _ = os.Setenv("HOME", os.Getenv("HOME")) })

	if _, ok := expandPath("~/foo"); ok {
		t.Errorf("expected ok=false when HOME unset")
	}
}

func TestFindAgent(t *testing.T) {
	if a := FindAgent("claude-code"); a == nil || a.Name != "claude-code" {
		t.Errorf("FindAgent(claude-code) = %+v, want claude-code", a)
	}
	if a := FindAgent("CLAUDE-CODE"); a == nil || a.Name != "claude-code" {
		t.Errorf("FindAgent(CLAUDE-CODE) case-insensitive failed: %+v", a)
	}
	if a := FindAgent("nope"); a != nil {
		t.Errorf("FindAgent(nope) = %+v, want nil", a)
	}
}

func TestAgentsCatalog(t *testing.T) {
	want := []string{
		"claude-code", "cursor", "windsurf", "copilot", "gemini-cli",
		"cline", "codex", "pi", "opencode", "junie",
	}
	if len(AGENTS) != len(want) {
		t.Fatalf("AGENTS has %d entries, want %d", len(AGENTS), len(want))
	}
	for i, name := range want {
		if AGENTS[i].Name != name {
			t.Errorf("AGENTS[%d].Name = %q, want %q", i, AGENTS[i].Name, name)
		}
		if AGENTS[i].DisplayName == "" {
			t.Errorf("AGENTS[%d].DisplayName empty", i)
		}
		if AGENTS[i].DetectDir == "" || AGENTS[i].SkillsDir == "" {
			t.Errorf("AGENTS[%d] has empty path", i)
		}
	}
}

func TestDetectAgents(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
		t.Fatalf("unset XDG_CONFIG_HOME: %v", err)
	}

	// No agent dirs exist yet.
	if got := DetectAgents(); len(got) != 0 {
		t.Errorf("expected no detected agents in empty HOME, got %d", len(got))
	}

	// Create two detect_dirs.
	if err := os.MkdirAll(filepath.Join(tmp, ".claude"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmp, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}

	got := DetectAgents()
	if len(got) != 2 {
		t.Fatalf("expected 2 detected agents, got %d", len(got))
	}
	if got[0].Name != "claude-code" || got[1].Name != "cursor" {
		t.Errorf("detected order = [%s,%s], want [claude-code,cursor]", got[0].Name, got[1].Name)
	}
}

func TestAgentSkillsPath(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	a := FindAgent("claude-code")
	if a == nil {
		t.Fatal("claude-code not found")
	}
	got, ok := a.SkillsPath()
	if !ok {
		t.Fatal("SkillsPath ok=false")
	}
	want := filepath.Join(tmp, ".claude", "skills")
	if got != want {
		t.Errorf("SkillsPath = %q, want %q", got, want)
	}
}
