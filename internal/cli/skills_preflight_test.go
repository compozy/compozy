package cli

import (
	"strings"
	"testing"

	core "github.com/compozy/compozy/internal/core"
	"github.com/compozy/compozy/internal/setup"
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
	if got := strings.Join(state.RefreshSkillNames(), ","); got != "cy-final-verify,ext-drift,ext-missing" {
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
	var bundledCalled bool
	var extensionCalled bool
	state.installBundledSkills = func(cfg setup.InstallConfig) (*setup.Result, error) {
		bundledCalled = true
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

	state := newCommandState(commandKindStart, core.ModePRDTasks)
	state.listBundledSkills = func() ([]setup.Skill, error) {
		return []setup.Skill{{Name: "cy-execute-task"}, {Name: "cy-final-verify"}}, nil
	}
	state.verifyBundledSkills = func(cfg setup.VerifyConfig) (setup.VerifyResult, error) {
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
