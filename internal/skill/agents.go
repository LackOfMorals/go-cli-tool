// Package skill provides the catalog of supported AI agents and helpers for
// resolving their on-disk locations. The Agent list mirrors the Rust reference
// in oskarhane/homebrew-neo4j-query: detect_dir is checked for existence to
// determine whether an agent is installed locally; skills_dir is where the
// generated SKILL.md is dropped under a per-tool subdirectory.
package skill

import (
	"os"
	"path/filepath"
	"strings"
)

// Agent describes a single supported AI agent and the conventional locations
// where its presence is detected and where skills are installed.
type Agent struct {
	// Name is the canonical, lowercase identifier used on the CLI.
	Name string
	// DisplayName is the human-friendly label shown in tables and messages.
	DisplayName string
	// DetectDir is the path whose existence indicates the agent is installed.
	// May contain a leading `~` or `$XDG_CONFIG_HOME` placeholder.
	DetectDir string
	// SkillsDir is the directory where skill subfolders are placed. Same
	// expansion rules as DetectDir.
	SkillsDir string
}

// AGENTS is the hardcoded catalog of supported AI agents. Order is meaningful:
// it determines presentation in `skill list` output. Keep in sync with the
// Rust reference in homebrew-neo4j-query/src/skill.rs.
var AGENTS = []Agent{
	{Name: "claude-code", DisplayName: "Claude Code", DetectDir: "~/.claude", SkillsDir: "~/.claude/skills"},
	{Name: "cursor", DisplayName: "Cursor", DetectDir: "~/.cursor", SkillsDir: "~/.cursor/skills"},
	{Name: "windsurf", DisplayName: "Windsurf", DetectDir: "~/.codeium/windsurf", SkillsDir: "~/.codeium/windsurf/skills"},
	{Name: "copilot", DisplayName: "Copilot", DetectDir: "~/.copilot", SkillsDir: "~/.copilot/skills"},
	{Name: "gemini-cli", DisplayName: "Gemini CLI", DetectDir: "~/.gemini", SkillsDir: "~/.gemini/skills"},
	{Name: "cline", DisplayName: "Cline", DetectDir: "~/.cline", SkillsDir: "~/.agents/skills"},
	{Name: "codex", DisplayName: "Codex", DetectDir: "~/.codex", SkillsDir: "~/.codex/skills"},
	{Name: "pi", DisplayName: "Pi", DetectDir: "~/.pi/agent", SkillsDir: "~/.pi/agent/skills"},
	{Name: "opencode", DisplayName: "OpenCode", DetectDir: "$XDG_CONFIG_HOME/opencode", SkillsDir: "$XDG_CONFIG_HOME/opencode/skills"},
	{Name: "junie", DisplayName: "Junie", DetectDir: "~/.junie", SkillsDir: "~/.junie/skills"},
}

// DetectPath returns the absolute, expanded form of DetectDir. Returns an
// empty string with ok=false when the user's home directory cannot be
// determined.
func (a Agent) DetectPath() (string, bool) {
	return expandPath(a.DetectDir)
}

// SkillsPath returns the absolute, expanded form of SkillsDir. Returns an
// empty string with ok=false when the user's home directory cannot be
// determined.
func (a Agent) SkillsPath() (string, bool) {
	return expandPath(a.SkillsDir)
}

// FindAgent looks up an agent by canonical name. Matching is case-insensitive.
// Returns nil for unknown names.
func FindAgent(name string) *Agent {
	lower := strings.ToLower(name)
	for i := range AGENTS {
		if AGENTS[i].Name == lower {
			return &AGENTS[i]
		}
	}
	return nil
}

// DetectAgents returns the subset of AGENTS whose DetectDir exists on disk.
// Order matches the AGENTS slice. An agent whose DetectDir cannot be expanded
// (no HOME) is skipped silently.
func DetectAgents() []*Agent {
	var out []*Agent
	for i := range AGENTS {
		p, ok := AGENTS[i].DetectPath()
		if !ok {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			out = append(out, &AGENTS[i])
		}
	}
	return out
}

// expandPath resolves leading `~` and embedded `$XDG_CONFIG_HOME` references.
// Behaviour mirrors the Rust reference:
//   - `~/foo` and bare `~` expand against $HOME.
//   - `$XDG_CONFIG_HOME` expands to the env var if set and non-empty,
//     otherwise to `$HOME/.config`.
//   - Any other path is returned verbatim.
//
// Returns ok=false when expansion requires HOME and HOME is unset.
func expandPath(path string) (string, bool) {
	home, ok := os.LookupEnv("HOME")
	if !ok || home == "" {
		return "", false
	}

	if path == "~" {
		return home, true
	}
	if rest, found := strings.CutPrefix(path, "~/"); found {
		return filepath.Join(home, rest), true
	}
	if strings.Contains(path, "$XDG_CONFIG_HOME") {
		xdg := os.Getenv("XDG_CONFIG_HOME")
		if xdg == "" {
			xdg = filepath.Join(home, ".config")
		}
		return strings.ReplaceAll(path, "$XDG_CONFIG_HOME", xdg), true
	}
	return path, true
}
