package service_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/cli/go-cli-tool/internal/service"
	"github.com/cli/go-cli-tool/internal/skill"
)

// ---- SkillService tests -------------------------------------------------
//
// All tests run against a t.TempDir-isolated $HOME. The production
// catalog is not used: each test injects a small slice of skill.Agent
// values whose DetectDir/SkillsDir start with `~/` so they expand into
// the temp HOME via skill.expandPath. The real OSFilesystem is used so
// the wiring is exercised end-to-end.

const fakeSkillContent = "fake skill body\n"

func newSkillTestEnv(t *testing.T) (string, []skill.Agent) {
	t.Helper()
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
		t.Fatalf("unset XDG_CONFIG_HOME: %v", err)
	}
	agents := []skill.Agent{
		{Name: "alpha", DisplayName: "Alpha", DetectDir: "~/.alpha", SkillsDir: "~/.alpha/skills"},
		{Name: "beta", DisplayName: "Beta", DetectDir: "~/.beta", SkillsDir: "~/.beta/skills"},
		{Name: "gamma", DisplayName: "Gamma", DetectDir: "~/.gamma", SkillsDir: "~/.gamma/skills"},
	}
	return tmp, agents
}

func newSkillService(agents []skill.Agent) service.SkillService {
	return service.NewSkillServiceWith(skill.OSFilesystem{}, agents, []byte(fakeSkillContent))
}

func mkDetectDir(t *testing.T, home, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(home, name), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", name, err)
	}
}

func skillFilePath(home, agentDir string) string {
	return filepath.Join(home, agentDir, "skills", "neo4j-cli", "SKILL.md")
}

// ---- Install ------------------------------------------------------------

func TestSkillService_Install_NoAgentsDetected(t *testing.T) {
	_, agents := newSkillTestEnv(t)
	svc := newSkillService(agents)

	_, err := svc.Install(context.Background(), "")
	if err == nil {
		t.Fatal("expected error when no agents are detected")
	}
	if !strings.Contains(err.Error(), "no supported AI agents detected") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestSkillService_Install_OneDetectedAgent(t *testing.T) {
	home, agents := newSkillTestEnv(t)
	mkDetectDir(t, home, ".beta")
	svc := newSkillService(agents)

	results, err := svc.Install(context.Background(), "")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(results) != 1 || results[0].Agent != "beta" {
		t.Fatalf("expected single beta install, got %+v", results)
	}
	want := skillFilePath(home, ".beta")
	if results[0].Path != want {
		t.Errorf("Path = %q, want %q", results[0].Path, want)
	}

	got, err := os.ReadFile(want)
	if err != nil {
		t.Fatalf("read installed file: %v", err)
	}
	if string(got) != fakeSkillContent {
		t.Errorf("file content = %q, want %q", got, fakeSkillContent)
	}
}

func TestSkillService_Install_AllDetected(t *testing.T) {
	home, agents := newSkillTestEnv(t)
	mkDetectDir(t, home, ".alpha")
	mkDetectDir(t, home, ".beta")
	mkDetectDir(t, home, ".gamma")
	svc := newSkillService(agents)

	results, err := svc.Install(context.Background(), "")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d (%+v)", len(results), results)
	}
	gotNames := []string{results[0].Agent, results[1].Agent, results[2].Agent}
	wantNames := []string{"alpha", "beta", "gamma"}
	for i, w := range wantNames {
		if gotNames[i] != w {
			t.Errorf("results[%d].Agent = %q, want %q", i, gotNames[i], w)
		}
	}
}

func TestSkillService_Install_OverExistingTarget(t *testing.T) {
	home, agents := newSkillTestEnv(t)
	mkDetectDir(t, home, ".alpha")
	svc := newSkillService(agents)

	target := skillFilePath(home, ".alpha")
	// Pre-seed a stale file with different content.
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(target, []byte("STALE"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := svc.Install(context.Background(), ""); err != nil {
		t.Fatalf("Install: %v", err)
	}
	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != fakeSkillContent {
		t.Errorf("expected content overwritten; got %q", got)
	}

	// Re-running install must be idempotent (no error, identical bytes).
	if _, err := svc.Install(context.Background(), ""); err != nil {
		t.Fatalf("re-install: %v", err)
	}
	got2, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("re-read: %v", err)
	}
	if string(got2) != fakeSkillContent {
		t.Errorf("re-install changed content: %q", got2)
	}
}

func TestSkillService_Install_KnownButUndetected(t *testing.T) {
	home, agents := newSkillTestEnv(t)
	// no detect dir created; alpha is "known" but not detected.
	svc := newSkillService(agents)

	results, err := svc.Install(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if len(results) != 1 || results[0].Agent != "alpha" {
		t.Fatalf("unexpected results: %+v", results)
	}
	if _, err := os.Stat(skillFilePath(home, ".alpha")); err != nil {
		t.Errorf("expected SKILL.md installed; stat err: %v", err)
	}
}

func TestSkillService_Install_UnknownAgent(t *testing.T) {
	_, agents := newSkillTestEnv(t)
	svc := newSkillService(agents)

	_, err := svc.Install(context.Background(), "nope")
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
	if !strings.Contains(err.Error(), "unknown agent") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ---- Remove -------------------------------------------------------------

func TestSkillService_Remove_NoInstalls(t *testing.T) {
	_, agents := newSkillTestEnv(t)
	svc := newSkillService(agents)

	results, err := svc.Remove(context.Background(), "")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected zero removals, got %+v", results)
	}
}

func TestSkillService_Remove_AcrossMultipleAgents(t *testing.T) {
	home, agents := newSkillTestEnv(t)
	mkDetectDir(t, home, ".alpha")
	mkDetectDir(t, home, ".gamma")
	svc := newSkillService(agents)

	if _, err := svc.Install(context.Background(), ""); err != nil {
		t.Fatalf("Install: %v", err)
	}

	results, err := svc.Remove(context.Background(), "")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 removals, got %d (%+v)", len(results), results)
	}
	gotNames := []string{results[0].Agent, results[1].Agent}
	sort.Strings(gotNames)
	wantNames := []string{"alpha", "gamma"}
	for i, w := range wantNames {
		if gotNames[i] != w {
			t.Errorf("removed[%d] = %q, want %q", i, gotNames[i], w)
		}
	}

	// Both skill dirs should be gone.
	for _, dir := range []string{".alpha", ".gamma"} {
		if _, err := os.Stat(filepath.Join(home, dir, "skills", "neo4j-cli")); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("expected %s skill dir removed; stat err: %v", dir, err)
		}
	}
}

func TestSkillService_Remove_NamedAgent_MissingTargetSilent(t *testing.T) {
	_, agents := newSkillTestEnv(t)
	svc := newSkillService(agents)

	// Removing a known agent with no install must not error and must
	// produce zero results (nothing was actually removed).
	results, err := svc.Remove(context.Background(), "alpha")
	if err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected zero removals, got %+v", results)
	}
}

func TestSkillService_Remove_UnknownAgent(t *testing.T) {
	_, agents := newSkillTestEnv(t)
	svc := newSkillService(agents)

	_, err := svc.Remove(context.Background(), "nope")
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
}

// ---- List ---------------------------------------------------------------

func TestSkillService_List_Shape(t *testing.T) {
	home, agents := newSkillTestEnv(t)
	mkDetectDir(t, home, ".alpha")
	mkDetectDir(t, home, ".beta")
	svc := newSkillService(agents)

	// Install only on alpha so detected != installed for beta.
	if _, err := svc.Install(context.Background(), "alpha"); err != nil {
		t.Fatalf("Install alpha: %v", err)
	}

	rows, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows (one per catalog agent), got %d", len(rows))
	}
	want := []service.AgentStatus{
		{Name: "alpha", DisplayName: "Alpha", Detected: true, Installed: true},
		{Name: "beta", DisplayName: "Beta", Detected: true, Installed: false},
		{Name: "gamma", DisplayName: "Gamma", Detected: false, Installed: false},
	}
	for i, w := range want {
		if rows[i] != w {
			t.Errorf("rows[%d] = %+v, want %+v", i, rows[i], w)
		}
	}
}
