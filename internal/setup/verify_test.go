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

func TestVerifyReusableAgentsPrefersProjectScopeOverGlobal(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	homeDir := t.TempDir()
	reusableAgents := testExtensionReusableAgents(t, "architect-advisor")

	successes, failures, err := InstallReusableAgents(ReusableAgentInstallConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		ReusableAgents: reusableAgents,
		Global:         true,
	})
	if err != nil {
		t.Fatalf("install global reusable agents: %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("expected no reusable-agent installation failures, got %#v", failures)
	}
	if len(successes) == 0 {
		t.Fatal("expected bundled reusable-agent install to produce successes")
	}

	successes, failures, err = InstallReusableAgents(ReusableAgentInstallConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		ReusableAgents: reusableAgents,
		Global:         false,
	})
	if err != nil {
		t.Fatalf("install project reusable agents: %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("expected no reusable-agent installation failures, got %#v", failures)
	}
	if len(successes) == 0 {
		t.Fatal("expected project reusable-agent install to produce successes")
	}

	result, err := VerifyReusableAgents(ReusableAgentVerifyConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		ReusableAgents: reusableAgents,
	})
	if err != nil {
		t.Fatalf("verify reusable agents: %v", err)
	}
	if result.Scope != InstallScopeProject {
		t.Fatalf("expected project scope, got %q", result.Scope)
	}
	if result.HasMissing() || result.HasDrift() {
		t.Fatalf(
			"expected current reusable-agent install, got missing=%#v drift=%#v",
			result.MissingReusableAgentNames(),
			result.DriftedReusableAgentNames(),
		)
	}
}

func TestVerifyReusableAgentsFallsBackToGlobalScope(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	homeDir := t.TempDir()
	reusableAgents := testExtensionReusableAgents(t, "architect-advisor")

	successes, failures, err := InstallReusableAgents(ReusableAgentInstallConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		ReusableAgents: reusableAgents,
		Global:         true,
	})
	if err != nil {
		t.Fatalf("install global reusable agents: %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("expected no reusable-agent installation failures, got %#v", failures)
	}
	if len(successes) == 0 {
		t.Fatal("expected bundled reusable-agent install to produce successes")
	}

	result, err := VerifyReusableAgents(ReusableAgentVerifyConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		ReusableAgents: reusableAgents,
	})
	if err != nil {
		t.Fatalf("verify reusable agents: %v", err)
	}
	if result.Scope != InstallScopeGlobal {
		t.Fatalf("expected global scope, got %q", result.Scope)
	}
	if result.HasMissing() || result.HasDrift() {
		t.Fatalf(
			"expected current reusable-agent install, got missing=%#v drift=%#v",
			result.MissingReusableAgentNames(),
			result.DriftedReusableAgentNames(),
		)
	}
}

func TestVerifyReusableAgentsScopeHintWins(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	homeDir := t.TempDir()
	reusableAgents := testExtensionReusableAgents(t, "architect-advisor")

	successes, failures, err := InstallReusableAgents(ReusableAgentInstallConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		ReusableAgents: reusableAgents,
		Global:         true,
	})
	if err != nil {
		t.Fatalf("install global reusable agents: %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("expected no reusable-agent installation failures, got %#v", failures)
	}
	if len(successes) == 0 {
		t.Fatal("expected bundled reusable-agent install to produce successes")
	}

	result, err := VerifyReusableAgents(ReusableAgentVerifyConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		ReusableAgents: reusableAgents,
		ScopeHint:      InstallScopeProject,
	})
	if err != nil {
		t.Fatalf("verify reusable agents with scope hint: %v", err)
	}
	if result.Scope != InstallScopeProject {
		t.Fatalf("expected project scope from hint, got %q", result.Scope)
	}
	if !result.HasMissing() {
		t.Fatal("expected project-scope verification to report missing reusable agents")
	}
}

func TestVerifyReusableAgentsReportsProjectDriftWhenProjectOverridesGlobal(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	homeDir := t.TempDir()
	reusableAgents := testExtensionReusableAgents(t, "architect-advisor")

	successes, failures, err := InstallReusableAgents(ReusableAgentInstallConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		ReusableAgents: reusableAgents,
		Global:         true,
	})
	if err != nil {
		t.Fatalf("install global reusable agents: %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("expected no reusable-agent installation failures, got %#v", failures)
	}
	if len(successes) == 0 {
		t.Fatal("expected bundled reusable-agent install to produce successes")
	}

	successes, failures, err = InstallReusableAgents(ReusableAgentInstallConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		ReusableAgents: reusableAgents,
		Global:         false,
	})
	if err != nil {
		t.Fatalf("install project reusable agents: %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("expected no reusable-agent installation failures, got %#v", failures)
	}
	if len(successes) == 0 {
		t.Fatal("expected project reusable-agent install to produce successes")
	}

	projectAgentPath := filepath.Join(projectDir, ".compozy", "agents", "architect-advisor", "AGENT.md")
	if err := os.WriteFile(projectAgentPath, []byte("drifted\n"), 0o644); err != nil {
		t.Fatalf("write drifted reusable agent file: %v", err)
	}

	result, err := VerifyReusableAgents(ReusableAgentVerifyConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		ReusableAgents: reusableAgents,
	})
	if err != nil {
		t.Fatalf("verify reusable agents: %v", err)
	}
	if result.Scope != InstallScopeProject {
		t.Fatalf("expected project scope, got %q", result.Scope)
	}

	verified := findVerifiedReusableAgent(t, result, "architect-advisor")
	if verified.State != VerifyStateDrifted {
		t.Fatalf("expected architect-advisor to be drifted, got %q", verified.State)
	}
	if !reflect.DeepEqual(verified.Drift.ChangedFiles, []string{"AGENT.md"}) {
		t.Fatalf("expected changed AGENT.md, got %#v", verified.Drift)
	}
}

func TestVerifyReusableAgentsUsesProjectOverridesAndGlobalFallbackPerAgent(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	homeDir := t.TempDir()
	reusableAgents := testExtensionReusableAgents(t, "architect-advisor", "product-mind")

	successes, failures, err := InstallReusableAgents(ReusableAgentInstallConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		ReusableAgents: reusableAgents,
		Global:         true,
	})
	if err != nil {
		t.Fatalf("install global reusable agents: %v", err)
	}
	if len(failures) != 0 || len(successes) != len(reusableAgents) {
		t.Fatalf("unexpected global reusable-agent install result: successes=%#v failures=%#v", successes, failures)
	}

	projectOverride := make([]ReusableAgent, 0, 1)
	for i := range reusableAgents {
		if reusableAgents[i].Name == "architect-advisor" {
			projectOverride = append(projectOverride, reusableAgents[i])
		}
	}
	if len(projectOverride) != 1 {
		t.Fatalf("expected one project override reusable agent, got %#v", projectOverride)
	}
	successes, failures, err = InstallReusableAgents(ReusableAgentInstallConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		ReusableAgents: projectOverride,
		Global:         false,
	})
	if err != nil {
		t.Fatalf("install project reusable agent override: %v", err)
	}
	if len(failures) != 0 || len(successes) != 1 {
		t.Fatalf("unexpected project reusable-agent install result: successes=%#v failures=%#v", successes, failures)
	}

	result, err := VerifyReusableAgents(ReusableAgentVerifyConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		ReusableAgents: reusableAgents,
	})
	if err != nil {
		t.Fatalf("verify reusable agents: %v", err)
	}
	if result.Scope != InstallScopeProject {
		t.Fatalf("expected project scope, got %q", result.Scope)
	}
	if result.HasMissing() || result.HasDrift() {
		t.Fatalf(
			"expected merged reusable-agent install to be current, got missing=%#v drift=%#v",
			result.MissingReusableAgentNames(),
			result.DriftedReusableAgentNames(),
		)
	}

	architect := findVerifiedReusableAgent(t, result, "architect-advisor")
	if want := filepath.Join(projectDir, ".compozy", "agents", "architect-advisor"); architect.TargetPath != want {
		t.Fatalf("expected project override target path\nwant: %s\ngot:  %s", want, architect.TargetPath)
	}

	product := findVerifiedReusableAgent(t, result, "product-mind")
	if want := filepath.Join(homeDir, ".compozy", "agents", "product-mind"); product.TargetPath != want {
		t.Fatalf("expected global fallback target path\nwant: %s\ngot:  %s", want, product.TargetPath)
	}
}

func findVerifiedReusableAgent(
	t *testing.T,
	result ReusableAgentVerifyResult,
	name string,
) VerifiedReusableAgent {
	t.Helper()

	for i := range result.Agents {
		if result.Agents[i].ReusableAgent.Name == name {
			return result.Agents[i]
		}
	}
	t.Fatalf("verified reusable agent %q not found", name)
	return VerifiedReusableAgent{}
}
