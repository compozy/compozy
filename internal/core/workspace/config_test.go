package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestDiscoverFindsNearestWorkspaceRoot(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	nested := filepath.Join(root, "pkg", "feature", "subdir")
	if err := os.MkdirAll(filepath.Join(root, ".compozy"), 0o755); err != nil {
		t.Fatalf("mkdir .compozy: %v", err)
	}
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	got, err := Discover(context.Background(), nested)
	if err != nil {
		t.Fatalf("discover workspace: %v", err)
	}
	if mustEvalSymlinksWorkspaceTest(t, got) != mustEvalSymlinksWorkspaceTest(t, root) {
		t.Fatalf("unexpected workspace root\nwant: %q\ngot:  %q", root, got)
	}
}

func TestDiscoverFallsBackToStartDirectoryWhenWorkspaceIsMissing(t *testing.T) {
	t.Parallel()

	start := filepath.Join(t.TempDir(), "pkg", "feature")
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("mkdir start: %v", err)
	}

	got, err := Discover(context.Background(), start)
	if err != nil {
		t.Fatalf("discover workspace: %v", err)
	}
	if mustEvalSymlinksWorkspaceTest(t, got) != mustEvalSymlinksWorkspaceTest(t, start) {
		t.Fatalf("unexpected fallback root\nwant: %q\ngot:  %q", start, got)
	}
}

func TestDiscoverMemoizesSuccessfulResultPerStartDir(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "pkg", "feature", "subdir")
	if err := os.MkdirAll(filepath.Join(root, ".compozy"), 0o755); err != nil {
		t.Fatalf("mkdir .compozy: %v", err)
	}
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	previous := discoverWorkspaceRoot
	var calls atomic.Int64
	discoverWorkspaceRoot = func(ctx context.Context, startDir string) (string, error) {
		calls.Add(1)
		return previous(ctx, startDir)
	}
	t.Cleanup(func() {
		discoverWorkspaceRoot = previous
	})

	first, err := Discover(context.Background(), nested)
	if err != nil {
		t.Fatalf("first discover workspace: %v", err)
	}
	second, err := Discover(context.Background(), nested)
	if err != nil {
		t.Fatalf("second discover workspace: %v", err)
	}
	if first != second {
		t.Fatalf("memoized discover roots differ: %q vs %q", first, second)
	}
	if got := calls.Load(); got != 1 {
		t.Fatalf("discoverWorkspaceRoot calls = %d, want 1", got)
	}
}

func TestLoadConfigReturnsZeroConfigWhenFileIsMissing(t *testing.T) {
	isolateWorkspaceConfigHome(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".compozy"), 0o755); err != nil {
		t.Fatalf("mkdir .compozy: %v", err)
	}

	cfg, path, err := LoadConfig(context.Background(), root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if path != filepath.Join(root, ".compozy", "config.toml") {
		t.Fatalf("unexpected config path: %q", path)
	}
	if cfg != (ProjectConfig{}) {
		t.Fatalf("expected zero project config, got %#v", cfg)
	}
}

func TestLoadConfigReturnsGlobalConfigWhenWorkspaceFileIsMissing(t *testing.T) {
	homeDir := isolateWorkspaceConfigHome(t)
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".compozy"), 0o755); err != nil {
		t.Fatalf("mkdir .compozy: %v", err)
	}
	writeGlobalConfig(t, homeDir, `
[defaults]
ide = "claude"
model = "sonnet"

[tasks]
types = ["mobile", "api"]
`)

	cfg, path, err := LoadConfig(context.Background(), root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if path != filepath.Join(homeDir, ".compozy", "config.toml") {
		t.Fatalf("unexpected effective config path: %q", path)
	}
	if cfg.Defaults.IDE == nil || *cfg.Defaults.IDE != "claude" {
		t.Fatalf("unexpected defaults.ide: %#v", cfg.Defaults.IDE)
	}
	if cfg.Tasks.Types == nil || !equalStrings(*cfg.Tasks.Types, []string{"mobile", "api"}) {
		t.Fatalf("unexpected tasks.types: %#v", cfg.Tasks.Types)
	}
}

func TestLoadConfigRejectsUnknownFields(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeWorkspaceConfig(t, root, `
[defaults]
unknown = "value"
`)

	_, _, err := LoadConfig(context.Background(), root)
	if err == nil {
		t.Fatal("expected unknown field error")
	}
	if !strings.Contains(err.Error(), "decode workspace config") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigRejectsLegacyStartSection(t *testing.T) {
	t.Parallel()

	t.Run("Should reject legacy start section", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		writeWorkspaceConfig(t, root, `
[start]
include_completed = true
`)

		_, _, err := LoadConfig(context.Background(), root)
		if err == nil {
			t.Fatal("expected legacy start section error")
		}
		if !strings.Contains(err.Error(), "workspace config section [start] was removed; use [tasks.run] instead") {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestLoadConfigRejectsInvalidTimeout(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeWorkspaceConfig(t, root, `
[defaults]
timeout = "not-a-duration"
`)

	_, _, err := LoadConfig(context.Background(), root)
	if err == nil {
		t.Fatal("expected invalid timeout error")
	}
	if !strings.Contains(err.Error(), "defaults.timeout") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigParsesValidSections(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeWorkspaceConfig(t, root, `
[defaults]
ide = "claude"
model = "sonnet"
output_format = "text"
reasoning_effort = "high"
access_mode = "full"
timeout = "5m"
tail_lines = 0
add_dirs = []
auto_commit = true
max_retries = 0
retry_backoff_multiplier = 1.5

[tasks.run]
include_completed = false
output_format = "json"
tui = false

[fix_reviews]
concurrent = 2
batch_size = 3
include_resolved = false
output_format = "raw-json"
tui = false

[fetch_reviews]
provider = "coderabbit"
nitpicks = true

[watch_reviews]
max_rounds = 6
poll_interval = "30s"
review_timeout = "30m"
quiet_period = "20s"
auto_push = false
until_clean = true
push_remote = "origin"
push_branch = "feature"

[exec]
model = "gpt-5.5"
output_format = "json"
`)

	cfg, _, err := LoadConfig(context.Background(), root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Defaults.IDE == nil || *cfg.Defaults.IDE != "claude" {
		t.Fatalf("unexpected defaults.ide: %#v", cfg.Defaults.IDE)
	}
	if cfg.Defaults.OutputFormat == nil || *cfg.Defaults.OutputFormat != "text" {
		t.Fatalf("unexpected defaults.output_format: %#v", cfg.Defaults.OutputFormat)
	}
	if cfg.Defaults.AccessMode == nil || *cfg.Defaults.AccessMode != "full" {
		t.Fatalf("unexpected defaults.access_mode: %#v", cfg.Defaults.AccessMode)
	}
	if cfg.Defaults.Timeout == nil || *cfg.Defaults.Timeout != "5m" {
		t.Fatalf("unexpected defaults.timeout: %#v", cfg.Defaults.Timeout)
	}
	if cfg.Defaults.TailLines == nil || *cfg.Defaults.TailLines != 0 {
		t.Fatalf("unexpected defaults.tail_lines: %#v", cfg.Defaults.TailLines)
	}
	if cfg.Defaults.AddDirs == nil || len(*cfg.Defaults.AddDirs) != 0 {
		t.Fatalf("unexpected defaults.add_dirs: %#v", cfg.Defaults.AddDirs)
	}
	if cfg.Defaults.AutoCommit == nil || !*cfg.Defaults.AutoCommit {
		t.Fatalf("unexpected defaults.auto_commit: %#v", cfg.Defaults.AutoCommit)
	}
	if cfg.Tasks.Run.IncludeCompleted == nil || *cfg.Tasks.Run.IncludeCompleted {
		t.Fatalf("unexpected tasks.run.include_completed: %#v", cfg.Tasks.Run.IncludeCompleted)
	}
	if cfg.Tasks.Run.OutputFormat == nil || *cfg.Tasks.Run.OutputFormat != "json" {
		t.Fatalf("unexpected tasks.run.output_format: %#v", cfg.Tasks.Run.OutputFormat)
	}
	if cfg.Tasks.Run.TUI == nil || *cfg.Tasks.Run.TUI {
		t.Fatalf("unexpected tasks.run.tui: %#v", cfg.Tasks.Run.TUI)
	}
	if cfg.FixReviews.Concurrent == nil || *cfg.FixReviews.Concurrent != 2 {
		t.Fatalf("unexpected fix_reviews.concurrent: %#v", cfg.FixReviews.Concurrent)
	}
	if cfg.FixReviews.OutputFormat == nil || *cfg.FixReviews.OutputFormat != "raw-json" {
		t.Fatalf("unexpected fix_reviews.output_format: %#v", cfg.FixReviews.OutputFormat)
	}
	if cfg.FixReviews.TUI == nil || *cfg.FixReviews.TUI {
		t.Fatalf("unexpected fix_reviews.tui: %#v", cfg.FixReviews.TUI)
	}
	if cfg.FetchReviews.Provider == nil || *cfg.FetchReviews.Provider != "coderabbit" {
		t.Fatalf("unexpected fetch_reviews.provider: %#v", cfg.FetchReviews.Provider)
	}
	if cfg.FetchReviews.Nitpicks == nil || !*cfg.FetchReviews.Nitpicks {
		t.Fatalf("unexpected fetch_reviews.nitpicks: %#v", cfg.FetchReviews.Nitpicks)
	}
	if cfg.WatchReviews.MaxRounds == nil || *cfg.WatchReviews.MaxRounds != 6 {
		t.Fatalf("unexpected watch_reviews.max_rounds: %#v", cfg.WatchReviews.MaxRounds)
	}
	if cfg.WatchReviews.PollInterval == nil || *cfg.WatchReviews.PollInterval != "30s" {
		t.Fatalf("unexpected watch_reviews.poll_interval: %#v", cfg.WatchReviews.PollInterval)
	}
	if cfg.WatchReviews.ReviewTimeout == nil || *cfg.WatchReviews.ReviewTimeout != "30m" {
		t.Fatalf("unexpected watch_reviews.review_timeout: %#v", cfg.WatchReviews.ReviewTimeout)
	}
	if cfg.WatchReviews.QuietPeriod == nil || *cfg.WatchReviews.QuietPeriod != "20s" {
		t.Fatalf("unexpected watch_reviews.quiet_period: %#v", cfg.WatchReviews.QuietPeriod)
	}
	if cfg.WatchReviews.AutoPush == nil || *cfg.WatchReviews.AutoPush {
		t.Fatalf("unexpected watch_reviews.auto_push: %#v", cfg.WatchReviews.AutoPush)
	}
	if cfg.WatchReviews.UntilClean == nil || !*cfg.WatchReviews.UntilClean {
		t.Fatalf("unexpected watch_reviews.until_clean: %#v", cfg.WatchReviews.UntilClean)
	}
	if cfg.WatchReviews.PushRemote == nil || *cfg.WatchReviews.PushRemote != "origin" {
		t.Fatalf("unexpected watch_reviews.push_remote: %#v", cfg.WatchReviews.PushRemote)
	}
	if cfg.WatchReviews.PushBranch == nil || *cfg.WatchReviews.PushBranch != "feature" {
		t.Fatalf("unexpected watch_reviews.push_branch: %#v", cfg.WatchReviews.PushBranch)
	}
	if cfg.Exec.Model == nil || *cfg.Exec.Model != "gpt-5.5" {
		t.Fatalf("unexpected exec.model: %#v", cfg.Exec.Model)
	}
	if cfg.Exec.OutputFormat == nil || *cfg.Exec.OutputFormat != "json" {
		t.Fatalf("unexpected exec.output_format: %#v", cfg.Exec.OutputFormat)
	}
}

func TestLoadConfigAcceptsRawJSONExecOutputFormat(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeWorkspaceConfig(t, root, `
[exec]
output_format = "raw-json"
`)

	cfg, _, err := LoadConfig(context.Background(), root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Exec.OutputFormat == nil || *cfg.Exec.OutputFormat != "raw-json" {
		t.Fatalf("unexpected exec.output_format: %#v", cfg.Exec.OutputFormat)
	}
}

func TestLoadConfigTaskTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		content   string
		wantErr   string
		wantTypes []string
		wantNil   bool
	}{
		{
			name:    "leaves task types nil when section is absent",
			content: ``,
			wantNil: true,
		},
		{
			name: "rejects explicit empty list",
			content: `
[tasks]
types = []
`,
			wantErr: "workspace config tasks.types cannot be empty",
		},
		{
			name: "rejects duplicates",
			content: `
[tasks]
types = ["frontend", "frontend"]
`,
			wantErr: `duplicate task type "frontend"`,
		},
		{
			name: "rejects invalid slug",
			content: `
[tasks]
types = ["Invalid Slug"]
`,
			wantErr: `Invalid Slug`,
		},
		{
			name: "preserves valid custom list",
			content: `
[tasks]
types = ["frontend", "backend"]
`,
			wantTypes: []string{"frontend", "backend"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeWorkspaceConfig(t, root, tt.content)

			cfg, _, err := LoadConfig(context.Background(), root)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q", tt.wantErr)
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("unexpected error\nwant substring: %q\ngot: %v", tt.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("load config: %v", err)
			}
			if tt.wantNil {
				if cfg.Tasks.Types != nil {
					t.Fatalf("expected tasks.types to be nil, got %#v", cfg.Tasks.Types)
				}
				return
			}
			if cfg.Tasks.Types == nil {
				t.Fatal("expected tasks.types to be populated")
			}
			if !equalStrings(*cfg.Tasks.Types, tt.wantTypes) {
				t.Fatalf("unexpected task types\nwant: %#v\ngot:  %#v", tt.wantTypes, *cfg.Tasks.Types)
			}
		})
	}
}

func TestResolveLoadsConfigFromNearestWorkspace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	start := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("mkdir start: %v", err)
	}
	writeWorkspaceConfig(t, root, `
[defaults]
ide = "claude"
`)

	workspaceCtx, err := Resolve(context.Background(), start)
	if err != nil {
		t.Fatalf("resolve workspace: %v", err)
	}
	if mustEvalSymlinksWorkspaceTest(t, workspaceCtx.Root) != mustEvalSymlinksWorkspaceTest(t, root) {
		t.Fatalf("unexpected workspace root: %q", workspaceCtx.Root)
	}
	if workspaceCtx.Config.Defaults.IDE == nil || *workspaceCtx.Config.Defaults.IDE != "claude" {
		t.Fatalf("unexpected loaded ide: %#v", workspaceCtx.Config.Defaults.IDE)
	}
}

func TestLoadConfigParsesStartTaskRuntimeRules(t *testing.T) {
	isolateWorkspaceConfigHome(t)

	root := t.TempDir()
	writeWorkspaceConfig(t, root, `
[tasks.run]
[[tasks.run.task_runtime_rules]]
type = "frontend"
ide = "codex"
model = "gpt-5.5"
reasoning_effort = "xhigh"
`)

	cfg, _, err := LoadConfig(context.Background(), root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Tasks.Run.TaskRuntimeRules == nil || len(*cfg.Tasks.Run.TaskRuntimeRules) != 1 {
		t.Fatalf("unexpected tasks.run.task_runtime_rules: %#v", cfg.Tasks.Run.TaskRuntimeRules)
	}
	rule := (*cfg.Tasks.Run.TaskRuntimeRules)[0]
	if rule.Type == nil || *rule.Type != "frontend" {
		t.Fatalf("unexpected rule type: %#v", rule.Type)
	}
	if rule.IDE == nil || *rule.IDE != "codex" {
		t.Fatalf("unexpected rule ide: %#v", rule.IDE)
	}
	if rule.Model == nil || *rule.Model != "gpt-5.5" {
		t.Fatalf("unexpected rule model: %#v", rule.Model)
	}
	if rule.ReasoningEffort == nil || *rule.ReasoningEffort != "xhigh" {
		t.Fatalf("unexpected rule reasoning: %#v", rule.ReasoningEffort)
	}
}

func TestLoadConfigMergesStartTaskRuntimeRulesByType(t *testing.T) {
	homeDir := isolateWorkspaceConfigHome(t)
	root := t.TempDir()
	writeGlobalConfig(t, homeDir, `
[tasks.run]
[[tasks.run.task_runtime_rules]]
type = "frontend"
ide = "claude"
model = "sonnet"

[[tasks.run.task_runtime_rules]]
type = "backend"
ide = "codex"
`)
	writeWorkspaceConfig(t, root, `
[tasks.run]
[[tasks.run.task_runtime_rules]]
type = "frontend"
ide = "codex"
model = "gpt-5.5"
`)

	cfg, _, err := LoadConfig(context.Background(), root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Tasks.Run.TaskRuntimeRules == nil || len(*cfg.Tasks.Run.TaskRuntimeRules) != 2 {
		t.Fatalf("unexpected merged tasks.run.task_runtime_rules: %#v", cfg.Tasks.Run.TaskRuntimeRules)
	}
	rules := *cfg.Tasks.Run.TaskRuntimeRules
	if rules[0].Type == nil || *rules[0].Type != "frontend" || rules[0].IDE == nil || *rules[0].IDE != "codex" {
		t.Fatalf("expected workspace frontend override to replace global rule, got %#v", rules[0])
	}
	if rules[1].Type == nil || *rules[1].Type != "backend" || rules[1].IDE == nil || *rules[1].IDE != "codex" {
		t.Fatalf("expected backend global rule to remain, got %#v", rules[1])
	}
}

func TestLoadConfigRejectsUnsupportedStartTaskRuntimeRuleID(t *testing.T) {
	isolateWorkspaceConfigHome(t)

	root := t.TempDir()
	writeWorkspaceConfig(t, root, `
[tasks.run]
[[tasks.run.task_runtime_rules]]
id = "task_01"
ide = "codex"
`)

	_, _, err := LoadConfig(context.Background(), root)
	if err == nil {
		t.Fatal("expected unsupported tasks.run.task_runtime_rules id error")
	}
	if !strings.Contains(err.Error(), "tasks.run.task_runtime_rules[0].id is not supported") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigRejectsInvalidStartTaskRuntimeRuleReasoningEffort(t *testing.T) {
	isolateWorkspaceConfigHome(t)

	root := t.TempDir()
	writeWorkspaceConfig(t, root, `
[tasks.run]
[[tasks.run.task_runtime_rules]]
type = "frontend"
reasoning_effort = "turbo"
`)

	_, _, err := LoadConfig(context.Background(), root)
	if err == nil {
		t.Fatal("expected invalid tasks.run.task_runtime_rules reasoning_effort error")
	}
	if !strings.Contains(err.Error(), "tasks.run.task_runtime_rules[0].reasoning_effort") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResolveLoadsTaskTypesFromNearestWorkspace(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	start := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("mkdir start: %v", err)
	}
	writeWorkspaceConfig(t, root, `
[tasks]
types = ["mobile", "api"]
`)

	workspaceCtx, err := Resolve(context.Background(), start)
	if err != nil {
		t.Fatalf("resolve workspace: %v", err)
	}
	if workspaceCtx.Config.Tasks.Types == nil {
		t.Fatal("expected task types to be populated")
	}
	if !equalStrings(*workspaceCtx.Config.Tasks.Types, []string{"mobile", "api"}) {
		t.Fatalf("unexpected loaded task types: %#v", *workspaceCtx.Config.Tasks.Types)
	}
}

func TestLoadConfigRejectsInvalidAccessMode(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeWorkspaceConfig(t, root, `
[defaults]
access_mode = "invalid"
`)

	_, _, err := LoadConfig(context.Background(), root)
	if err == nil {
		t.Fatal("expected invalid access mode error")
	}
	if !strings.Contains(err.Error(), "defaults.access_mode") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigRejectsInvalidExecOutputFormat(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeWorkspaceConfig(t, root, `
[exec]
output_format = "yaml"
`)

	_, _, err := LoadConfig(context.Background(), root)
	if err == nil {
		t.Fatal("expected invalid exec output format error")
	}
	if !strings.Contains(err.Error(), "exec.output_format") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigRejectsExecTUIWhenDefaultsOutputFormatIsJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeWorkspaceConfig(t, root, `
[defaults]
output_format = "json"

[exec]
tui = true
`)

	_, _, err := LoadConfig(context.Background(), root)
	if err == nil {
		t.Fatal("expected invalid exec tui/output format combination")
	}
	if !strings.Contains(err.Error(), "exec.tui cannot be true") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigRejectsStartTUIWhenDefaultsOutputFormatIsJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeWorkspaceConfig(t, root, `
[defaults]
output_format = "json"

[tasks.run]
tui = true
`)

	_, _, err := LoadConfig(context.Background(), root)
	if err == nil {
		t.Fatal("expected invalid start tui/output format combination")
	}
	if !strings.Contains(err.Error(), "tasks.run.tui cannot be true") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigRejectsFixReviewsTUIWhenDefaultsOutputFormatIsJSON(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeWorkspaceConfig(t, root, `
[defaults]
output_format = "json"

[fix_reviews]
tui = true
`)

	_, _, err := LoadConfig(context.Background(), root)
	if err == nil {
		t.Fatal("expected invalid fix_reviews tui/output format combination")
	}
	if !strings.Contains(err.Error(), "fix_reviews.tui cannot be true") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigRejectsInvalidSharedRuntimeOverrideValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "defaults reasoning effort uses shared validation",
			content: `
[defaults]
reasoning_effort = "turbo"
`,
			wantErr: "defaults.reasoning_effort",
		},
		{
			name: "exec retry backoff uses shared validation",
			content: `
[exec]
retry_backoff_multiplier = 0
`,
			wantErr: "exec.retry_backoff_multiplier",
		},
		{
			name: "defaults add dirs reject unsupported defaults ide",
			content: `
[defaults]
ide = "cursor-agent"
add_dirs = ["../shared"]
`,
			wantErr: "defaults.add_dirs",
		},
		{
			name: "exec add dirs reject unsupported exec ide",
			content: `
[exec]
ide = "cursor-agent"
add_dirs = ["../shared"]
`,
			wantErr: "exec.add_dirs",
		},
		{
			name: "defaults add dirs inherited by exec reject unsupported exec ide",
			content: `
[defaults]
add_dirs = ["../shared"]

[exec]
ide = "cursor-agent"
`,
			wantErr: "defaults.add_dirs",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeWorkspaceConfig(t, root, tt.content)

			_, _, err := LoadConfig(context.Background(), root)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("unexpected error\nwant substring: %q\ngot: %v", tt.wantErr, err)
			}
		})
	}
}

func TestLoadConfigRejectsInvalidFixReviewsValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "concurrent must be positive",
			content: `
[fix_reviews]
concurrent = 0
`,
			wantErr: "fix_reviews.concurrent",
		},
		{
			name: "batch size must be positive",
			content: `
[fix_reviews]
batch_size = 0
`,
			wantErr: "fix_reviews.batch_size",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeWorkspaceConfig(t, root, tt.content)

			_, _, err := LoadConfig(context.Background(), root)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("unexpected error\nwant substring: %q\ngot: %v", tt.wantErr, err)
			}
		})
	}
}

func TestLoadConfigRejectsEmptyFetchReviewsProvider(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeWorkspaceConfig(t, root, `
[fetch_reviews]
provider = "   "
`)

	_, _, err := LoadConfig(context.Background(), root)
	if err == nil {
		t.Fatal("expected empty provider error")
	}
	if !strings.Contains(err.Error(), "fetch_reviews.provider") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigMergesWatchReviewsPrecedence(t *testing.T) {
	homeDir := isolateWorkspaceConfigHome(t)
	root := t.TempDir()
	writeGlobalConfig(t, homeDir, `
[defaults]
auto_commit = true

[watch_reviews]
max_rounds = 3
poll_interval = "45s"
review_timeout = "10m"
quiet_period = "5s"
until_clean = true
auto_push = false
push_remote = "origin"
push_branch = "main"

[fix_reviews]
concurrent = 2
include_resolved = true

[fetch_reviews]
provider = "coderabbit"
nitpicks = false
`)
	writeWorkspaceConfig(t, root, `
[watch_reviews]
max_rounds = 5
poll_interval = "15s"
auto_push = true
push_remote = "upstream"
push_branch = "feature"

[fix_reviews]
concurrent = 4

[fetch_reviews]
nitpicks = true
`)

	cfg, _, err := LoadConfig(context.Background(), root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.WatchReviews.MaxRounds == nil || *cfg.WatchReviews.MaxRounds != 5 {
		t.Fatalf("watch_reviews.max_rounds = %#v, want 5", cfg.WatchReviews.MaxRounds)
	}
	if cfg.WatchReviews.PollInterval == nil || *cfg.WatchReviews.PollInterval != "15s" {
		t.Fatalf("watch_reviews.poll_interval = %#v, want 15s", cfg.WatchReviews.PollInterval)
	}
	if cfg.WatchReviews.ReviewTimeout == nil || *cfg.WatchReviews.ReviewTimeout != "10m" {
		t.Fatalf("watch_reviews.review_timeout = %#v, want 10m", cfg.WatchReviews.ReviewTimeout)
	}
	if cfg.WatchReviews.QuietPeriod == nil || *cfg.WatchReviews.QuietPeriod != "5s" {
		t.Fatalf("watch_reviews.quiet_period = %#v, want 5s", cfg.WatchReviews.QuietPeriod)
	}
	if cfg.WatchReviews.AutoPush == nil || !*cfg.WatchReviews.AutoPush {
		t.Fatalf("watch_reviews.auto_push = %#v, want true", cfg.WatchReviews.AutoPush)
	}
	if cfg.WatchReviews.UntilClean == nil || !*cfg.WatchReviews.UntilClean {
		t.Fatalf("watch_reviews.until_clean = %#v, want true", cfg.WatchReviews.UntilClean)
	}
	if cfg.WatchReviews.PushRemote == nil || *cfg.WatchReviews.PushRemote != "upstream" {
		t.Fatalf("watch_reviews.push_remote = %#v, want upstream", cfg.WatchReviews.PushRemote)
	}
	if cfg.WatchReviews.PushBranch == nil || *cfg.WatchReviews.PushBranch != "feature" {
		t.Fatalf("watch_reviews.push_branch = %#v, want feature", cfg.WatchReviews.PushBranch)
	}
	if cfg.FixReviews.Concurrent == nil || *cfg.FixReviews.Concurrent != 4 {
		t.Fatalf("fix_reviews.concurrent = %#v, want 4", cfg.FixReviews.Concurrent)
	}
	if cfg.FixReviews.IncludeResolved == nil || !*cfg.FixReviews.IncludeResolved {
		t.Fatalf("fix_reviews.include_resolved = %#v, want true", cfg.FixReviews.IncludeResolved)
	}
	if cfg.FetchReviews.Provider == nil || *cfg.FetchReviews.Provider != "coderabbit" {
		t.Fatalf("fetch_reviews.provider = %#v, want coderabbit", cfg.FetchReviews.Provider)
	}
	if cfg.FetchReviews.Nitpicks == nil || !*cfg.FetchReviews.Nitpicks {
		t.Fatalf("fetch_reviews.nitpicks = %#v, want true", cfg.FetchReviews.Nitpicks)
	}
}

func TestLoadConfigRejectsInvalidWatchReviewsValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "max rounds must be positive when until clean is true",
			content: `
[watch_reviews]
until_clean = true
max_rounds = 0
`,
			wantErr: "watch_reviews.max_rounds",
		},
		{
			name: "poll interval must be positive",
			content: `
[watch_reviews]
poll_interval = "0s"
`,
			wantErr: "watch_reviews.poll_interval",
		},
		{
			name: "review timeout must be positive",
			content: `
[watch_reviews]
review_timeout = "-1s"
`,
			wantErr: "watch_reviews.review_timeout",
		},
		{
			name: "quiet period must be positive",
			content: `
[watch_reviews]
quiet_period = "0s"
`,
			wantErr: "watch_reviews.quiet_period",
		},
		{
			name: "push remote cannot be empty",
			content: `
[watch_reviews]
push_remote = "  "
push_branch = "feature"
`,
			wantErr: "watch_reviews.push_remote",
		},
		{
			name: "push target must be complete",
			content: `
[watch_reviews]
push_remote = "origin"
`,
			wantErr: "watch_reviews.push_remote",
		},
		{
			name: "auto push requires auto commit in config",
			content: `
[defaults]
auto_commit = false

[watch_reviews]
auto_push = true
`,
			wantErr: "watch_reviews.auto_push",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			writeWorkspaceConfig(t, root, tt.content)

			_, _, err := LoadConfig(context.Background(), root)
			if err == nil {
				t.Fatalf("expected error containing %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("unexpected error\nwant substring: %q\ngot: %v", tt.wantErr, err)
			}
		})
	}
}

func TestDiscoverResolvesSymlinkStartDirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	realStart := filepath.Join(root, "pkg", "feature")
	if err := os.MkdirAll(filepath.Join(root, ".compozy"), 0o755); err != nil {
		t.Fatalf("mkdir .compozy: %v", err)
	}
	if err := os.MkdirAll(realStart, 0o755); err != nil {
		t.Fatalf("mkdir real start: %v", err)
	}

	link := filepath.Join(t.TempDir(), "feature-link")
	if err := os.Symlink(realStart, link); err != nil {
		t.Fatalf("symlink start dir: %v", err)
	}

	got, err := Discover(context.Background(), link)
	if err != nil {
		t.Fatalf("discover workspace: %v", err)
	}
	if mustEvalSymlinksWorkspaceTest(t, got) != mustEvalSymlinksWorkspaceTest(t, root) {
		t.Fatalf("unexpected workspace root\nwant: %q\ngot:  %q", root, got)
	}
}

func TestDiscoverReturnsContextErrorWhenCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := filepath.Join(t.TempDir(), "pkg", "feature")
	if err := os.MkdirAll(start, 0o755); err != nil {
		t.Fatalf("mkdir start: %v", err)
	}

	_, err := Discover(ctx, start)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}
}

func TestLoadConfigReturnsContextErrorWhenCanceled(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	root := t.TempDir()
	_, _, err := LoadConfig(ctx, root)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled error, got %v", err)
	}
}

func TestLoadConfigSoundSection(t *testing.T) {
	cases := []struct {
		name          string
		content       string
		wantErr       string
		wantEnabled   *bool
		wantCompleted *string
		wantFailed    *string
	}{
		{
			name: "Should parse a fully populated [sound] section",
			content: `
[sound]
enabled = true
on_completed = "glass"
on_failed = "basso"
`,
			wantEnabled:   ptrBool(true),
			wantCompleted: ptrString("glass"),
			wantFailed:    ptrString("basso"),
		},
		{
			name:    "Should leave [sound] fields nil when the section is absent",
			content: ``,
		},
		{
			name: "Should reject whitespace-only sound.on_completed",
			content: `
[sound]
on_completed = "   "
`,
			wantErr: "sound.on_completed",
		},
		{
			name: "Should reject whitespace-only sound.on_failed",
			content: `
[sound]
on_failed = "\t"
`,
			wantErr: "sound.on_failed",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isolateWorkspaceConfigHome(t)
			root := t.TempDir()
			writeWorkspaceConfig(t, root, tc.content)

			cfg, _, err := LoadConfig(context.Background(), root)
			if tc.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tc.wantErr)
				}
				if !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("unexpected error: got %q, want substring %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("load config: %v", err)
			}
			assertOptionalBool(t, "sound.enabled", cfg.Sound.Enabled, tc.wantEnabled)
			assertOptionalString(t, "sound.on_completed", cfg.Sound.OnCompleted, tc.wantCompleted)
			assertOptionalString(t, "sound.on_failed", cfg.Sound.OnFailed, tc.wantFailed)
		})
	}
}

func TestLoadConfigMergesWorkspaceOverGlobalConfig(t *testing.T) {
	homeDir := isolateWorkspaceConfigHome(t)
	root := t.TempDir()
	writeGlobalConfig(t, homeDir, `
[defaults]
ide = "claude"
model = "sonnet"
access_mode = "default"

[tasks.run]
include_completed = false
`)
	writeWorkspaceConfig(t, root, `
[defaults]
model = "gpt-5.5"

[tasks.run]
include_completed = true
`)

	cfg, path, err := LoadConfig(context.Background(), root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if path != filepath.Join(root, ".compozy", "config.toml") {
		t.Fatalf("unexpected effective config path: %q", path)
	}
	if cfg.Defaults.IDE == nil || *cfg.Defaults.IDE != "claude" {
		t.Fatalf("expected global defaults.ide fallback, got %#v", cfg.Defaults.IDE)
	}
	if cfg.Defaults.Model == nil || *cfg.Defaults.Model != "gpt-5.5" {
		t.Fatalf("expected workspace defaults.model override, got %#v", cfg.Defaults.Model)
	}
	if cfg.Tasks.Run.IncludeCompleted == nil || !*cfg.Tasks.Run.IncludeCompleted {
		t.Fatalf("expected workspace tasks.run.include_completed override, got %#v", cfg.Tasks.Run.IncludeCompleted)
	}
}

func TestLoadConfigKeepsWorkspaceDefaultsAheadOfGlobalCommandOverrides(t *testing.T) {
	homeDir := isolateWorkspaceConfigHome(t)
	root := t.TempDir()
	writeGlobalConfig(t, homeDir, `
[defaults]
model = "sonnet"
output_format = "json"

[tasks.run]
output_format = "raw-json"
tui = false

[exec]
model = "gpt-5.5"
output_format = "raw-json"
verbose = true
`)
	writeWorkspaceConfig(t, root, `
[defaults]
model = "o4-mini"
output_format = "text"
`)

	cfg, _, err := LoadConfig(context.Background(), root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Defaults.Model == nil || *cfg.Defaults.Model != "o4-mini" {
		t.Fatalf("expected workspace defaults.model to win, got %#v", cfg.Defaults.Model)
	}
	if cfg.Defaults.OutputFormat == nil || *cfg.Defaults.OutputFormat != "text" {
		t.Fatalf("expected workspace defaults.output_format to win, got %#v", cfg.Defaults.OutputFormat)
	}
	if cfg.Tasks.Run.OutputFormat != nil {
		t.Fatalf("expected global tasks.run.output_format to stay shadowed, got %#v", cfg.Tasks.Run.OutputFormat)
	}
	if cfg.Tasks.Run.TUI == nil || *cfg.Tasks.Run.TUI {
		t.Fatalf("expected global tasks.run.tui to remain available, got %#v", cfg.Tasks.Run.TUI)
	}
	if cfg.Exec.Model != nil {
		t.Fatalf("expected global exec.model to stay shadowed, got %#v", cfg.Exec.Model)
	}
	if cfg.Exec.OutputFormat != nil {
		t.Fatalf("expected global exec.output_format to stay shadowed, got %#v", cfg.Exec.OutputFormat)
	}
	if cfg.Exec.Verbose == nil || !*cfg.Exec.Verbose {
		t.Fatalf("expected global exec.verbose to remain available, got %#v", cfg.Exec.Verbose)
	}
}

func TestLoadConfigRejectsInvalidMergedCrossScopeCombination(t *testing.T) {
	homeDir := isolateWorkspaceConfigHome(t)
	root := t.TempDir()
	writeGlobalConfig(t, homeDir, `
[defaults]
output_format = "json"
`)
	writeWorkspaceConfig(t, root, `
[tasks.run]
tui = true
`)

	_, _, err := LoadConfig(context.Background(), root)
	if err == nil {
		t.Fatal("expected merged config validation error")
	}
	if !strings.Contains(err.Error(), "effective config tasks.run.tui cannot be true") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigResolvesGlobalAddDirsRelativeToHome(t *testing.T) {
	homeDir := isolateWorkspaceConfigHome(t)
	root := t.TempDir()
	writeGlobalConfig(t, homeDir, `
[defaults]
add_dirs = ["shared", "/opt/tools"]
`)

	cfg, _, err := LoadConfig(context.Background(), root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Defaults.AddDirs == nil {
		t.Fatal("expected defaults.add_dirs to be populated")
	}
	want := []string{
		filepath.Join(homeDir, "shared"),
		"/opt/tools",
	}
	if !equalStrings(*cfg.Defaults.AddDirs, want) {
		t.Fatalf("unexpected defaults.add_dirs\nwant: %#v\ngot:  %#v", want, *cfg.Defaults.AddDirs)
	}
}

func TestLoadConfigReturnsErrorWhenHomeLookupFails(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceConfig(t, root, `
[defaults]
ide = "claude"
`)
	homeErr := errors.New("home unavailable")
	stubWorkspaceUserHomeDir(t, func() (string, error) {
		return "", homeErr
	})

	_, _, err := LoadConfig(context.Background(), root)
	if err == nil {
		t.Fatal("expected load config error")
	}
	if !errors.Is(err, homeErr) {
		t.Fatalf("expected home lookup error, got %v", err)
	}
	if !strings.Contains(err.Error(), "resolve config paths: lookup user home directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadConfigReturnsErrorWhenGlobalBaseDirCannotBeResolved(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceConfig(t, root, `
[defaults]
ide = "claude"
`)
	stubWorkspaceUserHomeDir(t, func() (string, error) {
		return " ", nil
	})

	_, _, err := LoadConfig(context.Background(), root)
	if err == nil {
		t.Fatal("expected load config error")
	}
	if !strings.Contains(err.Error(), "resolve config paths: resolve global config base dir") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "base directory is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func ptrBool(b bool) *bool       { return &b }
func ptrString(s string) *string { return &s }

func assertOptionalBool(t *testing.T, field string, got *bool, want *bool) {
	t.Helper()
	switch {
	case want == nil && got != nil:
		t.Fatalf("%s: expected nil, got %v", field, *got)
	case want != nil && got == nil:
		t.Fatalf("%s: expected %v, got nil", field, *want)
	case want != nil && got != nil && *want != *got:
		t.Fatalf("%s: expected %v, got %v", field, *want, *got)
	}
}

func assertOptionalString(t *testing.T, field string, got *string, want *string) {
	t.Helper()
	switch {
	case want == nil && got != nil:
		t.Fatalf("%s: expected nil, got %q", field, *got)
	case want != nil && got == nil:
		t.Fatalf("%s: expected %q, got nil", field, *want)
	case want != nil && got != nil && *want != *got:
		t.Fatalf("%s: expected %q, got %q", field, *want, *got)
	}
}

func writeWorkspaceConfig(t *testing.T, workspaceRoot, content string) {
	t.Helper()

	configDir := filepath.Join(workspaceRoot, ".compozy")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(strings.TrimLeft(content, "\n")), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
}

func writeGlobalConfig(t *testing.T, homeDir, content string) {
	t.Helper()

	configDir := filepath.Join(homeDir, ".compozy")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("mkdir global config dir: %v", err)
	}
	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(strings.TrimLeft(content, "\n")), 0o600); err != nil {
		t.Fatalf("write global config: %v", err)
	}
}

func isolateWorkspaceConfigHome(t *testing.T) string {
	t.Helper()

	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	return homeDir
}

func stubWorkspaceUserHomeDir(t *testing.T, fn func() (string, error)) {
	t.Helper()

	original := osUserHomeDir
	osUserHomeDir = fn
	t.Cleanup(func() {
		osUserHomeDir = original
	})
}

func mustEvalSymlinksWorkspaceTest(t *testing.T, path string) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("eval symlinks for %s: %v", path, err)
	}
	return resolved
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for idx := range got {
		if got[idx] != want[idx] {
			return false
		}
	}
	return true
}
