package setup

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAgentNameForIDE(t *testing.T) {
	t.Parallel()

	tests := map[string]string{
		"claude":       "claude-code",
		"codex":        "codex",
		"cursor-agent": "cursor",
		"droid":        "droid",
		"gemini":       "gemini-cli",
		"opencode":     "opencode",
		"pi":           "pi",
	}

	for ide, want := range tests {
		ide := ide
		want := want
		t.Run(ide, func(t *testing.T) {
			t.Parallel()

			got, err := AgentNameForIDE(ide)
			if err != nil {
				t.Fatalf("agent name for IDE %q: %v", ide, err)
			}
			if got != want {
				t.Fatalf("unexpected agent mapping for %q\nwant: %s\ngot:  %s", ide, want, got)
			}
		})
	}
}

func TestVerifyProjectInstallMatchesBundledSkills(t *testing.T) {
	t.Parallel()

	bundle := newTestBundle(t, map[string]string{
		"cy-create-prd/SKILL.md":   "---\nname: cy-create-prd\ndescription: Create a PRD\n---\n",
		"cy-final-verify/SKILL.md": "---\nname: cy-final-verify\ndescription: Verify completion\n---\n",
	})
	projectDir := t.TempDir()
	homeDir := t.TempDir()

	_, err := Install(InstallConfig{
		Bundle: bundle,
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		SkillNames: []string{"cy-create-prd", "cy-final-verify"},
		AgentNames: []string{"claude-code"},
		Mode:       InstallModeCopy,
	})
	if err != nil {
		t.Fatalf("install project skills: %v", err)
	}

	result, err := Verify(VerifyConfig{
		Bundle: bundle,
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		AgentName:  "claude-code",
		SkillNames: []string{"cy-create-prd", "cy-final-verify"},
	})
	if err != nil {
		t.Fatalf("verify project skills: %v", err)
	}

	if result.Scope != InstallScopeProject {
		t.Fatalf("expected project scope, got %q", result.Scope)
	}
	if result.Mode != InstallModeCopy {
		t.Fatalf("expected copy mode, got %q", result.Mode)
	}
	if result.HasMissing() {
		t.Fatalf("expected no missing skills, got %#v", result.MissingSkillNames())
	}
	if result.HasDrift() {
		t.Fatalf("expected no drifted skills, got %#v", result.DriftedSkillNames())
	}
}

func TestVerifyFallsBackToGlobalScopeWhenProjectSkillsAreAbsent(t *testing.T) {
	t.Parallel()

	bundle := newTestBundle(t, map[string]string{
		"cy-create-prd/SKILL.md": "---\nname: cy-create-prd\ndescription: Create a PRD\n---\n",
	})
	projectDir := t.TempDir()
	homeDir := t.TempDir()

	_, err := Install(InstallConfig{
		Bundle: bundle,
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		SkillNames: []string{"cy-create-prd"},
		AgentNames: []string{"codex"},
		Global:     true,
		Mode:       InstallModeSymlink,
	})
	if err != nil {
		t.Fatalf("install global skills: %v", err)
	}

	result, err := Verify(VerifyConfig{
		Bundle: bundle,
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		AgentName:  "codex",
		SkillNames: []string{"cy-create-prd"},
	})
	if err != nil {
		t.Fatalf("verify global skills: %v", err)
	}

	if result.Scope != InstallScopeGlobal {
		t.Fatalf("expected global scope, got %q", result.Scope)
	}
	if result.Mode != InstallModeSymlink {
		t.Fatalf("expected symlink mode, got %q", result.Mode)
	}
	if result.HasMissing() || result.HasDrift() {
		t.Fatalf(
			"expected current global install, got missing=%#v drift=%#v",
			result.MissingSkillNames(),
			result.DriftedSkillNames(),
		)
	}
}

func TestVerifyPrefersProjectScopeOverGlobalWhenProjectInstallIsPartial(t *testing.T) {
	t.Parallel()

	bundle := newTestBundle(t, map[string]string{
		"cy-create-prd/SKILL.md":   "---\nname: cy-create-prd\ndescription: Create a PRD\n---\n",
		"cy-final-verify/SKILL.md": "---\nname: cy-final-verify\ndescription: Verify completion\n---\n",
	})
	projectDir := t.TempDir()
	homeDir := t.TempDir()

	_, err := Install(InstallConfig{
		Bundle: bundle,
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		SkillNames: []string{"cy-create-prd", "cy-final-verify"},
		AgentNames: []string{"claude-code"},
		Global:     true,
		Mode:       InstallModeCopy,
	})
	if err != nil {
		t.Fatalf("install global skills: %v", err)
	}

	_, err = Install(InstallConfig{
		Bundle: bundle,
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		SkillNames: []string{"cy-create-prd"},
		AgentNames: []string{"claude-code"},
		Mode:       InstallModeCopy,
	})
	if err != nil {
		t.Fatalf("install partial project skills: %v", err)
	}

	result, err := Verify(VerifyConfig{
		Bundle: bundle,
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		AgentName:  "claude-code",
		SkillNames: []string{"cy-create-prd", "cy-final-verify"},
	})
	if err != nil {
		t.Fatalf("verify partial project skills: %v", err)
	}

	if result.Scope != InstallScopeProject {
		t.Fatalf("expected project scope, got %q", result.Scope)
	}
	if got := result.MissingSkillNames(); !reflect.DeepEqual(got, []string{"cy-final-verify"}) {
		t.Fatalf("unexpected missing skills\nwant: %#v\ngot:  %#v", []string{"cy-final-verify"}, got)
	}
}

func TestVerifyReportsChangedFilesAsDrift(t *testing.T) {
	t.Parallel()

	bundle := newTestBundle(t, map[string]string{
		"cy-create-prd/SKILL.md":   "---\nname: cy-create-prd\ndescription: Create a PRD\n---\n",
		"cy-final-verify/SKILL.md": "---\nname: cy-final-verify\ndescription: Verify completion\n---\n",
	})
	projectDir := t.TempDir()
	homeDir := t.TempDir()

	_, err := Install(InstallConfig{
		Bundle: bundle,
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		SkillNames: []string{"cy-create-prd", "cy-final-verify"},
		AgentNames: []string{"claude-code"},
		Mode:       InstallModeCopy,
	})
	if err != nil {
		t.Fatalf("install project skills: %v", err)
	}

	skillPath := filepath.Join(projectDir, ".claude", "skills", "cy-create-prd", "SKILL.md")
	if err := os.WriteFile(skillPath, []byte("drifted\n"), 0o644); err != nil {
		t.Fatalf("write drifted skill file: %v", err)
	}

	result, err := Verify(VerifyConfig{
		Bundle: bundle,
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		AgentName:  "claude-code",
		SkillNames: []string{"cy-create-prd", "cy-final-verify"},
	})
	if err != nil {
		t.Fatalf("verify drifted skills: %v", err)
	}

	if got := result.DriftedSkillNames(); !reflect.DeepEqual(got, []string{"cy-create-prd"}) {
		t.Fatalf("unexpected drifted skills\nwant: %#v\ngot:  %#v", []string{"cy-create-prd"}, got)
	}
	if !reflect.DeepEqual(result.Skills[0].Drift.ChangedFiles, []string{"SKILL.md"}) {
		t.Fatalf("expected changed SKILL.md, got %#v", result.Skills[0].Drift)
	}
}

func TestVerifyReportsExtraFilesAsDrift(t *testing.T) {
	t.Parallel()

	bundle := newTestBundle(t, map[string]string{
		"cy-create-prd/SKILL.md": "---\nname: cy-create-prd\ndescription: Create a PRD\n---\n",
	})
	projectDir := t.TempDir()
	homeDir := t.TempDir()

	_, err := Install(InstallConfig{
		Bundle: bundle,
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		SkillNames: []string{"cy-create-prd"},
		AgentNames: []string{"claude-code"},
		Mode:       InstallModeCopy,
	})
	if err != nil {
		t.Fatalf("install project skills: %v", err)
	}

	extraPath := filepath.Join(projectDir, ".claude", "skills", "cy-create-prd", "notes.txt")
	if err := os.WriteFile(extraPath, []byte("unexpected\n"), 0o644); err != nil {
		t.Fatalf("write extra file: %v", err)
	}

	result, err := Verify(VerifyConfig{
		Bundle: bundle,
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		AgentName:  "claude-code",
		SkillNames: []string{"cy-create-prd"},
	})
	if err != nil {
		t.Fatalf("verify extra file drift: %v", err)
	}

	if got := result.DriftedSkillNames(); !reflect.DeepEqual(got, []string{"cy-create-prd"}) {
		t.Fatalf("unexpected drifted skills\nwant: %#v\ngot:  %#v", []string{"cy-create-prd"}, got)
	}
	if !reflect.DeepEqual(result.Skills[0].Drift.ExtraFiles, []string{"notes.txt"}) {
		t.Fatalf("expected extra notes.txt, got %#v", result.Skills[0].Drift)
	}
}
