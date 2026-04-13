package setup

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSelectSkillsDeduplicatesAndSorts(t *testing.T) {
	t.Parallel()

	all := []Skill{
		{Name: "beta"},
		{Name: "alpha"},
		{Name: "gamma"},
	}

	selected, err := SelectSkills(all, []string{"gamma", "alpha", "gamma"})
	if err != nil {
		t.Fatalf("select skills: %v", err)
	}

	got := skillNames(selected)
	want := []string{"alpha", "gamma"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected selected skills\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestSelectAgentsDeduplicatesAliasesAndSortsByDisplayName(t *testing.T) {
	t.Parallel()

	all := []Agent{
		{Name: "claude-code", DisplayName: "Claude Code"},
		{Name: "codex", DisplayName: "Codex"},
		{Name: "cursor", DisplayName: "Cursor"},
	}

	selected, err := SelectAgents(all, []string{"codex", "claude", "claude-code", "cursor"})
	if err != nil {
		t.Fatalf("select agents: %v", err)
	}

	got := agentNames(selected)
	want := []string{"claude-code", "codex", "cursor"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unexpected selected agents\nwant: %#v\ngot:  %#v", want, got)
	}
}

func TestSupportedAgentsUseDeclarativePaths(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	homeDir := t.TempDir()
	xdgConfigHome := filepath.Join(homeDir, ".config-alt")
	codeXHome := filepath.Join(homeDir, ".codex-alt")
	claudeConfigDir := filepath.Join(homeDir, ".claude-alt")

	if err := ensureDir(filepath.Join(homeDir, ".clawdbot")); err != nil {
		t.Fatalf("create openclaw dir: %v", err)
	}
	if err := ensureDir(codeXHome); err != nil {
		t.Fatalf("create codex dir: %v", err)
	}
	if err := ensureDir(claudeConfigDir); err != nil {
		t.Fatalf("create claude dir: %v", err)
	}
	if err := ensureDir(filepath.Join(homeDir, ".trae-cn")); err != nil {
		t.Fatalf("create trae-cn dir: %v", err)
	}

	agents, err := SupportedAgents(ResolverOptions{
		CWD:             projectDir,
		HomeDir:         homeDir,
		XDGConfigHome:   xdgConfigHome,
		CodeXHome:       codeXHome,
		ClaudeConfigDir: claudeConfigDir,
	})
	if err != nil {
		t.Fatalf("supported agents: %v", err)
	}

	byName := indexAgentsByName(agents)

	assertAgent(t, byName["claude-code"], Agent{
		Name:           "claude-code",
		DisplayName:    "Claude Code",
		ProjectRootDir: ".claude/skills",
		GlobalRootDir:  filepath.Join(claudeConfigDir, "skills"),
		Detected:       true,
	})

	assertAgent(t, byName["codex"], Agent{
		Name:           "codex",
		DisplayName:    "Codex",
		ProjectRootDir: ".agents/skills",
		GlobalRootDir:  filepath.Join(codeXHome, "skills"),
		Universal:      true,
		Detected:       true,
	})

	assertAgent(t, byName["openclaw"], Agent{
		Name:           "openclaw",
		DisplayName:    "OpenClaw",
		ProjectRootDir: "skills",
		GlobalRootDir:  filepath.Join(homeDir, ".clawdbot", "skills"),
		Detected:       true,
	})

	assertAgent(t, byName["trae-cn"], Agent{
		Name:           "trae-cn",
		DisplayName:    "Trae CN",
		ProjectRootDir: ".trae/skills",
		GlobalRootDir:  filepath.Join(homeDir, ".trae-cn", "skills"),
		Detected:       true,
	})
}

func skillNames(skills []Skill) []string {
	names := make([]string, 0, len(skills))
	for i := range skills {
		names = append(names, skills[i].Name)
	}
	return names
}

func agentNames(agents []Agent) []string {
	names := make([]string, 0, len(agents))
	for i := range agents {
		names = append(names, agents[i].Name)
	}
	return names
}

func indexAgentsByName(agents []Agent) map[string]Agent {
	index := make(map[string]Agent, len(agents))
	for _, agent := range agents {
		index[agent.Name] = agent
	}
	return index
}

func assertAgent(t *testing.T, got Agent, want Agent) {
	t.Helper()

	if got != want {
		t.Fatalf("unexpected agent\nwant: %#v\ngot:  %#v", want, got)
	}
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}
