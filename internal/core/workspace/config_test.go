package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
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

func TestLoadConfigReturnsZeroConfigWhenFileIsMissing(t *testing.T) {
	t.Parallel()

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
reasoning_effort = "high"
access_mode = "full"
timeout = "5m"
tail_lines = 0
add_dirs = []
auto_commit = true
max_retries = 0
retry_backoff_multiplier = 1.5

[start]
include_completed = false

[fix_reviews]
concurrent = 2
batch_size = 3
include_resolved = false

[fetch_reviews]
provider = "coderabbit"
`)

	cfg, _, err := LoadConfig(context.Background(), root)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	if cfg.Defaults.IDE == nil || *cfg.Defaults.IDE != "claude" {
		t.Fatalf("unexpected defaults.ide: %#v", cfg.Defaults.IDE)
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
	if cfg.Start.IncludeCompleted == nil || *cfg.Start.IncludeCompleted {
		t.Fatalf("unexpected start.include_completed: %#v", cfg.Start.IncludeCompleted)
	}
	if cfg.FixReviews.Concurrent == nil || *cfg.FixReviews.Concurrent != 2 {
		t.Fatalf("unexpected fix_reviews.concurrent: %#v", cfg.FixReviews.Concurrent)
	}
	if cfg.FetchReviews.Provider == nil || *cfg.FetchReviews.Provider != "coderabbit" {
		t.Fatalf("unexpected fetch_reviews.provider: %#v", cfg.FetchReviews.Provider)
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
