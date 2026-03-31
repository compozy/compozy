package agent

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/compozy/looper/internal/looper/model"
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

	cmd := buildClaudeCommand("", []string{"../shared", "../docs"}, "follow the prompt file")
	if !strings.Contains(cmd, "--add-dir ../shared --add-dir ../docs") {
		t.Fatalf("expected claude command to include add-dir flags, got %q", cmd)
	}
}

func TestBuildClaudeCommandUsesInteractiveSystemPromptFlags(t *testing.T) {
	t.Parallel()

	cmd := buildClaudeCommand("", []string{"../shared"}, "follow the prompt file")

	requiredSnippets := []string{
		"CLAUDE_CODE_EXPERIMENTAL_AGENT_TEAMS=1",
		"claude --model " + model.DefaultClaudeModel,
		`--system-prompt "follow the prompt file"`,
		"--permission-mode bypassPermissions",
		"--dangerously-skip-permissions",
		"--add-dir ../shared",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(cmd, snippet) {
			t.Fatalf("expected claude command to include %q, got %q", snippet, cmd)
		}
	}

	forbiddenSnippets := []string{"--print", "--output-format", "--verbose", "--append-system-prompt"}
	for _, snippet := range forbiddenSnippets {
		if strings.Contains(cmd, snippet) {
			t.Fatalf("expected claude command to omit %q, got %q", snippet, cmd)
		}
	}
}

func TestBuildShellCommandStringForClaudeUsesSystemPrompt(t *testing.T) {
	t.Parallel()

	cmd := BuildShellCommandString(&model.RuntimeConfig{
		IDE:          model.IDEClaude,
		AddDirs:      []string{"../shared"},
		SystemPrompt: "follow the prompt file",
	})

	requiredSnippets := []string{
		"claude --model " + model.DefaultClaudeModel,
		`--system-prompt "follow the prompt file"`,
		"--permission-mode bypassPermissions",
		"--dangerously-skip-permissions",
	}
	for _, snippet := range requiredSnippets {
		if !strings.Contains(cmd, snippet) {
			t.Fatalf("expected shell preview to include %q, got %q", snippet, cmd)
		}
	}

	forbiddenSnippets := []string{"--print", "--output-format", "--verbose", "--append-system-prompt"}
	for _, snippet := range forbiddenSnippets {
		if strings.Contains(cmd, snippet) {
			t.Fatalf("expected shell preview to omit %q, got %q", snippet, cmd)
		}
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

func TestCommandPassesClaudeSystemPromptThroughRuntimeConfig(t *testing.T) {
	t.Parallel()

	systemPrompt := "follow the prompt file"
	cmd := Command(context.Background(), &model.RuntimeConfig{
		IDE:          model.IDEClaude,
		AddDirs:      []string{"../shared", "../docs"},
		SystemPrompt: systemPrompt,
	})
	if cmd == nil {
		t.Fatalf("expected claude command")
	}

	if got := argValue(cmd.Args, "--system-prompt"); got != systemPrompt {
		t.Fatalf("system prompt = %q, want %q", got, systemPrompt)
	}

	requiredArgs := []string{
		"--model", model.DefaultClaudeModel,
		"--permission-mode", "bypassPermissions",
		"--dangerously-skip-permissions",
		"--add-dir", "../shared",
		"--add-dir", "../docs",
	}
	for _, arg := range requiredArgs {
		if !containsArg(cmd.Args, arg) {
			t.Fatalf("expected claude args to include %q, got %v", arg, cmd.Args)
		}
	}

	forbiddenArgs := []string{"--print", "--output-format", "stream-json", "--verbose", "--append-system-prompt"}
	for _, arg := range forbiddenArgs {
		if containsArg(cmd.Args, arg) {
			t.Fatalf("expected claude args to omit %q, got %v", arg, cmd.Args)
		}
	}
}

func TestCommandNonClaudeIDEsRemainUnchanged(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cfg  *model.RuntimeConfig
		want []string
	}{
		{
			name: "codex",
			cfg: &model.RuntimeConfig{
				IDE:             model.IDECodex,
				AddDirs:         []string{"../shared"},
				ReasoningEffort: "high",
			},
			want: []string{
				model.IDECodex,
				"--dangerously-bypass-approvals-and-sandbox",
				"-m", model.DefaultCodexModel,
				"-c", "model_reasoning_effort=high",
				"--add-dir", "../shared",
				"exec", "--json", "-",
			},
		},
		{
			name: "droid",
			cfg: &model.RuntimeConfig{
				IDE:             model.IDEDroid,
				ReasoningEffort: "medium",
			},
			want: []string{
				model.IDEDroid,
				"exec",
				"--skip-permissions-unsafe",
				"--reasoning-effort", "medium",
				"--output-format", "stream-json",
				"--model", model.DefaultCodexModel,
			},
		},
		{
			name: "cursor",
			cfg: &model.RuntimeConfig{
				IDE:     model.IDECursor,
				AddDirs: []string{"../shared"},
			},
			want: []string{
				model.IDECursor,
				"--print",
				"--output-format", "stream-json",
				"--model", model.DefaultCursorModel,
			},
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			cmd := Command(context.Background(), tc.cfg)
			if cmd == nil {
				t.Fatalf("expected command for %s", tc.name)
			}
			if !reflect.DeepEqual(cmd.Args, tc.want) {
				t.Fatalf("unexpected args for %s\nwant: %v\ngot:  %v", tc.name, tc.want, cmd.Args)
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

func TestValidateRuntimeConfigRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		cfg  *model.RuntimeConfig
		want string
	}{
		{
			name: "invalid mode",
			cfg: &model.RuntimeConfig{
				Mode: model.ExecutionMode("invalid"),
				IDE:  model.IDECodex,
			},
			want: "invalid --mode value",
		},
		{
			name: "invalid ide",
			cfg: &model.RuntimeConfig{
				Mode: model.ExecutionModePRReview,
				IDE:  "unknown",
			},
			want: "invalid --ide value",
		},
		{
			name: "negative retries",
			cfg: &model.RuntimeConfig{
				Mode:       model.ExecutionModePRReview,
				IDE:        model.IDECodex,
				MaxRetries: -1,
			},
			want: "max-retries cannot be negative",
		},
		{
			name: "non-positive backoff",
			cfg: &model.RuntimeConfig{
				Mode:                   model.ExecutionModePRReview,
				IDE:                    model.IDECodex,
				RetryBackoffMultiplier: 0,
			},
			want: "retry-backoff-multiplier must be positive",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateRuntimeConfig(tc.cfg)
			if err == nil {
				t.Fatalf("expected validation error for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected error to contain %q, got %v", tc.want, err)
			}
		})
	}
}

func TestDisplayNameAndBuildShellCommandStringHandleUnknownInput(t *testing.T) {
	t.Parallel()

	if got := DisplayName(model.IDEClaude); got != "Claude" {
		t.Fatalf("display name = %q, want %q", got, "Claude")
	}
	if got := DisplayName("unknown"); got != "" {
		t.Fatalf("unknown display name = %q, want empty", got)
	}
	if got := BuildShellCommandString(nil); got != "" {
		t.Fatalf("nil shell command string = %q, want empty", got)
	}
	if got := BuildShellCommandString(&model.RuntimeConfig{IDE: "unknown"}); got != "" {
		t.Fatalf("unknown shell command string = %q, want empty", got)
	}
}

func TestBuildDroidAndCursorCommandPreviewHelpers(t *testing.T) {
	t.Parallel()

	if got := buildDroidCommand("", "medium"); got !=
		"droid exec --skip-permissions-unsafe --reasoning-effort medium --output-format stream-json" {
		t.Fatalf("unexpected default droid preview: %q", got)
	}
	if got := buildDroidCommand("gpt-5.4-mini", "high"); got !=
		"droid exec --skip-permissions-unsafe --reasoning-effort high --output-format stream-json --model gpt-5.4-mini" {
		t.Fatalf("unexpected custom droid preview: %q", got)
	}
	if got := buildCursorCommand("", ""); got !=
		"cursor-agent --print --output-format stream-json --model "+model.DefaultCursorModel {
		t.Fatalf("unexpected default cursor preview: %q", got)
	}
	if got := buildCursorCommand("composer-2", ""); got !=
		"cursor-agent --print --output-format stream-json --model composer-2" {
		t.Fatalf("unexpected custom cursor preview: %q", got)
	}
}

func TestEnsureAvailableHonorsDryRunAndExecutableChecks(t *testing.T) {
	if err := EnsureAvailable(&model.RuntimeConfig{DryRun: true, IDE: model.IDECodex}); err != nil {
		t.Fatalf("dry-run availability check returned error: %v", err)
	}

	binDir := t.TempDir()
	workingIDE := filepath.Join(binDir, "stub-ide")
	if err := os.WriteFile(workingIDE, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("write working stub: %v", err)
	}

	failingIDE := filepath.Join(binDir, "stub-ide-fail")
	if err := os.WriteFile(failingIDE, []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
		t.Fatalf("write failing stub: %v", err)
	}

	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	if err := assertIDEExists("stub-ide"); err != nil {
		t.Fatalf("expected stub-ide to be found: %v", err)
	}
	if err := assertExecSupported("stub-ide"); err != nil {
		t.Fatalf("expected stub-ide help command to succeed: %v", err)
	}
	if err := EnsureAvailable(&model.RuntimeConfig{IDE: "stub-ide"}); err != nil {
		t.Fatalf("expected EnsureAvailable to succeed for stub-ide: %v", err)
	}

	if err := assertIDEExists("missing-ide"); err == nil {
		t.Fatalf("expected missing ide lookup to fail")
	}
	if err := assertExecSupported("stub-ide-fail"); err == nil {
		t.Fatalf("expected failing stub help command to fail")
	}
}

func containsArg(args []string, target string) bool {
	for _, arg := range args {
		if arg == target {
			return true
		}
	}
	return false
}

func argValue(args []string, flag string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == flag {
			return args[i+1]
		}
	}
	return ""
}
