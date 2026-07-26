package skills

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	aghconfig "github.com/compozy/agh/internal/config"
	"github.com/compozy/agh/internal/filesnap"
	"github.com/compozy/agh/internal/frontmatter"
	hookspkg "github.com/compozy/agh/internal/hooks"
)

func TestParseSkillContentValidCases(t *testing.T) {
	t.Parallel()

	longBody := strings.Repeat("abc123", 9_000)
	tests := []struct {
		name           string
		content        string
		wantMeta       SkillMeta
		wantBody       string
		wantBodyLength int
	}{
		{
			name: "all fields",
			content: strings.Join([]string{
				"---",
				"name: code-review",
				"description: Review code carefully",
				"version: 1.2.3",
				"metadata:",
				"  agh:",
				"    category: quality",
				"---",
				"# Heading",
				"",
				"Review body",
			}, "\n"),
			wantMeta: SkillMeta{
				Name:        "code-review",
				Description: "Review code carefully",
				Version:     "1.2.3",
				Metadata: map[string]any{
					"agh": map[string]any{
						"category": "quality",
					},
				},
			},
			wantBody: "# Heading\n\nReview body",
		},
		{
			name: "required fields only",
			content: strings.Join([]string{
				"---",
				"name: debug",
				"description: Diagnose runtime failures",
				"---",
				"Debug this system",
			}, "\n"),
			wantMeta: SkillMeta{
				Name:        "debug",
				Description: "Diagnose runtime failures",
			},
			wantBody: "Debug this system",
		},
		{
			name: "empty body",
			content: strings.Join([]string{
				"---",
				"name: empty",
				"description: Body optional",
				"---",
			}, "\n"),
			wantMeta: SkillMeta{
				Name:        "empty",
				Description: "Body optional",
			},
			wantBody: "",
		},
		{
			name: "long body",
			content: strings.Join([]string{
				"---",
				"name: long-body",
				"description: Long content",
				"---",
				longBody,
			}, "\n"),
			wantMeta: SkillMeta{
				Name:        "long-body",
				Description: "Long content",
			},
			wantBodyLength: len(longBody),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotMeta, gotBody, err := parseSkillContent([]byte(tt.content))
			if err != nil {
				t.Fatalf("parseSkillContent() error = %v", err)
			}

			if !reflect.DeepEqual(gotMeta, tt.wantMeta) {
				t.Fatalf("parseSkillContent() meta mismatch\nwant: %#v\ngot:  %#v", tt.wantMeta, gotMeta)
			}

			switch {
			case tt.wantBodyLength > 0 && len(gotBody) != tt.wantBodyLength:
				t.Fatalf("parseSkillContent() body length = %d, want %d", len(gotBody), tt.wantBodyLength)
			case tt.wantBodyLength == 0 && gotBody != tt.wantBody:
				t.Fatalf("parseSkillContent() body = %q, want %q", gotBody, tt.wantBody)
			}
		})
	}
}

func TestParseSkillContentErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		wantErr error
	}{
		{
			name:    "delimiter only",
			content: "---",
			wantErr: frontmatter.ErrUnterminated,
		},
		{
			name:    "missing opening delimiter",
			content: "name: invalid" + "\n" + "description: missing delimiters",
			wantErr: frontmatter.ErrMissing,
		},
		{
			name: "unterminated frontmatter",
			content: strings.Join([]string{
				"---",
				"name: invalid",
				"description: missing close",
			}, "\n"),
			wantErr: frontmatter.ErrUnterminated,
		},
		{
			name: "malformed yaml",
			content: strings.Join([]string{
				"---",
				"name: [broken",
				"description: invalid yaml",
				"---",
			}, "\n"),
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, _, err := parseSkillContent([]byte(tt.content))
			if err == nil {
				t.Fatal("parseSkillContent() error = nil, want error")
			}

			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Fatalf("parseSkillContent() error = %v, want %v", err, tt.wantErr)
			}
			if tt.wantErr == nil && !strings.Contains(err.Error(), "decode YAML frontmatter") {
				t.Fatalf("parseSkillContent() error = %v, want YAML decode error", err)
			}
		})
	}
}

func TestParseSkillContentWarnsOnUnknownFields(t *testing.T) {
	original := slog.Default()
	var logs bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() {
		slog.SetDefault(original)
	})

	content := strings.Join([]string{
		"---",
		"name: warning-test",
		"description: unknown fields are allowed",
		"extra: true",
		"---",
		"body",
	}, "\n")

	meta, body, err := parseSkillContent([]byte(content))
	if err != nil {
		t.Fatalf("parseSkillContent() error = %v", err)
	}
	if meta.Name != "warning-test" {
		t.Fatalf("parseSkillContent() meta.Name = %q, want %q", meta.Name, "warning-test")
	}
	if body != "body" {
		t.Fatalf("parseSkillContent() body = %q, want %q", body, "body")
	}
	if !strings.Contains(logs.String(), "unknown frontmatter field") || !strings.Contains(logs.String(), "extra") {
		t.Fatalf("expected unknown field warning in logs, got %q", logs.String())
	}
}

func TestParseSkillFile(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := writeSkillFile(t, root, filepath.Join("quality", skillFileName), strings.Join([]string{
		"---",
		"name: quality",
		"description: Validate output",
		"version: 2.0.0",
		"---",
		"Check every requirement.",
	}, "\n"))

	skill, err := ParseSkillFile(path)
	if err != nil {
		t.Fatalf("ParseSkillFile() error = %v", err)
	}

	if skill.Meta.Name != "quality" {
		t.Fatalf("ParseSkillFile() meta.Name = %q, want %q", skill.Meta.Name, "quality")
	}
	if skill.FilePath != path {
		t.Fatalf("ParseSkillFile() FilePath = %q, want %q", skill.FilePath, path)
	}
	if skill.Dir != filepath.Dir(path) {
		t.Fatalf("ParseSkillFile() Dir = %q, want %q", skill.Dir, filepath.Dir(path))
	}
	if !skill.Enabled {
		t.Fatal("ParseSkillFile() Enabled = false, want true")
	}
}

func TestParseSkillFileRejectsSymlinkEscape(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	outside := t.TempDir()
	outsideSkill := writeSkillFile(t, outside, skillFileName, strings.Join([]string{
		"---",
		"name: escaped",
		"description: Outside skill",
		"---",
		"secret",
	}, "\n"))
	skillDir := filepath.Join(root, "skill")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(skillDir) error = %v", err)
	}
	linkPath := filepath.Join(skillDir, skillFileName)
	if err := os.Symlink(outsideSkill, linkPath); err != nil {
		t.Skipf("os.Symlink(SKILL.md) unavailable: %v", err)
	}

	_, err := ParseSkillFile(linkPath)
	if err == nil {
		t.Fatal("ParseSkillFile() error = nil, want symlink escape rejection")
	}
	if !strings.Contains(err.Error(), "escapes skill root") {
		t.Fatalf("ParseSkillFile() error = %v, want escape rejection", err)
	}
}

func TestReadSkillContent(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := writeSkillFile(t, root, filepath.Join("quality", skillFileName), strings.Join([]string{
		"---",
		"name: quality",
		"description: Validate output",
		"---",
		"Check every requirement.",
	}, "\n"))

	content, err := ReadSkillContent(path)
	if err != nil {
		t.Fatalf("ReadSkillContent() error = %v", err)
	}
	if content != "Check every requirement." {
		t.Fatalf("ReadSkillContent() = %q, want %q", content, "Check every requirement.")
	}
}

func TestParseSkillFileMissingName(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := writeSkillFile(t, root, filepath.Join("missing-name", skillFileName), strings.Join([]string{
		"---",
		"description: No name present",
		"---",
		"body",
	}, "\n"))

	_, err := ParseSkillFile(path)
	if err == nil {
		t.Fatal("ParseSkillFile() error = nil, want error")
	}
	if !strings.Contains(err.Error(), errSkillNameRequired.Error()) {
		t.Fatalf("ParseSkillFile() error = %v, want containing %q", err, errSkillNameRequired)
	}
}

func TestParseSkillFileWarnsOnMissingDescription(t *testing.T) {
	original := slog.Default()
	var logs bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() {
		slog.SetDefault(original)
	})

	root := t.TempDir()
	path := writeSkillFile(t, root, filepath.Join("no-description", skillFileName), strings.Join([]string{
		"---",
		"name: no-description",
		"---",
		"body",
	}, "\n"))

	skill, err := ParseSkillFile(path)
	if err != nil {
		t.Fatalf("ParseSkillFile() error = %v", err)
	}
	if skill.Meta.Name != "no-description" {
		t.Fatalf("ParseSkillFile() meta.Name = %q, want %q", skill.Meta.Name, "no-description")
	}
	if !strings.Contains(logs.String(), "parsed skill without description") {
		t.Fatalf("expected missing description warning in logs, got %q", logs.String())
	}
}

func TestParseSkillFileMergesMCPSidecar(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	skillPath := writeSkillFile(t, root, filepath.Join("sidecar", skillFileName), strings.Join([]string{
		"---",
		"name: sidecar",
		"description: Sidecar MCP merge",
		"metadata:",
		"  agh:",
		"    mcp_servers:",
		"      - name: inline-only",
		"        command: inline-only-command",
		"      - name: shared",
		"        command: inline-shared",
		"        args: [\"--inline\"]",
		"---",
		"body",
	}, "\n"))
	writeSkillMCPSidecar(t, filepath.Dir(skillPath), `{
  "mcpServers": {
    "shared": {
      "command": "sidecar-shared"
    },
    "sidecar-only": {
      "command": "sidecar-only-command"
    }
  }
}`)

	skill, err := ParseSkillFile(skillPath)
	if err != nil {
		t.Fatalf("ParseSkillFile() error = %v", err)
	}

	if got, want := len(skill.MCPServers), 3; got != want {
		t.Fatalf("len(skill.MCPServers) = %d, want %d (%#v)", got, want, skill.MCPServers)
	}
	if got, want := skill.MCPServers[1].Command, "sidecar-shared"; got != want {
		t.Fatalf("skill.MCPServers[1].Command = %q, want %q", got, want)
	}
	if got := len(skill.MCPServers[1].Args); got != 0 {
		t.Fatalf("skill.MCPServers[1].Args = %#v, want sidecar whole-object replacement", skill.MCPServers[1].Args)
	}
	if got, want := skill.MCPServers[2].Name, "sidecar-only"; got != want {
		t.Fatalf("skill.MCPServers[2].Name = %q, want %q", got, want)
	}
}

func TestParseSkillFileParsesAGHMetadataFixtures(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		fixture  string
		wantMCP  []MCPServerDecl
		wantHook []hookspkg.HookDecl
	}{
		{
			name:    "mcp servers only",
			fixture: "mcp-only",
			wantMCP: []MCPServerDecl{{
				Name:    "filesystem",
				Command: "npx",
				Args:    []string{"-y", "@modelcontextprotocol/server-filesystem"},
				Env: map[string]string{
					"ROOT": "${WORKSPACE_ROOT}",
					"MODE": "read-only",
				},
			}},
		},
		{
			name:    "hooks only",
			fixture: "hooks-only",
			wantHook: []hookspkg.HookDecl{{
				Name:        "hooks-only",
				Event:       hookspkg.HookSessionPostCreate,
				Source:      hookspkg.HookSourceSkill,
				Mode:        hookspkg.HookModeAsync,
				Priority:    0,
				Timeout:     5 * time.Second,
				Command:     "/bin/sh",
				Args:        []string{"-c", "echo ready"},
				Env:         map[string]string{"HOOK_ENV": "enabled"},
				SkillSource: hookspkg.HookSkillSourceBundled,
			}},
		},
		{
			name:    "mcp servers and hooks",
			fixture: "combined",
			wantMCP: []MCPServerDecl{{
				Name:    "git",
				Command: "uvx",
				Args:    []string{"mcp-server-git"},
				Env: map[string]string{
					"REPO_ROOT": "${REPO_ROOT}",
				},
			}},
			wantHook: []hookspkg.HookDecl{{
				Name:        "combined",
				Event:       hookspkg.HookSessionPostStop,
				Source:      hookspkg.HookSourceSkill,
				Mode:        hookspkg.HookModeAsync,
				Priority:    0,
				Timeout:     30 * time.Second,
				Command:     "/usr/bin/env",
				Args:        []string{"bash", "-lc", "echo cleanup"},
				Env:         map[string]string{"PHASE": "stop"},
				SkillSource: hookspkg.HookSkillSourceBundled,
			}},
		},
		{
			name:    "without agh metadata",
			fixture: "no-agh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			skill, err := ParseSkillFile(loaderFixturePath(tt.fixture))
			if err != nil {
				t.Fatalf("ParseSkillFile(%q) error = %v", tt.fixture, err)
			}

			if !reflect.DeepEqual(skill.MCPServers, tt.wantMCP) {
				t.Fatalf(
					"ParseSkillFile(%q) MCPServers mismatch\nwant: %#v\ngot:  %#v",
					tt.fixture,
					tt.wantMCP,
					skill.MCPServers,
				)
			}
			if !reflect.DeepEqual(skill.Hooks, tt.wantHook) {
				t.Fatalf(
					"ParseSkillFile(%q) Hooks mismatch\nwant: %#v\ngot:  %#v",
					tt.fixture,
					tt.wantHook,
					skill.Hooks,
				)
			}
		})
	}
}

func TestParseSkillFileParsesActivationGatesStrictly(t *testing.T) {
	t.Parallel()

	t.Run("Should parse every supported activation gate", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		skillPath := writeSkillFile(t, root, filepath.Join("gated", skillFileName), strings.Join([]string{
			"---",
			"name: gated",
			"description: Gated skill",
			"metadata:",
			"  agh:",
			"    when:",
			"      platforms: [darwin, linux]",
			"      environments: [container]",
			"      requires_tools: [agh__skill_view]",
			"      requires_capabilities: [review.code]",
			"---",
			"body",
		}, "\n"))

		skill, err := ParseSkillFile(skillPath)
		if err != nil {
			t.Fatalf("ParseSkillFile() error = %v", err)
		}
		want := ActivationGates{
			Platforms:            []string{"darwin", "linux"},
			Environments:         []string{"container"},
			RequiresTools:        []string{"agh__skill_view"},
			RequiresCapabilities: []string{"review.code"},
		}
		if !reflect.DeepEqual(skill.ActivationGates, want) {
			t.Fatalf("ParseSkillFile() ActivationGates = %#v, want %#v", skill.ActivationGates, want)
		}
		if !skill.Activation.Active || len(skill.Activation.Reasons) != 0 {
			t.Fatalf("ParseSkillFile() Activation = %#v, want unevaluated active state", skill.Activation)
		}
	})

	t.Run("Should reject unknown when keys", func(t *testing.T) {
		t.Parallel()

		root := t.TempDir()
		skillPath := writeSkillFile(t, root, filepath.Join("unknown-gate", skillFileName), strings.Join([]string{
			"---",
			"name: unknown-gate",
			"description: Invalid gated skill",
			"metadata:",
			"  agh:",
			"    when:",
			"      platforms: [linux]",
			"      requires_toolsets: [terminal]",
			"---",
			"body",
		}, "\n"))

		_, err := ParseSkillFile(skillPath)
		if err == nil {
			t.Fatal("ParseSkillFile() error = nil, want unknown when key failure")
		}
		if !strings.Contains(err.Error(), `unknown key "requires_toolsets"`) {
			t.Fatalf("ParseSkillFile() error = %v, want unknown key detail", err)
		}
	})
}

func TestParseBundledSkillParsesAGHMetadata(t *testing.T) {
	t.Parallel()

	fsys := os.DirFS(filepath.Join("testdata", "loader"))
	skill, err := parseBundledSkill(fsys, path.Join("combined", skillFileName))
	if err != nil {
		t.Fatalf("parseBundledSkill() error = %v", err)
	}

	if skill.Source != SourceBundled {
		t.Fatalf("parseBundledSkill() Source = %v, want %v", skill.Source, SourceBundled)
	}
	if len(skill.MCPServers) != 1 || skill.MCPServers[0].Name != "git" {
		t.Fatalf("parseBundledSkill() MCPServers = %#v, want populated git server", skill.MCPServers)
	}
	if len(skill.Hooks) != 1 || skill.Hooks[0].Event != hookspkg.HookSessionPostStop {
		t.Fatalf("parseBundledSkill() Hooks = %#v, want populated stop hook", skill.Hooks)
	}

	content, err := readBundledSkillContent(fsys, path.Join("combined", skillFileName))
	if err != nil {
		t.Fatalf("readBundledSkillContent() error = %v", err)
	}
	if strings.TrimSpace(content) == "" {
		t.Fatal("readBundledSkillContent() returned empty body")
	}
}

func TestParseBundledSkillParsesMCPSidecar(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeSkillFile(
		t,
		root,
		filepath.Join("bundled-sidecar", skillFileName),
		skillWithDescription("bundled-sidecar", "Bundled sidecar"),
	)
	writeSkillMCPSidecar(t, filepath.Join(root, "bundled-sidecar"), `{
  "mcp_servers": {
    "bundled-json": {
      "command": "bundled-json-command"
    }
  }
}`)

	fsys := os.DirFS(root)
	skill, err := parseBundledSkill(fsys, path.Join("bundled-sidecar", skillFileName))
	if err != nil {
		t.Fatalf("parseBundledSkill() error = %v", err)
	}

	if got, want := len(skill.MCPServers), 1; got != want {
		t.Fatalf("len(skill.MCPServers) = %d, want %d (%#v)", got, want, skill.MCPServers)
	}
	if got, want := skill.MCPServers[0].Command, "bundled-json-command"; got != want {
		t.Fatalf("skill.MCPServers[0].Command = %q, want %q", got, want)
	}
}

func TestParseSkillFileWarnsOnMalformedAGHMetadata(t *testing.T) {
	original := slog.Default()
	var logs bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() {
		slog.SetDefault(original)
	})

	skill, err := ParseSkillFile(loaderFixturePath("malformed-agh"))
	if err != nil {
		t.Fatalf("ParseSkillFile() error = %v", err)
	}

	if skill.MCPServers != nil {
		t.Fatalf("ParseSkillFile() MCPServers = %#v, want nil", skill.MCPServers)
	}
	if skill.Hooks != nil {
		t.Fatalf("ParseSkillFile() Hooks = %#v, want nil", skill.Hooks)
	}
	if !strings.Contains(logs.String(), "malformed metadata.agh block") {
		t.Fatalf("expected malformed metadata warning in logs, got %q", logs.String())
	}
}

func TestParseSkillFileRejectsInvalidMCPServerEntriesWithWarnings(t *testing.T) {
	original := slog.Default()
	var logs bytes.Buffer
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, nil)))
	t.Cleanup(func() {
		slog.SetDefault(original)
	})

	skill, err := ParseSkillFile(loaderFixturePath("invalid-mcp"))
	if err != nil {
		t.Fatalf("ParseSkillFile() error = %v", err)
	}

	want := []MCPServerDecl{{
		Name:    "valid-server",
		Command: "node",
		Args:    []string{"server.js"},
	}}
	if !reflect.DeepEqual(skill.MCPServers, want) {
		t.Fatalf("ParseSkillFile() MCPServers mismatch\nwant: %#v\ngot:  %#v", want, skill.MCPServers)
	}
	if !strings.Contains(logs.String(), "reason=\"missing name\"") {
		t.Fatalf("expected missing name warning in logs, got %q", logs.String())
	}
	if !strings.Contains(logs.String(), "reason=\"missing command\"") {
		t.Fatalf("expected missing command warning in logs, got %q", logs.String())
	}
}

func TestParseSkillFileRejectsUnknownHookEvents(t *testing.T) {
	skill, err := ParseSkillFile(loaderFixturePath("invalid-hook"))
	if err == nil {
		t.Fatal("ParseSkillFile() error = nil, want unknown hook event failure")
	}
	if skill != nil {
		t.Fatalf("ParseSkillFile() skill = %#v, want nil on invalid hook event", skill)
	}
	if !strings.Contains(err.Error(), `unknown hook event "foo.bar"`) {
		t.Fatalf("ParseSkillFile() error = %v, want unknown event detail", err)
	}
}

func TestParseSkillFileRejectsHooksMissingCommand(t *testing.T) {
	t.Parallel()

	_, err := ParseSkillFile(loaderFixturePath("invalid-hook-command"))
	if err == nil {
		t.Fatal("ParseSkillFile() error = nil, want missing hook command failure")
	}
	if !strings.Contains(err.Error(), `invalid metadata.agh.hooks entry for "invalid-hook-command"`) {
		t.Fatalf("ParseSkillFile() error = %v, want skill identifier context", err)
	}
	if !strings.Contains(err.Error(), "command is required") {
		t.Fatalf("ParseSkillFile() error = %v, want missing command context", err)
	}
}

func TestParseSkillFileRejectsLegacyHookEventsWithReplacement(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := writeSkillFile(t, root, filepath.Join("legacy-hook", skillFileName), strings.Join([]string{
		"---",
		"name: legacy-hook",
		"description: Legacy hook names are rejected",
		"metadata:",
		"  agh:",
		"    hooks:",
		"      - event: on_session_created",
		"        command: /bin/echo",
		"---",
		"body",
	}, "\n"))

	_, err := ParseSkillFile(path)
	if err == nil {
		t.Fatal("ParseSkillFile() error = nil, want legacy hook event failure")
	}
	if !strings.Contains(err.Error(), `hook event "on_session_created" was removed; use "session.post_create"`) {
		t.Fatalf("ParseSkillFile() error = %v, want replacement guidance", err)
	}
}

func TestParseSkillFileParsesHookOptionalFields(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := writeSkillFile(t, root, filepath.Join("hook-options", skillFileName), strings.Join([]string{
		"---",
		"name: hook-options",
		"description: Hook with optional fields",
		"metadata:",
		"  agh:",
		"    hooks:",
		"      - event: session.post_create",
		"        command: /bin/echo",
		"        mode: sync",
		"        priority: 7",
		"        matcher:",
		"          agent_name: codex",
		"          workspace_id: ws-1",
		"---",
		"body",
	}, "\n"))

	skill, err := ParseSkillFile(path)
	if err != nil {
		t.Fatalf("ParseSkillFile() error = %v", err)
	}

	want := hookspkg.HookDecl{
		Name:        "hook-options",
		Event:       hookspkg.HookSessionPostCreate,
		Source:      hookspkg.HookSourceSkill,
		Mode:        hookspkg.HookModeSync,
		Priority:    7,
		PrioritySet: true,
		Command:     "/bin/echo",
		Matcher: hookspkg.HookMatcher{
			AgentName:   "codex",
			WorkspaceID: "ws-1",
		},
		SkillSource: hookspkg.HookSkillSourceBundled,
	}
	if got := skill.Hooks; !reflect.DeepEqual(got, []hookspkg.HookDecl{want}) {
		t.Fatalf("ParseSkillFile() Hooks mismatch\nwant: %#v\ngot:  %#v", []hookspkg.HookDecl{want}, got)
	}
}

func TestParseSkillFileDefaultsMinimalHookFields(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := writeSkillFile(t, root, filepath.Join("hook-defaults", skillFileName), strings.Join([]string{
		"---",
		"name: hook-defaults",
		"description: Minimal hook declaration",
		"metadata:",
		"  agh:",
		"    hooks:",
		"      - event: session.post_create",
		"        command: /bin/echo",
		"---",
		"body",
	}, "\n"))

	skill, err := ParseSkillFile(path)
	if err != nil {
		t.Fatalf("ParseSkillFile() error = %v", err)
	}
	if len(skill.Hooks) != 1 {
		t.Fatalf("len(skill.Hooks) = %d, want 1", len(skill.Hooks))
	}
	hook := skill.Hooks[0]
	if hook.Mode != hookspkg.HookModeAsync {
		t.Fatalf("hook.Mode = %q, want %q", hook.Mode, hookspkg.HookModeAsync)
	}
	if hook.Priority != 0 {
		t.Fatalf("hook.Priority = %d, want 0", hook.Priority)
	}
	if hook.PrioritySet {
		t.Fatal("hook.PrioritySet = true, want false for default priority")
	}
	if hook.Source != hookspkg.HookSourceSkill {
		t.Fatalf("hook.Source = %q, want skill source", hook.Source)
	}
	if hook.Name != "hook-defaults" {
		t.Fatalf("hook.Name = %q, want %q", hook.Name, "hook-defaults")
	}
}

func TestSkillHooksFieldUsesInternalHooksDeclarations(t *testing.T) {
	t.Parallel()

	got := reflect.TypeFor[[]hookspkg.HookDecl]()
	want := reflect.TypeFor[[]hookspkg.HookDecl]()
	if got != want {
		t.Fatalf("reflect.TypeOf(Skill{}.Hooks) = %v, want %v", got, want)
	}
}

func TestScanDirectoryHonorsDepthAndSkips(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	expected := []string{
		writeSkillFile(t, root, filepath.Join("depth1", skillFileName), defaultSkillContent("depth1")),
		writeSkillFile(t, root, filepath.Join("a", "depth2", skillFileName), defaultSkillContent("depth2")),
		writeSkillFile(t, root, filepath.Join("a", "b", "depth3", skillFileName), defaultSkillContent("depth3")),
		writeSkillFile(t, root, filepath.Join("a", "b", "c", "depth4", skillFileName), defaultSkillContent("depth4")),
		writeSkillFile(
			t,
			root,
			filepath.Join(".agh", "agents", "shared", "skills", skillFileName),
			defaultSkillContent("agent-local"),
		),
		writeSkillFile(t, root, filepath.Join(".agh", "workspace", skillFileName), defaultSkillContent("workspace")),
	}
	writeSkillFile(t, root, filepath.Join("a", "b", "c", "d", "too-deep", skillFileName), defaultSkillContent("depth5"))
	writeSkillFile(t, root, filepath.Join(".git", "ignored", skillFileName), defaultSkillContent("git"))
	writeSkillFile(t, root, filepath.Join("node_modules", "pkg", skillFileName), defaultSkillContent("node"))
	writeSkillFile(t, root, filepath.Join(".hidden", "ignored", skillFileName), defaultSkillContent("hidden"))

	got, err := scanDirectory(root)
	if err != nil {
		t.Fatalf("scanDirectory() error = %v", err)
	}

	slices.Sort(expected)
	if !reflect.DeepEqual(got, expected) {
		t.Fatalf("scanDirectory() mismatch\nwant: %#v\ngot:  %#v", expected, got)
	}
}

func TestScanDirectoryCandidateLimit(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for i := range maxScanCandidates + 5 {
		writeSkillFile(
			t,
			root,
			filepath.Join(fmt.Sprintf("skill-%03d", i), skillFileName),
			defaultSkillContent(fmt.Sprintf("skill-%03d", i)),
		)
	}

	got, err := scanDirectory(root)
	if err != nil {
		t.Fatalf("scanDirectory() error = %v", err)
	}
	if len(got) != maxScanCandidates {
		t.Fatalf("scanDirectory() len = %d, want %d", len(got), maxScanCandidates)
	}
}

func TestScanDirectoryMissingRoot(t *testing.T) {
	t.Parallel()

	got, err := scanDirectory(filepath.Join(t.TempDir(), "missing"))
	if err != nil {
		t.Fatalf("scanDirectory() error = %v, want nil", err)
	}
	if len(got) != 0 {
		t.Fatalf("scanDirectory() len = %d, want 0", len(got))
	}
}

func TestScanDirectoryRejectsInvalidRoots(t *testing.T) {
	t.Parallel()

	if _, err := scanDirectory("   "); err == nil {
		t.Fatal("scanDirectory() error = nil, want error for blank root")
	}

	fileRoot := filepath.Join(t.TempDir(), "SKILL.md")
	if err := os.WriteFile(fileRoot, []byte(defaultSkillContent("not-a-dir")), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", fileRoot, err)
	}

	if _, err := scanDirectory(fileRoot); err == nil {
		t.Fatal("scanDirectory() error = nil, want error for non-directory root")
	}
}

func TestSnapshotFile(t *testing.T) {
	t.Parallel()

	path := writeSkillFile(t, t.TempDir(), filepath.Join("skill", skillFileName), defaultSkillContent("snapshot"))

	snapshot, err := filesnap.FromPath(path)
	if err != nil {
		t.Fatalf("filesnap.FromPath() error = %v", err)
	}
	if snapshot.Size <= 0 {
		t.Fatalf("filesnap.FromPath() size = %d, want > 0", snapshot.Size)
	}
	if snapshot.ModTime.IsZero() {
		t.Fatal("filesnap.FromPath() mod time = zero, want populated")
	}

	if _, err := filesnap.FromPath(filepath.Join(t.TempDir(), "missing", skillFileName)); err == nil {
		t.Fatal("filesnap.FromPath() error = nil, want error for missing path")
	}
}

func writeSkillFile(t *testing.T, root, relPath, content string) string {
	t.Helper()

	path := filepath.Join(root, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	writeSkillFileAtomically(t, path, content)

	return path
}

func writeSkillFileAtomically(t *testing.T, path, content string) {
	t.Helper()

	dir := filepath.Dir(path)
	tempFile, err := os.CreateTemp(dir, ".skill-*")
	if err != nil {
		t.Fatalf("CreateTemp(%q) error = %v", dir, err)
	}

	tempPath := tempFile.Name()
	cleanup := func() {
		_ = os.Remove(tempPath)
	}
	defer cleanup()

	if _, err := tempFile.WriteString(content); err != nil {
		_ = tempFile.Close()
		t.Fatalf("WriteString(%q) error = %v", tempPath, err)
	}
	if err := tempFile.Chmod(0o644); err != nil {
		_ = tempFile.Close()
		t.Fatalf("Chmod(%q) error = %v", tempPath, err)
	}
	if err := tempFile.Close(); err != nil {
		t.Fatalf("Close(%q) error = %v", tempPath, err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		t.Fatalf("Rename(%q, %q) error = %v", tempPath, path, err)
	}
}

func defaultSkillContent(name string) string {
	return strings.Join([]string{
		"---",
		"name: " + name,
		"description: test skill",
		"---",
		"body",
	}, "\n")
}

func loaderFixturePath(name string) string {
	return filepath.Join("testdata", "loader", name, skillFileName)
}

func writeSkillMCPSidecar(t *testing.T, skillDir, content string) string {
	t.Helper()

	path := filepath.Join(skillDir, aghconfig.MCPJSONName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), fs.FileMode(0o644)); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}

	return path
}
