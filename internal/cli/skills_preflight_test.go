package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/core/model"
	"github.com/compozy/compozy/internal/setup"
	"github.com/spf13/cobra"
)

func TestRequiredSkillStateHelpers(t *testing.T) {
	t.Parallel()

	state := requiredSkillState{
		AgentName:         "codex",
		BundledSkillNames: []string{"cy-execute-task", "cy-final-verify"},
		Bundled: setup.VerifyResult{
			Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
			Scope: setup.InstallScopeProject,
			Mode:  setup.InstallModeSymlink,
			Skills: []setup.VerifiedSkill{
				{Skill: setup.Skill{Name: "cy-final-verify"}, State: setup.VerifyStateDrifted},
				{Skill: setup.Skill{Name: "cy-execute-task"}, State: setup.VerifyStateMissing},
			},
		},
		Extensions: setup.ExtensionVerifyResult{
			Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
			Skills: []setup.ExtensionVerifiedSkill{
				{
					VerifiedSkill: setup.VerifiedSkill{
						Skill: setup.Skill{Name: "ext-drift"},
						State: setup.VerifyStateDrifted,
					},
				},
				{
					VerifiedSkill: setup.VerifiedSkill{
						Skill: setup.Skill{Name: "ext-missing"},
						State: setup.VerifyStateMissing,
					},
				},
			},
		},
	}

	if got := state.Scope(); got != setup.InstallScopeProject {
		t.Fatalf("unexpected scope: %q", got)
	}
	if got := state.Mode(); got != setup.InstallModeSymlink {
		t.Fatalf("unexpected mode: %q", got)
	}
	if got := state.AgentDisplayName(); got != "Codex" {
		t.Fatalf("unexpected agent display name: %q", got)
	}
	if got := strings.Join(state.MissingSkillNames(), ","); got != "cy-execute-task,ext-missing" {
		t.Fatalf("unexpected missing skill names: %q", got)
	}
	if got := strings.Join(state.DriftedSkillNames(), ","); got != "cy-final-verify,ext-drift" {
		t.Fatalf("unexpected drifted skill names: %q", got)
	}
	if !state.HasMissing() {
		t.Fatal("expected missing skills")
	}
	if !state.HasDrift() {
		t.Fatal("expected drifted skills")
	}
	if got := strings.Join(state.BlockingMissingSkillNames(), ","); got != "cy-execute-task" {
		t.Fatalf("unexpected blocking missing skill names: %q", got)
	}
	if !state.HasBlockingMissing() {
		t.Fatal("expected blocking missing skills")
	}
	if got := strings.Join(
		state.RefreshSkillNames(),
		",",
	); got != "cy-execute-task,cy-final-verify,ext-drift,ext-missing" {
		t.Fatalf("unexpected refresh skill names: %q", got)
	}
	if !state.HasRefreshableChanges() {
		t.Fatal("expected refreshable changes")
	}
}

func TestBuildMissingSkillErrorUsesScopeSpecificGuidance(t *testing.T) {
	t.Parallel()

	base := requiredSkillState{
		Bundled: setup.VerifyResult{
			Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
			Skills: []setup.VerifiedSkill{
				{Skill: setup.Skill{Name: "cy-execute-task"}, State: setup.VerifyStateMissing},
			},
		},
	}

	tests := []struct {
		name        string
		scope       setup.InstallScope
		wantSnippet string
	}{
		{
			name:        "project scope",
			scope:       setup.InstallScopeProject,
			wantSnippet: "Run `compozy setup --agent codex` to update project skills",
		},
		{
			name:        "global scope",
			scope:       setup.InstallScopeGlobal,
			wantSnippet: "Run `compozy setup --agent codex --global` to update global skills",
		},
		{
			name:        "unknown scope",
			scope:       setup.InstallScopeUnknown,
			wantSnippet: "No compatible skills were found in project or global scope",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			state := base
			state.Bundled.Scope = tc.scope
			err := buildMissingSkillError("compozy tasks run", "codex", state)
			if err == nil || !strings.Contains(err.Error(), tc.wantSnippet) {
				t.Fatalf("expected error containing %q, got %v", tc.wantSnippet, err)
			}
		})
	}
}

func TestInteractivePreflightBlocksWhenMissingSkillRefreshIsDeclined(t *testing.T) {
	t.Parallel()

	state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
	state.isInteractive = func() bool { return true }
	state.confirmSkillRefresh = func(*cobra.Command, skillRefreshPrompt) (bool, error) {
		return false, nil
	}
	state.listBundledSkills = func() ([]setup.Skill, error) {
		return []setup.Skill{{Name: "cy-execute-task"}}, nil
	}
	state.verifyBundledSkills = func(setup.VerifyConfig) (setup.VerifyResult, error) {
		return setup.VerifyResult{
			Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
			Scope: setup.InstallScopeProject,
			Skills: []setup.VerifiedSkill{{
				Skill: setup.Skill{Name: "cy-execute-task"},
				State: setup.VerifyStateMissing,
			}},
		}, nil
	}
	state.verifyExtensionSkills = func(setup.ExtensionVerifyConfig) (setup.ExtensionVerifyResult, error) {
		return setup.ExtensionVerifyResult{}, nil
	}

	cmd := &cobra.Command{Use: "tasks run"}
	cmd.SetOut(&bytes.Buffer{})
	err := state.preflightBundledSkills(cmd, core.Config{IDE: core.IDECodex}, nil)
	if err == nil || !strings.Contains(err.Error(), "missing: cy-execute-task") {
		t.Fatalf("preflight error = %v, want blocking missing-skill error", err)
	}
}

func TestEnsureBundledSkillsCurrent(t *testing.T) {
	t.Parallel()

	baseState := requiredSkillState{
		AgentName:         "codex",
		BundledSkillNames: []string{"cy-execute-task"},
		ExtensionPacks:    []setup.SkillPackSource{{ExtensionName: "ext", ManifestPath: "/tmp/ext.json"}},
		Bundled: setup.VerifyResult{
			Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
			Scope: setup.InstallScopeProject,
		},
		Extensions: setup.ExtensionVerifyResult{
			Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
			Scope: setup.InstallScopeProject,
		},
	}

	tests := []struct {
		name       string
		bundled    setup.VerifyResult
		extensions setup.ExtensionVerifyResult
		wantErr    string
	}{
		{
			name: "success",
			bundled: setup.VerifyResult{
				Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
				Scope: setup.InstallScopeProject,
			},
			extensions: setup.ExtensionVerifyResult{
				Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
				Scope: setup.InstallScopeProject,
			},
		},
		{
			name: "missing remains",
			bundled: setup.VerifyResult{
				Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
				Scope: setup.InstallScopeProject,
				Skills: []setup.VerifiedSkill{
					{Skill: setup.Skill{Name: "cy-execute-task"}, State: setup.VerifyStateMissing},
				},
			},
			extensions: setup.ExtensionVerifyResult{
				Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
				Scope: setup.InstallScopeProject,
			},
			wantErr: "missing skills remain: cy-execute-task",
		},
		{
			name: "drift remains",
			bundled: setup.VerifyResult{
				Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
				Scope: setup.InstallScopeProject,
			},
			extensions: setup.ExtensionVerifyResult{
				Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
				Scope: setup.InstallScopeProject,
				Skills: []setup.ExtensionVerifiedSkill{
					{
						VerifiedSkill: setup.VerifiedSkill{
							Skill: setup.Skill{Name: "ext-pack"},
							State: setup.VerifyStateDrifted,
						},
					},
				},
			},
			wantErr: "drift remains: ext-pack",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ensureBundledSkillsCurrent(
				baseState,
				func(setup.VerifyConfig) (setup.VerifyResult, error) {
					return tc.bundled, nil
				},
				func(setup.ExtensionVerifyConfig) (setup.ExtensionVerifyResult, error) {
					return tc.extensions, nil
				},
			)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected success, got %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestRefreshBundledSkillsInstallsBundledAndExtensionSkills(t *testing.T) {
	t.Parallel()

	state := &commandState{}
	state.workspaceRoot = "/tmp/workspace-root"
	var bundledCalled bool
	var extensionCalled bool
	state.installBundledSkills = func(cfg setup.InstallConfig) (*setup.Result, error) {
		bundledCalled = true
		if cfg.CWD != state.workspaceRoot {
			t.Fatalf("unexpected bundled resolver cwd: %q", cfg.CWD)
		}
		if !cfg.Global {
			t.Fatal("expected global bundled refresh")
		}
		if cfg.Mode != setup.InstallModeSymlink {
			t.Fatalf("unexpected bundled install mode: %q", cfg.Mode)
		}
		if got := strings.Join(cfg.SkillNames, ","); got != "cy-execute-task,cy-final-verify" {
			t.Fatalf("unexpected bundled skill names: %q", got)
		}
		return &setup.Result{}, nil
	}
	state.installExtensionSkills = func(cfg setup.ExtensionInstallConfig) (*setup.ExtensionResult, error) {
		extensionCalled = true
		if cfg.CWD != state.workspaceRoot {
			t.Fatalf("unexpected extension resolver cwd: %q", cfg.CWD)
		}
		if !cfg.Global {
			t.Fatal("expected global extension refresh")
		}
		if cfg.Mode != setup.InstallModeSymlink {
			t.Fatalf("unexpected extension install mode: %q", cfg.Mode)
		}
		if len(cfg.Packs) != 1 || cfg.Packs[0].ExtensionName != "ext" {
			t.Fatalf("unexpected extension packs: %#v", cfg.Packs)
		}
		return &setup.ExtensionResult{}, nil
	}

	err := state.refreshBundledSkills(requiredSkillState{
		ResolverOptions:   setup.ResolverOptions{CWD: state.workspaceRoot},
		AgentName:         "codex",
		BundledSkillNames: []string{"cy-execute-task", "cy-final-verify"},
		ExtensionPacks:    []setup.SkillPackSource{{ExtensionName: "ext", ManifestPath: "/tmp/ext.json"}},
		Bundled: setup.VerifyResult{
			Scope: setup.InstallScopeGlobal,
			Mode:  setup.InstallModeSymlink,
			Skills: []setup.VerifiedSkill{
				{Skill: setup.Skill{Name: "cy-execute-task"}, State: setup.VerifyStateDrifted},
			},
		},
	})
	if err != nil {
		t.Fatalf("refresh bundled skills: %v", err)
	}
	if !bundledCalled {
		t.Fatal("expected bundled install to run")
	}
	if !extensionCalled {
		t.Fatal("expected extension install to run")
	}
}

func TestPreflightResolverOptionsPreservesEveryOMPInput(t *testing.T) {
	t.Setenv("OMP_PROFILE", "ambient")
	t.Setenv("PI_PROFILE", "ambient-legacy")
	t.Setenv("PI_CONFIG_DIR", ".ambient-omp")
	t.Setenv("PI_CODING_AGENT_DIR", "/ambient/agent")

	explicitEmpty := ""
	legacy := legacyOMPProfile
	provided := setup.ResolverOptions{
		CWD:              "/provided/workspace",
		HomeDir:          "/provided/home",
		XDGConfigHome:    "/provided/xdg",
		CodeXHome:        "/provided/codex",
		ClaudeConfigDir:  "/provided/claude",
		OMPProfile:       &explicitEmpty,
		PIProfile:        &legacy,
		PIConfigDir:      ".provided-omp",
		PICodingAgentDir: "/provided/agent",
	}

	got := preflightResolverOptions(provided, "/ambient/workspace")
	if got.CWD != provided.CWD || got.HomeDir != provided.HomeDir ||
		got.XDGConfigHome != provided.XDGConfigHome || got.CodeXHome != provided.CodeXHome ||
		got.ClaudeConfigDir != provided.ClaudeConfigDir || got.PIConfigDir != provided.PIConfigDir ||
		got.PICodingAgentDir != provided.PICodingAgentDir {
		t.Fatalf("preflight resolver lost supplied string options: got=%#v want=%#v", got, provided)
	}
	if got.OMPProfile == nil || *got.OMPProfile != "" {
		t.Fatalf("OMP profile = %#v, want explicit empty pointer", got.OMPProfile)
	}
	if got.PIProfile == nil || *got.PIProfile != legacy {
		t.Fatalf("PI profile = %#v, want %q", got.PIProfile, legacy)
	}

	provided.PIConfigDir = "   "
	provided.PICodingAgentDir = "\t"
	got = preflightResolverOptions(provided, "/ambient/workspace")
	if got.PIConfigDir != provided.PIConfigDir || got.PICodingAgentDir != provided.PICodingAgentDir {
		t.Fatalf("preflight resolver normalized supplied compatibility paths: got=%#v want=%#v", got, provided)
	}
}

func TestOMPRequiredSkillPreflightRefreshesNamedProfileAtVerifiedScope(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("OMP_PROFILE", "work")
	t.Setenv("PI_PROFILE", legacyOMPProfile)
	t.Setenv("PI_CODING_AGENT_DIR", filepath.Join(t.TempDir(), "default-agent"))

	skills, err := setup.ListBundledSkills()
	if err != nil {
		t.Fatalf("list bundled skills: %v", err)
	}
	if len(skills) < 2 {
		t.Fatalf("expected at least two bundled skills, got %d", len(skills))
	}
	skillNames := make([]string, 0, len(skills))
	for i := range skills {
		skillNames = append(skillNames, skills[i].Name)
	}
	resolver := currentResolverOptions(workspaceRoot)
	installed, err := setup.InstallBundledSkills(setup.InstallConfig{
		ResolverOptions: resolver,
		SkillNames:      skillNames,
		AgentNames:      []string{"omp"},
		Global:          true,
		Mode:            setup.InstallModeCopy,
	})
	if err != nil {
		t.Fatalf("install OMP preflight fixture: %v", err)
	}
	if len(installed.Failed) != 0 {
		t.Fatalf("install OMP preflight fixture failures: %#v", installed.Failed)
	}

	missingPath := filepath.Join(homeDir, ".omp", "profiles", "work", "agent", "skills", skillNames[0])
	if err := os.RemoveAll(missingPath); err != nil {
		t.Fatalf("remove one installed skill: %v", err)
	}

	state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
	state.workspaceRoot = workspaceRoot
	state.isInteractive = func() bool { return true }
	state.confirmSkillRefresh = func(_ *cobra.Command, prompt skillRefreshPrompt) (bool, error) {
		if prompt.AgentName != "omp" || prompt.Scope != setup.InstallScopeGlobal {
			t.Fatalf("unexpected OMP refresh prompt: %#v", prompt)
		}
		return true, nil
	}
	cmd := &cobra.Command{Use: "tasks run"}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})

	if err := state.preflightBundledSkills(cmd, core.Config{IDE: core.IDE(model.IDEOMP)}, nil); err != nil {
		t.Fatalf("run OMP required-skill preflight: %v", err)
	}
	verified, err := setup.VerifyBundledSkills(setup.VerifyConfig{
		ResolverOptions: currentResolverOptions(workspaceRoot),
		AgentName:       "omp",
		SkillNames:      skillNames,
	})
	if err != nil {
		t.Fatalf("verify refreshed OMP skills: %v", err)
	}
	if verified.Scope != setup.InstallScopeGlobal || verified.HasMissing() || verified.HasDrift() {
		t.Fatalf("unexpected refreshed OMP verification: %#v", verified)
	}
	if _, err := os.Stat(filepath.Join(homeDir, ".omp", "profiles", legacyOMPProfile)); !os.IsNotExist(err) {
		t.Fatalf("legacy profile unexpectedly modified: %v", err)
	}
}

func TestOMPPreflightReusesResolverSnapshotWhenAmbientEnvironmentChanges(t *testing.T) {
	workspaceRoot := t.TempDir()
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("OMP_PROFILE", "work")
	t.Setenv("PI_PROFILE", legacyOMPProfile)
	t.Setenv("PI_CONFIG_DIR", ".custom-omp")
	t.Setenv("PI_CODING_AGENT_DIR", "/initial/default-agent")

	state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
	state.workspaceRoot = workspaceRoot
	state.isInteractive = func() bool { return true }
	state.confirmSkillRefresh = func(*cobra.Command, skillRefreshPrompt) (bool, error) { return true, nil }
	state.listBundledSkills = func() ([]setup.Skill, error) {
		return []setup.Skill{{Name: "cy-execute-task"}}, nil
	}

	var resolverSnapshot setup.ResolverOptions
	verifyCalls := 0
	state.verifyBundledSkills = func(cfg setup.VerifyConfig) (setup.VerifyResult, error) {
		verifyCalls++
		if verifyCalls == 1 {
			resolverSnapshot = cfg.ResolverOptions
			t.Setenv("OMP_PROFILE", "changed")
			t.Setenv("PI_PROFILE", "changed-legacy")
			t.Setenv("PI_CONFIG_DIR", ".changed-omp")
			t.Setenv("PI_CODING_AGENT_DIR", "/changed/default-agent")
			return setup.VerifyResult{
				Agent: setup.Agent{Name: "omp", DisplayName: "Oh My Pi"},
				Scope: setup.InstallScopeGlobal,
				Mode:  setup.InstallModeCopy,
				Skills: []setup.VerifiedSkill{{
					Skill: setup.Skill{Name: "cy-execute-task"},
					State: setup.VerifyStateMissing,
				}},
			}, nil
		}
		if !reflect.DeepEqual(cfg.ResolverOptions, resolverSnapshot) {
			t.Fatalf("reverify resolver changed: got=%#v want=%#v", cfg.ResolverOptions, resolverSnapshot)
		}
		return setup.VerifyResult{
			Agent: setup.Agent{Name: "omp", DisplayName: "Oh My Pi"},
			Scope: setup.InstallScopeGlobal,
			Mode:  setup.InstallModeCopy,
			Skills: []setup.VerifiedSkill{{
				Skill: setup.Skill{Name: "cy-execute-task"},
				State: setup.VerifyStateCurrent,
			}},
		}, nil
	}
	state.verifyExtensionSkills = func(cfg setup.ExtensionVerifyConfig) (setup.ExtensionVerifyResult, error) {
		if !reflect.DeepEqual(cfg.ResolverOptions, resolverSnapshot) {
			t.Fatalf("extension verify resolver changed: got=%#v want=%#v", cfg.ResolverOptions, resolverSnapshot)
		}
		return setup.ExtensionVerifyResult{
			Agent: setup.Agent{Name: "omp", DisplayName: "Oh My Pi"},
			Scope: setup.InstallScopeGlobal,
			Mode:  setup.InstallModeCopy,
		}, nil
	}
	state.installBundledSkills = func(cfg setup.InstallConfig) (*setup.Result, error) {
		if !reflect.DeepEqual(cfg.ResolverOptions, resolverSnapshot) {
			t.Fatalf("refresh resolver changed: got=%#v want=%#v", cfg.ResolverOptions, resolverSnapshot)
		}
		return &setup.Result{}, nil
	}

	cmd := &cobra.Command{Use: "tasks run"}
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := state.preflightBundledSkills(cmd, core.Config{IDE: core.IDEOMP}, nil); err != nil {
		t.Fatalf("run OMP preflight: %v", err)
	}
	if verifyCalls != 2 {
		t.Fatalf("bundled verify calls = %d, want 2", verifyCalls)
	}
	if resolverSnapshot.HomeDir != homeDir || resolverSnapshot.OMPProfile == nil ||
		*resolverSnapshot.OMPProfile != "work" {
		t.Fatalf("initial resolver snapshot incomplete: %#v", resolverSnapshot)
	}
}

func TestScopeInstallFlagAndInstallScopeLabel(t *testing.T) {
	t.Parallel()

	if got := scopeInstallFlag(setup.InstallScopeGlobal); got != " --global" {
		t.Fatalf("unexpected global scope flag: %q", got)
	}
	if got := scopeInstallFlag(setup.InstallScopeProject); got != "" {
		t.Fatalf("unexpected project scope flag: %q", got)
	}
	if got := installScopeLabel(setup.InstallScopeProject); got != "project" {
		t.Fatalf("unexpected project scope label: %q", got)
	}
	if got := installScopeLabel(setup.InstallScopeUnknown); got != "unknown" {
		t.Fatalf("unexpected unknown scope label: %q", got)
	}
}

func TestVerifyRequiredSkillStateUsesSetupAgentNameAndExtensionScopeHint(t *testing.T) {
	t.Parallel()

	t.Run("Should use setup agent name, workspace cwd, and extension scope hint", func(t *testing.T) {
		t.Parallel()

		state := newCommandState(commandKindTasksRun, core.ModePRDTasks)
		state.workspaceRoot = "/tmp/workspace-root"
		state.listBundledSkills = func() ([]setup.Skill, error) {
			return []setup.Skill{{Name: "cy-execute-task"}, {Name: "cy-final-verify"}}, nil
		}
		state.verifyBundledSkills = func(cfg setup.VerifyConfig) (setup.VerifyResult, error) {
			if cfg.CWD != state.workspaceRoot {
				t.Fatalf("unexpected bundled resolver cwd: %q", cfg.CWD)
			}
			if cfg.AgentName != "codex" {
				t.Fatalf("unexpected setup agent name: %q", cfg.AgentName)
			}
			if got := strings.Join(cfg.SkillNames, ","); got != "cy-execute-task,cy-final-verify" {
				t.Fatalf("unexpected bundled skill names: %q", got)
			}
			return setup.VerifyResult{
				Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
				Scope: setup.InstallScopeGlobal,
				Mode:  setup.InstallModeSymlink,
				Skills: []setup.VerifiedSkill{
					{Skill: setup.Skill{Name: "cy-execute-task"}, State: setup.VerifyStateCurrent},
				},
			}, nil
		}
		state.verifyExtensionSkills = func(cfg setup.ExtensionVerifyConfig) (setup.ExtensionVerifyResult, error) {
			if cfg.CWD != state.workspaceRoot {
				t.Fatalf("unexpected extension resolver cwd: %q", cfg.CWD)
			}
			if cfg.AgentName != "codex" {
				t.Fatalf("unexpected extension setup agent name: %q", cfg.AgentName)
			}
			if cfg.ScopeHint != setup.InstallScopeGlobal {
				t.Fatalf("expected bundled scope hint to flow into extension verify, got %q", cfg.ScopeHint)
			}
			if len(cfg.Packs) != 1 || cfg.Packs[0].ExtensionName != "workspace-ext" {
				t.Fatalf("unexpected extension packs: %#v", cfg.Packs)
			}
			return setup.ExtensionVerifyResult{
				Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
				Scope: setup.InstallScopeGlobal,
				Mode:  setup.InstallModeSymlink,
				Skills: []setup.ExtensionVerifiedSkill{
					{
						VerifiedSkill: setup.VerifiedSkill{
							Skill: setup.Skill{Name: "ext-pack"},
							State: setup.VerifyStateCurrent,
						},
					},
				},
			}, nil
		}

		result, err := state.verifyRequiredSkillState(core.Config{IDE: core.IDECodex}, []setup.SkillPackSource{{
			ExtensionName: "workspace-ext",
			ManifestPath:  "/tmp/workspace-ext/extension.json",
		}})
		if err != nil {
			t.Fatalf("verify required skill state: %v", err)
		}
		if result.AgentName != "codex" {
			t.Fatalf("unexpected agent name: %q", result.AgentName)
		}
		if result.Scope() != setup.InstallScopeGlobal {
			t.Fatalf("unexpected scope: %q", result.Scope())
		}
		if result.Mode() != setup.InstallModeSymlink {
			t.Fatalf("unexpected mode: %q", result.Mode())
		}
	})
}

func TestRequiredSkillStateFallsBackToExtensionMetadata(t *testing.T) {
	t.Parallel()

	state := requiredSkillState{
		Bundled: setup.VerifyResult{},
		Extensions: setup.ExtensionVerifyResult{
			Agent: setup.Agent{Name: "codex", DisplayName: "Codex"},
			Scope: setup.InstallScopeGlobal,
			Mode:  setup.InstallModeSymlink,
			Skills: []setup.ExtensionVerifiedSkill{
				{
					VerifiedSkill: setup.VerifiedSkill{
						Skill: setup.Skill{Name: "ext-pack"},
						State: setup.VerifyStateCurrent,
					},
				},
			},
		},
	}

	if got := state.Scope(); got != setup.InstallScopeGlobal {
		t.Fatalf("unexpected fallback scope: %q", got)
	}
	if got := state.Mode(); got != setup.InstallModeSymlink {
		t.Fatalf("unexpected fallback mode: %q", got)
	}
	if got := state.AgentDisplayName(); got != "Codex" {
		t.Fatalf("unexpected fallback display name: %q", got)
	}
}
