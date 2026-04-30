package service

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/cli/go-cli-tool/internal/skill"
)

// skillSubdir is the per-tool folder created under each agent's skills_dir.
// The embedded SKILL.md lives at <skills_dir>/<skillSubdir>/SKILL.md.
const skillSubdir = "neo4j-cli"

// skillFile is the filename of the embedded skill document.
const skillFile = "SKILL.md"

// SkillServiceImpl is the production SkillService.
//
// It writes the embedded skill.SkillMD directly into each agent's skills
// directory — there is no canonical/cache dir (the Go version intentionally
// diverges from the Rust reference here). All filesystem access goes through
// the Filesystem interface so tests can substitute a TempDir-backed wrapper.
type SkillServiceImpl struct {
	fs      skill.Filesystem
	agents  []skill.Agent
	content []byte
}

// NewSkillService returns a SkillService backed by the OS filesystem and the
// embedded SKILL.md.
func NewSkillService() SkillService {
	return &SkillServiceImpl{
		fs:      skill.OSFilesystem{},
		agents:  skill.AGENTS,
		content: skill.SkillMD,
	}
}

// NewSkillServiceWith builds a SkillService with explicit dependencies. Used
// by tests that need to inject a fake Filesystem, alternative agent list, or
// stub content.
func NewSkillServiceWith(fs skill.Filesystem, agents []skill.Agent, content []byte) SkillService {
	return &SkillServiceImpl{fs: fs, agents: agents, content: content}
}

// Install writes the embedded SKILL.md into each target agent's skills
// directory. With an empty agentName, every detected agent is targeted; an
// error is returned if no agents are detected. With a known agentName, the
// install proceeds even if the agent is not currently detected. Any pre-
// existing file, directory, or symlink at the destination is removed first
// so the operation is idempotent.
func (s *SkillServiceImpl) Install(_ context.Context, agentName string) ([]InstallResult, error) {
	targets, err := s.resolveInstallTargets(agentName)
	if err != nil {
		return nil, err
	}

	results := make([]InstallResult, 0, len(targets))
	for _, agent := range targets {
		path, err := s.installOne(agent)
		if err != nil {
			return results, fmt.Errorf("install %s: %w", agent.Name, err)
		}
		results = append(results, InstallResult{Agent: agent.Name, Path: path})
	}
	return results, nil
}

// Remove deletes the per-tool skill folder for each target agent. With an
// empty agentName, every agent that currently has the skill installed is
// targeted. Missing targets are silently ignored per agent.
func (s *SkillServiceImpl) Remove(_ context.Context, agentName string) ([]RemoveResult, error) {
	targets, err := s.resolveRemoveTargets(agentName)
	if err != nil {
		return nil, err
	}

	results := make([]RemoveResult, 0, len(targets))
	for _, agent := range targets {
		removed, err := s.removeOne(agent)
		if err != nil {
			return results, fmt.Errorf("remove %s: %w", agent.Name, err)
		}
		if removed {
			results = append(results, RemoveResult{Agent: agent.Name})
		}
	}
	return results, nil
}

// List returns the per-agent status for every known agent. Detected reflects
// whether the agent's detect_dir exists; Installed reflects whether the
// per-tool skill directory exists under the agent's skills_dir.
func (s *SkillServiceImpl) List(_ context.Context) ([]AgentStatus, error) {
	out := make([]AgentStatus, 0, len(s.agents))
	for i := range s.agents {
		a := s.agents[i]
		out = append(out, AgentStatus{
			Name:        a.Name,
			DisplayName: a.DisplayName,
			Detected:    s.isDetected(a),
			Installed:   s.isInstalled(a),
		})
	}
	return out, nil
}

// resolveInstallTargets picks the agents an Install call should write to.
func (s *SkillServiceImpl) resolveInstallTargets(agentName string) ([]skill.Agent, error) {
	if agentName != "" {
		agent := s.findAgent(agentName)
		if agent == nil {
			return nil, fmt.Errorf("unknown agent %q", agentName)
		}
		return []skill.Agent{*agent}, nil
	}

	detected := s.detectedAgents()
	if len(detected) == 0 {
		return nil, errors.New("no supported AI agents detected; pass an agent name to install for a specific agent")
	}
	return detected, nil
}

// resolveRemoveTargets picks the agents a Remove call should clean up.
func (s *SkillServiceImpl) resolveRemoveTargets(agentName string) ([]skill.Agent, error) {
	if agentName != "" {
		agent := s.findAgent(agentName)
		if agent == nil {
			return nil, fmt.Errorf("unknown agent %q", agentName)
		}
		return []skill.Agent{*agent}, nil
	}

	var out []skill.Agent
	for i := range s.agents {
		if s.isInstalled(s.agents[i]) {
			out = append(out, s.agents[i])
		}
	}
	return out, nil
}

// installOne writes the embedded SKILL.md for a single agent. Returns the
// absolute path that was written.
func (s *SkillServiceImpl) installOne(agent skill.Agent) (string, error) {
	skillsDir, ok := agent.SkillsPath()
	if !ok {
		return "", fmt.Errorf("cannot resolve skills path for %s", agent.DisplayName)
	}

	targetDir := filepath.Join(skillsDir, skillSubdir)
	targetFile := filepath.Join(targetDir, skillFile)

	// Wipe any existing target — file, directory, or symlink — so reinstalls
	// are idempotent and never collide with a previous symlink-style install.
	if err := s.removePath(targetDir); err != nil {
		return "", err
	}

	if err := s.fs.MkdirAll(targetDir, 0o755); err != nil {
		return "", err
	}
	if err := s.fs.WriteFile(targetFile, s.content, 0o644); err != nil {
		return "", err
	}
	return targetFile, nil
}

// removeOne deletes the per-tool skill directory for a single agent. Returns
// (true, nil) if something was removed, (false, nil) if nothing was there.
func (s *SkillServiceImpl) removeOne(agent skill.Agent) (bool, error) {
	skillsDir, ok := agent.SkillsPath()
	if !ok {
		return false, nil
	}
	targetDir := filepath.Join(skillsDir, skillSubdir)

	if _, err := s.fs.Lstat(targetDir); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	if err := s.fs.RemoveAll(targetDir); err != nil {
		return false, err
	}
	return true, nil
}

// removePath removes name regardless of type (file, dir, symlink). Missing
// targets are not an error.
func (s *SkillServiceImpl) removePath(name string) error {
	if _, err := s.fs.Lstat(name); err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return err
	}
	return s.fs.RemoveAll(name)
}

// findAgent looks up an agent in this service's catalog (case-insensitive).
func (s *SkillServiceImpl) findAgent(name string) *skill.Agent {
	for i := range s.agents {
		if strings.EqualFold(s.agents[i].Name, name) {
			return &s.agents[i]
		}
	}
	return nil
}

// detectedAgents returns the subset of the catalog whose detect_dir exists.
func (s *SkillServiceImpl) detectedAgents() []skill.Agent {
	var out []skill.Agent
	for i := range s.agents {
		if s.isDetected(s.agents[i]) {
			out = append(out, s.agents[i])
		}
	}
	return out
}

// isDetected reports whether agent.detect_dir exists on the configured filesystem.
func (s *SkillServiceImpl) isDetected(agent skill.Agent) bool {
	p, ok := agent.DetectPath()
	if !ok {
		return false
	}
	_, err := s.fs.Stat(p)
	return err == nil
}

// isInstalled reports whether the per-tool skill directory exists under the
// agent's skills_dir.
func (s *SkillServiceImpl) isInstalled(agent skill.Agent) bool {
	p, ok := agent.SkillsPath()
	if !ok {
		return false
	}
	_, err := s.fs.Lstat(filepath.Join(p, skillSubdir))
	return err == nil
}
