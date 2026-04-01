package agent

import (
	"context"
	"strings"
	"testing"

	"github.com/compozy/compozy/internal/core/model"
)

func TestBuildCodexCommandUsesExecJSONOrder(t *testing.T) {
	t.Parallel()

	cmd := buildCodexCommand("", nil, "medium")
	want := "codex --dangerously-bypass-approvals-and-sandbox -m " + model.DefaultCodexModel +
		" -c model_reasoning_effort=medium exec --json -"
	if cmd != want {
		t.Fatalf("unexpected codex command string\nwant: %s\ngot:  %s", want, cmd)
	}
}

func TestBuildCodexCommandIncludesAddDirsBeforeExec(t *testing.T) {
	t.Parallel()

	cmd := buildCodexCommand("", []string{"../shared", "../docs"}, "medium")
	want := "codex --dangerously-bypass-approvals-and-sandbox -m " + model.DefaultCodexModel + " " +
		"-c model_reasoning_effort=medium --add-dir ../shared --add-dir ../docs exec --json -"
	if cmd != want {
		t.Fatalf("unexpected codex command string\nwant: %s\ngot:  %s", want, cmd)
	}
}

func TestBuildClaudeCommandIncludesAddDirs(t *testing.T) {
	t.Parallel()

	cmd := buildClaudeCommand("", []string{"../shared", "../docs"}, "medium")
	if !strings.Contains(cmd, "--add-dir ../shared --add-dir ../docs") {
		t.Fatalf("expected claude command to include add-dir flags, got %q", cmd)
	}
}

func TestCommandAddsDirsOnlyForSupportedIDEs(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		ide     string
		wantAdd bool
	}{
		{name: "codex", ide: model.IDECodex, wantAdd: true},
		{name: "claude", ide: model.IDEClaude, wantAdd: true},
		{name: "cursor", ide: model.IDECursor, wantAdd: false},
		{name: "droid", ide: model.IDEDroid, wantAdd: false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := Command(context.Background(), &model.RuntimeConfig{
				IDE:             tc.ide,
				AddDirs:         []string{"../shared", "../docs"},
				ReasoningEffort: "medium",
			})
			if cmd == nil {
				t.Fatalf("expected command for ide %q", tc.ide)
			}

			got := strings.Join(cmd.Args, " ")
			hasAddDir := strings.Contains(got, "--add-dir ../shared --add-dir ../docs")
			if hasAddDir != tc.wantAdd {
				t.Fatalf("unexpected add-dir presence for %s: %q", tc.ide, got)
			}
		})
	}
}

func TestValidateRuntimeConfigRejectsPRDTaskBatching(t *testing.T) {
	t.Parallel()

	cfg := &model.RuntimeConfig{
		Mode:      model.ExecutionModePRDTasks,
		IDE:       model.IDECodex,
		BatchSize: 2,
	}

	err := ValidateRuntimeConfig(cfg)
	if err == nil {
		t.Fatalf("expected validation error for prd-tasks batch size > 1")
	}
	if !strings.Contains(err.Error(), "batch size must be 1") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
