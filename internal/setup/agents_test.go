package setup

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
	if err := ensureDir(filepath.Join(xdgConfigHome, "devin")); err != nil {
		t.Fatalf("create devin dir: %v", err)
	}
	if err := ensureDir(filepath.Join(homeDir, ".trae-cn")); err != nil {
		t.Fatalf("create trae-cn dir: %v", err)
	}
	if err := ensureDir(filepath.Join(projectDir, ".omp")); err != nil {
		t.Fatalf("create OMP project dir: %v", err)
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

	assertAgent(t, byName["devin"], Agent{
		Name:           "devin",
		DisplayName:    "Devin CLI",
		ProjectRootDir: ".devin/skills",
		GlobalRootDir:  filepath.Join(xdgConfigHome, "devin", "skills"),
		Detected:       true,
	})

	assertAgent(t, byName["trae-cn"], Agent{
		Name:           "trae-cn",
		DisplayName:    "Trae CN",
		ProjectRootDir: ".trae/skills",
		GlobalRootDir:  filepath.Join(homeDir, ".trae-cn", "skills"),
		Detected:       true,
	})

	assertAgent(t, byName["pi"], Agent{
		Name:           "pi",
		DisplayName:    "Pi",
		ProjectRootDir: ".pi/skills",
		GlobalRootDir:  filepath.Join(homeDir, ".pi", "agent", "skills"),
	})

	assertAgent(t, byName["omp"], Agent{
		Name:           "omp",
		DisplayName:    "Oh My Pi",
		ProjectRootDir: ".omp/skills",
		GlobalRootDir:  filepath.Join(homeDir, ".omp", "agent", "skills"),
		Detected:       true,
	})
}

func TestSupportedAgentsDetectsOMPFromActiveProfileConfigRoot(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	homeDir := t.TempDir()
	profile := "work"
	if err := ensureDir(filepath.Join(homeDir, ".omp", "profiles", profile)); err != nil {
		t.Fatalf("create OMP profile config root: %v", err)
	}

	agents, err := SupportedAgents(ResolverOptions{
		CWD:        projectDir,
		HomeDir:    homeDir,
		OMPProfile: &profile,
	})
	if err != nil {
		t.Fatalf("supported agents: %v", err)
	}

	omp := indexAgentsByName(agents)["omp"]
	if !omp.Detected {
		t.Fatal("expected active OMP profile config root to detect Oh My Pi")
	}
}

func TestSupportedAgentsDefersOMPValidationUntilSelected(t *testing.T) {
	t.Parallel()

	invalidProfile := "UPPER"
	tests := []struct {
		name        string
		ompProfile  *string
		piConfigDir func(*testing.T) string
		wantError   string
	}{
		{
			name:       "Should defer invalid profile validation",
			ompProfile: &invalidProfile,
			wantError:  "invalid OMP profile",
		},
		{
			name: "Should defer traversing config directory validation",
			piConfigDir: func(*testing.T) string {
				return "../escape"
			},
			wantError: "OMP config root",
		},
		{
			name: "Should defer absolute config directory validation",
			piConfigDir: func(t *testing.T) string {
				return t.TempDir()
			},
			wantError: "OMP config root",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			options := ResolverOptions{
				CWD:        t.TempDir(),
				HomeDir:    t.TempDir(),
				OMPProfile: tc.ompProfile,
			}
			if tc.piConfigDir != nil {
				options.PIConfigDir = tc.piConfigDir(t)
			}

			agents, err := SupportedAgents(options)
			if err != nil {
				t.Fatalf("list supported agents: %v", err)
			}
			if _, err := SelectAgents(agents, []string{"codex"}); err != nil {
				t.Fatalf("select unrelated Codex agent: %v", err)
			}
			if _, err := SelectAgents(agents, []string{"omp"}); err == nil ||
				!strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("select OMP error = %v, want %q", err, tc.wantError)
			}
		})
	}
}

func TestSupportedAgentsResolvesOMPProfileRoots(t *testing.T) {
	t.Parallel()

	const (
		legacyProfile = "legacy"
		workProfile   = "work"
		emptyProfile  = ""
	)

	tests := []struct {
		name               string
		ompProfile         *string
		piProfile          *string
		piConfigDir        string
		useAgentOverride   bool
		wantAgentOverride  bool
		wantGlobalRelative string
	}{
		{
			name:               "Should use native root for default profile",
			wantGlobalRelative: ".omp/agent/skills",
		},
		{
			name:               "Should use profile root for canonical named profile",
			ompProfile:         stringPointer(workProfile),
			wantGlobalRelative: ".omp/profiles/work/agent/skills",
		},
		{
			name:               "Should prefer defined empty canonical profile over legacy",
			ompProfile:         stringPointer(emptyProfile),
			piProfile:          stringPointer(legacyProfile),
			wantGlobalRelative: ".omp/agent/skills",
		},
		{
			name:               "Should use legacy profile when canonical profile is undefined",
			piProfile:          stringPointer(legacyProfile),
			wantGlobalRelative: ".omp/profiles/legacy/agent/skills",
		},
		{
			name:               "Should select default profile for whitespace",
			ompProfile:         stringPointer(" \t "),
			piProfile:          stringPointer(legacyProfile),
			wantGlobalRelative: ".omp/agent/skills",
		},
		{
			name:               "Should select default profile for default sentinel",
			ompProfile:         stringPointer(" default "),
			piProfile:          stringPointer(legacyProfile),
			wantGlobalRelative: ".omp/agent/skills",
		},
		{
			name:               "Should apply custom config directory to default profile",
			piConfigDir:        ".custom-omp",
			wantGlobalRelative: ".custom-omp/agent/skills",
		},
		{
			name:               "Should apply custom config directory to named profile",
			ompProfile:         stringPointer(workProfile),
			piConfigDir:        ".custom-omp",
			wantGlobalRelative: ".custom-omp/profiles/work/agent/skills",
		},
		{
			name:              "Should honor agent directory override for default profile",
			useAgentOverride:  true,
			wantAgentOverride: true,
		},
		{
			name:               "Should ignore agent directory override for named profile",
			ompProfile:         stringPointer(workProfile),
			useAgentOverride:   true,
			wantGlobalRelative: ".omp/profiles/work/agent/skills",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			homeDir := t.TempDir()
			agentOverride := ""
			if tt.useAgentOverride {
				agentOverride = filepath.Join(homeDir, "omp-agent")
			}
			agents, err := SupportedAgents(ResolverOptions{
				CWD:              t.TempDir(),
				HomeDir:          homeDir,
				OMPProfile:       tt.ompProfile,
				PIProfile:        tt.piProfile,
				PIConfigDir:      tt.piConfigDir,
				PICodingAgentDir: agentOverride,
			})
			if err != nil {
				t.Fatalf("supported agents: %v", err)
			}

			want := filepath.Join(agentOverride, "skills")
			if !tt.wantAgentOverride {
				want = filepath.Join(homeDir, filepath.FromSlash(tt.wantGlobalRelative))
			}
			if got := indexAgentsByName(agents)["omp"].GlobalRootDir; got != want {
				t.Fatalf("unexpected OMP global root\nwant: %s\ngot:  %s", want, got)
			}
		})
	}
}

func TestSupportedAgentsRejectsInvalidOMPProfiles(t *testing.T) {
	t.Parallel()

	unsafeProfiles := []string{
		".",
		"..",
		"../escape",
		"/absolute",
		"UPPER",
		"trailing.",
		strings.Repeat("a", 65),
		"has space",
	}
	reservedProfiles := []string{
		"CON",
		"con.log",
		"prn.txt",
		"AUX",
		"aux.data",
		"nul",
		"COM0",
		"COM1",
		"COM2",
		"COM3",
		"COM4",
		"COM5",
		"COM6",
		"COM7",
		"COM8",
		"com9.log",
		"LPT0",
		"LPT1",
		"LPT2",
		"LPT3",
		"LPT4",
		"LPT5",
		"LPT6",
		"LPT7",
		"LPT8",
		"lpt9.txt",
	}

	for _, profile := range append(unsafeProfiles, reservedProfiles...) {
		profile := profile
		t.Run("Should reject "+profile, func(t *testing.T) {
			t.Parallel()

			agents, err := SupportedAgents(ResolverOptions{
				CWD:        t.TempDir(),
				HomeDir:    t.TempDir(),
				OMPProfile: &profile,
			})
			if err != nil {
				t.Fatalf("list supported agents: %v", err)
			}
			if _, err := SelectAgents(agents, []string{"omp"}); err == nil {
				t.Fatalf("expected invalid OMP profile %q to fail when OMP is selected", profile)
			} else if !strings.Contains(err.Error(), "resolve setup environment") ||
				!strings.Contains(err.Error(), "invalid OMP profile") {
				t.Fatalf("unexpected profile error: %v", err)
			}
		})
	}
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

func stringPointer(value string) *string {
	return &value
}
