package setup

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

func TestResolveDeclaredSkillPackSourceRejectsEmptyPath(t *testing.T) {
	t.Parallel()

	if _, _, err := resolveDeclaredSkillPackSource(SkillPackSource{ResolvedPath: "   "}); err == nil {
		t.Fatal("expected empty skill-pack path to fail")
	} else if !strings.Contains(err.Error(), "extension skill pack source path is required") {
		t.Fatalf("unexpected empty skill-pack error: %v", err)
	}
}

func TestResolveExtensionReusableAgentSourceRejectsEmptyPath(t *testing.T) {
	t.Parallel()

	if _, _, err := resolveExtensionReusableAgentSource(ExtensionReusableAgentSource{ResolvedPath: "   "}); err == nil {
		t.Fatal("expected empty reusable-agent path to fail")
	} else if !strings.Contains(err.Error(), "extension reusable agent source path is required") {
		t.Fatalf("unexpected empty reusable-agent error: %v", err)
	}
}

func TestParseReusableAgentRejectsUnsafeNames(t *testing.T) {
	t.Parallel()

	manifest := []byte("---\ntitle: Test Agent\ndescription: Test reusable agent\n---\n")
	bundle := fstest.MapFS{
		"AGENT.md": &fstest.MapFile{Data: manifest},
	}

	tests := []struct {
		name    string
		dir     string
		wantErr string
	}{
		{
			name:    "Current directory",
			dir:     ".",
			wantErr: `must not resolve to the current directory`,
		},
		{
			name:    "Contains traversal",
			dir:     "alpha/../missing",
			wantErr: `must not contain ".."`,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := parseReusableAgent(bundle, tt.dir); err == nil {
				t.Fatalf("expected parseReusableAgent(%q) to fail", tt.dir)
			} else if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("parseReusableAgent(%q) error = %v, want substring %q", tt.dir, err, tt.wantErr)
			}
		})
	}
}

func TestPreviewReusableAgentInstallRejectsUnsafeName(t *testing.T) {
	t.Parallel()

	_, err := PreviewReusableAgentInstall(ReusableAgentInstallConfig{
		ResolverOptions: ResolverOptions{
			CWD:     t.TempDir(),
			HomeDir: t.TempDir(),
		},
		ReusableAgents: []ReusableAgent{
			{Name: "..", Title: "Escape", Description: "unsafe"},
		},
	})
	if err == nil {
		t.Fatal("expected unsafe reusable-agent name to fail preview")
	}
	if !strings.Contains(err.Error(), `must not contain ".."`) {
		t.Fatalf("unexpected preview error: %v", err)
	}
}

func TestInstallReusableAgentsRejectsUnsafeName(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	homeDir := t.TempDir()

	successes, failures, err := InstallReusableAgents(ReusableAgentInstallConfig{
		ResolverOptions: ResolverOptions{
			CWD:     projectDir,
			HomeDir: homeDir,
		},
		ReusableAgents: []ReusableAgent{
			{
				Name:        "..",
				Title:       "Escape",
				Description: "unsafe",
				SourceFS: fstest.MapFS{
					"AGENT.md": &fstest.MapFile{Data: []byte("---\ntitle: Escape\ndescription: unsafe\n---\n")},
				},
				SourceDir: ".",
			},
		},
	})
	if err != nil {
		t.Fatalf("InstallReusableAgents() error = %v", err)
	}
	if len(successes) != 0 {
		t.Fatalf("expected no successful installs, got %#v", successes)
	}
	if len(failures) != 1 {
		t.Fatalf("expected one reusable-agent failure, got %#v", failures)
	}
	if !strings.Contains(failures[0].Error, "must not contain") {
		t.Fatalf("expected unsafe-name error, got %q", failures[0].Error)
	}
	if _, err := os.Stat(filepath.Join(projectDir, ".compozy", "agents")); !os.IsNotExist(err) {
		t.Fatalf("expected no install root to be created, stat err = %v", err)
	}
}

func TestVerifyReusableAgentsRejectsUnsafeName(t *testing.T) {
	t.Parallel()

	_, err := VerifyReusableAgents(ReusableAgentVerifyConfig{
		ResolverOptions: ResolverOptions{
			CWD:     t.TempDir(),
			HomeDir: t.TempDir(),
		},
		ReusableAgents: []ReusableAgent{
			{Name: "../escape", Title: "Escape", Description: "unsafe"},
		},
	})
	if err == nil {
		t.Fatal("expected unsafe reusable-agent name to fail verification")
	}
	if !strings.Contains(err.Error(), `must not contain ".."`) {
		t.Fatalf("unexpected verification error: %v", err)
	}
}

func TestReusableAgentVerifyResultNameListsAreSorted(t *testing.T) {
	t.Parallel()

	result := ReusableAgentVerifyResult{
		Agents: []VerifiedReusableAgent{
			{ReusableAgent: ReusableAgent{Name: "zeta"}, State: VerifyStateMissing},
			{ReusableAgent: ReusableAgent{Name: "alpha"}, State: VerifyStateMissing},
			{ReusableAgent: ReusableAgent{Name: "delta"}, State: VerifyStateDrifted},
			{ReusableAgent: ReusableAgent{Name: "beta"}, State: VerifyStateDrifted},
		},
	}

	if got := result.MissingReusableAgentNames(); !reflect.DeepEqual(got, []string{"alpha", "zeta"}) {
		t.Fatalf("unexpected missing reusable-agent names: %#v", got)
	}
	if got := result.DriftedReusableAgentNames(); !reflect.DeepEqual(got, []string{"beta", "delta"}) {
		t.Fatalf("unexpected drifted reusable-agent names: %#v", got)
	}
}
