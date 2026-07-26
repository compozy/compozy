package soul

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	aghconfig "github.com/compozy/agh/internal/config"
)

func TestParseMarkdownOnlySoul(t *testing.T) {
	t.Run("Should resolve markdown-only soul with deterministic digest", func(t *testing.T) {
		t.Parallel()

		cfg := testSoulConfig()
		first, err := Parse(context.Background(), ParseRequest{
			SourcePath: "/tmp/work/.agh/agents/coder/SOUL.md",
			Content:    []byte("# Persona\r\nLead with clarity.\n"),
			Config:     cfg,
		})
		if err != nil {
			t.Fatalf("Parse(first) error = %v", err)
		}
		second, err := Parse(context.Background(), ParseRequest{
			SourcePath: "/tmp/work/.agh/agents/coder/SOUL.md",
			Content:    []byte("# Persona\nLead with clarity.\n\n"),
			Config:     cfg,
		})
		if err != nil {
			t.Fatalf("Parse(second) error = %v", err)
		}

		if !first.Present || !first.Active || !first.Valid {
			t.Fatalf(
				"Parse() flags = present:%v active:%v valid:%v, want all true",
				first.Present,
				first.Active,
				first.Valid,
			)
		}
		if first.Digest == "" || first.Digest != second.Digest {
			t.Fatalf("Digest determinism mismatch: first=%q second=%q", first.Digest, second.Digest)
		}
		if got, want := first.Profile.Body, "# Persona\nLead with clarity."; got != want {
			t.Fatalf("Profile.Body = %q, want %q", got, want)
		}
		if got, want := first.SourcePath, ".agh/agents/coder/SOUL.md"; got != want {
			t.Fatalf("SourcePath = %q, want %q", got, want)
		}
	})
}

func TestParseStrictFrontmatterSoul(t *testing.T) {
	t.Run("Should resolve allowlisted strict frontmatter and body", func(t *testing.T) {
		t.Parallel()

		resolved, err := Parse(context.Background(), ParseRequest{
			SourcePath: "agents/reviewer/SOUL.md",
			Content: []byte(strings.Join([]string{
				"---",
				"version: 1",
				"role: Reviewer",
				"tone:",
				"  - direct",
				"  - concise",
				"principles:",
				"  - protect correctness",
				"constraints:",
				"  - no hidden authority",
				"collaboration:",
				"  - ask only when blocked",
				"memory_policy:",
				"  - cite durable memory",
				"tags:",
				"  - qa",
				"---",
				"Review implementation behavior.",
			}, "\n")),
			Config: testSoulConfig(),
		})
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}

		if got, want := resolved.Profile.Version, "1"; got != want {
			t.Fatalf("Profile.Version = %q, want %q", got, want)
		}
		if got, want := resolved.Profile.Role, "Reviewer"; got != want {
			t.Fatalf("Profile.Role = %q, want %q", got, want)
		}
		assertStrings(t, resolved.Profile.Tone, []string{"direct", "concise"})
		assertStrings(t, resolved.Profile.Principles, []string{"protect correctness"})
		assertStrings(t, resolved.Profile.Constraints, []string{"no hidden authority"})
		assertStrings(t, resolved.Profile.Collaboration, []string{"ask only when blocked"})
		assertStrings(t, resolved.Profile.MemoryPolicy, []string{"cite durable memory"})
		assertStrings(t, resolved.Profile.Tags, []string{"qa"})
		if got, want := resolved.ReadModel.Body, "Review implementation behavior."; got != want {
			t.Fatalf("ReadModel.Body = %q, want %q", got, want)
		}
	})
}

func TestParseWorkspaceRelativeSourcePath(t *testing.T) {
	t.Run("Should resolve relative source paths under workspace root", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		resolved, err := Parse(context.Background(), ParseRequest{
			SourcePath:    ".agh/agents/coder/SOUL.md",
			WorkspaceRoot: workspaceRoot,
			Content:       []byte("Lead with clarity."),
			Config:        testSoulConfig(),
		})
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}
		if got, want := resolved.SourcePath, ".agh/agents/coder/SOUL.md"; got != want {
			t.Fatalf("SourcePath = %q, want %q", got, want)
		}
	})
}

func TestParseRejectsAuthorityClaims(t *testing.T) {
	tests := []struct {
		name    string
		content string
		code    string
		field   string
		section string
		line    int
	}{
		{
			name: "Should reject AGENT owned provider frontmatter",
			content: strings.Join([]string{
				"---",
				"role: Reviewer",
				"provider: claude",
				"---",
				"Body",
			}, "\n"),
			code:  "forbidden_field",
			field: "provider",
			line:  3,
		},
		{
			name: "Should reject task runtime section",
			content: strings.Join([]string{
				"---",
				"role: Reviewer",
				"---",
				"## Task Runs",
				"claim_token = agh_claim_secret_value",
			}, "\n"),
			code:    "reserved_section",
			section: "task_runs",
			line:    4,
		},
		{
			name: "Should reject unsupported frontmatter field",
			content: strings.Join([]string{
				"---",
				"role: Reviewer",
				"favorite_color: orange",
				"---",
				"Body",
			}, "\n"),
			code:  "unsupported_field",
			field: "favorite_color",
			line:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resolved, err := Parse(context.Background(), ParseRequest{
				SourcePath: "/home/user/project/.agh/agents/reviewer/SOUL.md",
				Content:    []byte(tt.content),
				Config:     testSoulConfig(),
			})
			if err == nil {
				t.Fatal("Parse() error = nil, want diagnostic error")
			}
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("Parse() error = %v, want ErrInvalid", err)
			}
			if resolved.Valid {
				t.Fatal("Resolved.Valid = true, want false")
			}
			if !resolved.Present || resolved.Active || resolved.ReadModel.Valid || resolved.ReadModel.Active {
				t.Fatalf(
					"Resolved invalid flags = present:%v active:%v readValid:%v readActive:%v, want present true and inactive invalid read model",
					resolved.Present,
					resolved.Active,
					resolved.ReadModel.Valid,
					resolved.ReadModel.Active,
				)
			}
			if len(resolved.Diagnostics) != 1 {
				t.Fatalf("Diagnostics len = %d, want 1 (%#v)", len(resolved.Diagnostics), resolved.Diagnostics)
			}
			diag := resolved.Diagnostics[0]
			if diag.Code != tt.code || diag.Field != tt.field || diag.Section != tt.section {
				t.Fatalf("Diagnostic = %#v, want code=%q field=%q section=%q", diag, tt.code, tt.field, tt.section)
			}
			if diag.Line != tt.line || diag.Column != 1 {
				t.Fatalf("Diagnostic location = %d:%d, want %d:1", diag.Line, diag.Column, tt.line)
			}
			if strings.Contains(diag.SourcePath, "/home/user") {
				t.Fatalf("Diagnostic SourcePath leaked absolute path: %q", diag.SourcePath)
			}
			if strings.Contains(diag.Message, "agh_claim_secret_value") {
				t.Fatalf("Diagnostic message leaked forbidden body content: %q", diag.Message)
			}
		})
	}
}

func TestParseRejectsForbiddenOwnerCategories(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field string
		owner string
	}{
		{name: "Should reject capability authority", field: "capabilities", owner: "capabilities"},
		{name: "Should reject claim token authority", field: "claim_token", owner: "task runtime"},
		{name: "Should reject session liveness authority", field: "session_liveness", owner: "runtime state"},
		{name: "Should reject network presence authority", field: "presence", owner: "AGH Network presence"},
		{name: "Should reject spawn overlay authority", field: "spawn", owner: "session spawn overlays"},
		{name: "Should reject config authority", field: "settings", owner: "config"},
		{name: "Should reject memory runtime authority", field: "memory_scope", owner: "memory runtime"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resolved, err := Parse(context.Background(), ParseRequest{
				SourcePath: "SOUL.md",
				Content: []byte(strings.Join([]string{
					"---",
					tt.field + ": forbidden",
					"---",
					"Body",
				}, "\n")),
				Config: testSoulConfig(),
			})
			if err == nil {
				t.Fatal("Parse() error = nil, want diagnostic error")
			}
			if len(resolved.Diagnostics) != 1 {
				t.Fatalf("Diagnostics len = %d, want 1 (%#v)", len(resolved.Diagnostics), resolved.Diagnostics)
			}
			diag := resolved.Diagnostics[0]
			if diag.Code != "forbidden_field" || diag.Field != tt.field {
				t.Fatalf("Diagnostic = %#v, want forbidden field %q", diag, tt.field)
			}
			if !strings.Contains(diag.Message, tt.owner) {
				t.Fatalf("Diagnostic message = %q, want owner %q", diag.Message, tt.owner)
			}
		})
	}
}

func TestParseRejectsMalformedFrontmatterAndInvalidTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		code    string
		field   string
	}{
		{
			name:    "Should reject unterminated frontmatter",
			content: "---\nrole: Reviewer",
			code:    "malformed_frontmatter",
		},
		{
			name: "Should reject invalid YAML frontmatter",
			content: strings.Join([]string{
				"---",
				"role: [broken",
				"---",
				"Body",
			}, "\n"),
			code: "malformed_frontmatter",
		},
		{
			name: "Should reject non string role",
			content: strings.Join([]string{
				"---",
				"role:",
				"  - reviewer",
				"---",
				"Body",
			}, "\n"),
			code:  "invalid_field_type",
			field: "role",
		},
		{
			name: "Should reject non scalar version",
			content: strings.Join([]string{
				"---",
				"version: true",
				"---",
				"Body",
			}, "\n"),
			code:  "invalid_field_type",
			field: "version",
		},
		{
			name: "Should reject non string list item",
			content: strings.Join([]string{
				"---",
				"tone:",
				"  - direct",
				"  - 42",
				"---",
				"Body",
			}, "\n"),
			code:  "invalid_field_type",
			field: "tone",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			resolved, err := Parse(context.Background(), ParseRequest{
				SourcePath: "SOUL.md",
				Content:    []byte(tt.content),
				Config:     testSoulConfig(),
			})
			if err == nil {
				t.Fatal("Parse() error = nil, want diagnostic error")
			}
			if len(resolved.Diagnostics) != 1 {
				t.Fatalf("Diagnostics len = %d, want 1 (%#v)", len(resolved.Diagnostics), resolved.Diagnostics)
			}
			diag := resolved.Diagnostics[0]
			if diag.Code != tt.code || diag.Field != tt.field {
				t.Fatalf("Diagnostic = %#v, want code=%q field=%q", diag, tt.code, tt.field)
			}
		})
	}
}

func TestParseRejectsOversizedInputs(t *testing.T) {
	tests := []struct {
		name    string
		content string
		code    string
	}{
		{
			name:    "Should reject oversized body",
			content: strings.Repeat("a", 257),
			code:    "oversized_body",
		},
		{
			name: "Should reject oversized frontmatter",
			content: strings.Join([]string{
				"---",
				"role: " + strings.Repeat("a", 257),
				"---",
				"Body",
			}, "\n"),
			code: "oversized_frontmatter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := aghconfig.SoulConfig{
				Enabled:                true,
				MaxBodyBytes:           256,
				ContextProjectionBytes: 256,
			}
			resolved, err := Parse(context.Background(), ParseRequest{
				SourcePath: "SOUL.md",
				Content:    []byte(tt.content),
				Config:     cfg,
			})
			if err == nil {
				t.Fatal("Parse() error = nil, want diagnostic error")
			}
			if len(resolved.Diagnostics) != 1 || resolved.Diagnostics[0].Code != tt.code {
				t.Fatalf("Diagnostics = %#v, want code %q", resolved.Diagnostics, tt.code)
			}
		})
	}
}

func TestResolveMissingAndDisabledSoul(t *testing.T) {
	tests := []struct {
		name       string
		config     aghconfig.SoulConfig
		writeSoul  bool
		wantActive bool
		wantDigest bool
	}{
		{
			name:       "Should return enabled empty result for missing optional file",
			config:     testSoulConfig(),
			wantActive: false,
			wantDigest: false,
		},
		{
			name: "Should resolve present file as inactive when soul config is disabled",
			config: aghconfig.SoulConfig{
				Enabled:                false,
				MaxBodyBytes:           32768,
				ContextProjectionBytes: 2048,
			},
			writeSoul:  true,
			wantActive: false,
			wantDigest: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			workspaceRoot := t.TempDir()
			agentDir := filepath.Join(workspaceRoot, ".agh", "agents", "coder")
			agentPath := filepath.Join(agentDir, "AGENT.md")
			writeTestFile(t, agentPath, "---\nname: coder\nprovider: codex\n---\nBase prompt")
			if tt.writeSoul {
				writeTestFile(t, filepath.Join(agentDir, FileName), "Lead with precision.")
			}

			resolved, err := Resolve(context.Background(), ResolveRequest{
				AgentPath:     agentPath,
				WorkspaceRoot: workspaceRoot,
				Config:        tt.config,
			})
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if resolved.Active != tt.wantActive {
				t.Fatalf("Resolve() Active = %v, want %v", resolved.Active, tt.wantActive)
			}
			if (resolved.Digest != "") != tt.wantDigest {
				t.Fatalf("Resolve() Digest = %q, want present=%v", resolved.Digest, tt.wantDigest)
			}
			if resolved.Present != tt.writeSoul {
				t.Fatalf("Resolve() Present = %v, want %v", resolved.Present, tt.writeSoul)
			}
		})
	}
}

func TestResolveClosedDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("Should reject empty agent path with sanitized diagnostic", func(t *testing.T) {
		t.Parallel()

		resolved, err := Resolve(context.Background(), ResolveRequest{
			Config: testSoulConfig(),
		})
		if err == nil {
			t.Fatal("Resolve() error = nil, want diagnostic error")
		}
		if len(resolved.Diagnostics) != 1 {
			t.Fatalf("Diagnostics len = %d, want 1 (%#v)", len(resolved.Diagnostics), resolved.Diagnostics)
		}
		diag := resolved.Diagnostics[0]
		if diag.Code != "invalid_source_path" || diag.Message != "SOUL.md source path is required" {
			t.Fatalf("Diagnostic = %#v, want invalid_source_path", diag)
		}
	})

	t.Run("Should reject unreadable SOUL directory as parser IO", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		agentDir := filepath.Join(workspaceRoot, ".agh", "agents", "coder")
		agentPath := filepath.Join(agentDir, "AGENT.md")
		writeTestFile(t, agentPath, "---\nname: coder\nprovider: codex\n---\nBase prompt")
		if err := os.Mkdir(filepath.Join(agentDir, FileName), 0o755); err != nil {
			t.Fatalf("Mkdir(SOUL.md) error = %v", err)
		}

		resolved, err := Resolve(context.Background(), ResolveRequest{
			AgentPath:     agentPath,
			WorkspaceRoot: workspaceRoot,
			Config:        testSoulConfig(),
		})
		if err == nil {
			t.Fatal("Resolve() error = nil, want diagnostic error")
		}
		if len(resolved.Diagnostics) != 1 || resolved.Diagnostics[0].Code != "parser_io" {
			t.Fatalf("Diagnostics = %#v, want parser_io", resolved.Diagnostics)
		}
	})
}

func TestContextCancellation(t *testing.T) {
	t.Parallel()

	t.Run("Should return context cancellation before parsing", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := Parse(ctx, ParseRequest{
			SourcePath: "SOUL.md",
			Content:    []byte("Body"),
			Config:     testSoulConfig(),
		})
		if err == nil {
			t.Fatal("Parse(canceled) error = nil, want non-nil")
		}
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Parse(canceled) error = %v, want context.Canceled", err)
		}
	})
}

func TestCompactProjection(t *testing.T) {
	t.Run("Should reject impossible compact projection budgets", func(t *testing.T) {
		t.Parallel()

		_, err := Parse(context.Background(), ParseRequest{
			SourcePath: "agents/coder/SOUL.md",
			Content:    []byte("Keep the projection bounded."),
			Config: aghconfig.SoulConfig{
				Enabled:                true,
				MaxBodyBytes:           4096,
				ContextProjectionBytes: 8,
			},
		})
		if err == nil {
			t.Fatal("Parse(tiny projection budget) error = nil, want validation error")
		}
		if !strings.Contains(err.Error(), "agents.soul.context_projection_bytes") {
			t.Fatalf("Parse(tiny projection budget) error = %v, want context projection validation", err)
		}
	})

	t.Run("Should truncate compact projection without exposing full body", func(t *testing.T) {
		t.Parallel()

		resolved, err := Parse(context.Background(), ParseRequest{
			SourcePath: "agents/coder/SOUL.md",
			Content: []byte(strings.Join([]string{
				"---",
				"role: " + strings.Repeat("reviewer ", 20),
				"tone:",
				"  - " + strings.Repeat("direct ", 20),
				"principles:",
				"  - " + strings.Repeat("correctness ", 20),
				"  - " + strings.Repeat("evidence ", 20),
				"---",
				"FULL BODY MUST STAY OUT OF COMPACT CONTEXT",
			}, "\n")),
			Config: aghconfig.SoulConfig{
				Enabled:                true,
				MaxBodyBytes:           4096,
				ContextProjectionBytes: 256,
			},
		})
		if err != nil {
			t.Fatalf("Parse() error = %v", err)
		}

		if !resolved.Compact.Truncated || !resolved.Profile.Truncated || !resolved.ReadModel.Truncated {
			t.Fatalf("Truncated flags = compact:%v profile:%v read:%v, want all true",
				resolved.Compact.Truncated,
				resolved.Profile.Truncated,
				resolved.ReadModel.Truncated,
			)
		}
		compactJSON, err := json.Marshal(resolved.Compact)
		if err != nil {
			t.Fatalf("json.Marshal(compact) error = %v", err)
		}
		if got, wantMax := int64(len(compactJSON)), resolved.Compact.MaxBytes; got > wantMax {
			t.Fatalf("Compact JSON size = %d, want <= %d: %s", got, wantMax, compactJSON)
		}
		if strings.Contains(string(compactJSON), "FULL BODY MUST STAY OUT") {
			t.Fatalf("Compact projection leaked body: %s", compactJSON)
		}
		if got, want := resolved.ReadModel.Body, "FULL BODY MUST STAY OUT OF COMPACT CONTEXT"; got != want {
			t.Fatalf("ReadModel.Body = %q, want %q", got, want)
		}
	})
}

func TestResolvePathSafety(t *testing.T) {
	t.Run("Should reject symlinked SOUL path outside workspace root", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		outsideRoot := t.TempDir()
		outsideSoul := filepath.Join(outsideRoot, FileName)
		writeTestFile(t, outsideSoul, "Escaped persona")

		agentDir := filepath.Join(workspaceRoot, ".agh", "agents", "coder")
		agentPath := filepath.Join(agentDir, "AGENT.md")
		writeTestFile(t, agentPath, "---\nname: coder\nprovider: codex\n---\nBase prompt")
		if err := os.Symlink(outsideSoul, filepath.Join(agentDir, FileName)); err != nil {
			t.Skipf("symlink unavailable: %v", err)
		}

		resolved, err := Resolve(context.Background(), ResolveRequest{
			AgentPath:     agentPath,
			WorkspaceRoot: workspaceRoot,
			Config:        testSoulConfig(),
		})
		if err == nil {
			t.Fatal("Resolve() error = nil, want diagnostic error")
		}
		if len(resolved.Diagnostics) != 1 || resolved.Diagnostics[0].Code != "path_escape" {
			t.Fatalf("Diagnostics = %#v, want path_escape", resolved.Diagnostics)
		}
	})
}

func TestDiagnosticErrorHelpers(t *testing.T) {
	t.Parallel()

	t.Run("Should expose invalid sentinel for empty diagnostic errors", func(t *testing.T) {
		t.Parallel()

		err := &DiagnosticError{}
		if got, want := err.Error(), ErrInvalid.Error(); got != want {
			t.Fatalf("Error() = %q, want %q", got, want)
		}
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("errors.Is(DiagnosticError, ErrInvalid) = false, want true")
		}
	})
}

func TestResolveAfterAgentLoad(t *testing.T) {
	t.Run("Should resolve beside AGENT without mutating runtime state", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		agentDir := filepath.Join(workspaceRoot, ".agh", "agents", "reviewer")
		agentPath := filepath.Join(agentDir, "AGENT.md")
		writeTestFile(t, agentPath, "---\nname: reviewer\nprovider: codex\n---\nReview code")
		writeTestFile(t, filepath.Join(agentDir, FileName), "---\nrole: Reviewer\n---\nStay precise.")

		agent, err := aghconfig.LoadAgentDefFile(agentPath)
		if err != nil {
			t.Fatalf("LoadAgentDefFile() error = %v", err)
		}
		if got, want := agent.Name, "reviewer"; got != want {
			t.Fatalf("Agent.Name = %q, want %q", got, want)
		}

		resolved, err := Resolve(context.Background(), ResolveRequest{
			AgentPath:     agentPath,
			WorkspaceRoot: workspaceRoot,
			Config:        testSoulConfig(),
		})
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}
		if got, want := resolved.Profile.Role, "Reviewer"; got != want {
			t.Fatalf("Profile.Role = %q, want %q", got, want)
		}

		for _, forbidden := range []string{"sessions", "task_runs", "network"} {
			if _, err := os.Stat(filepath.Join(workspaceRoot, forbidden)); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("Resolve() created %s or returned unexpected stat error: %v", forbidden, err)
			}
		}
	})
}

func testSoulConfig() aghconfig.SoulConfig {
	return aghconfig.SoulConfig{
		Enabled:                true,
		MaxBodyBytes:           32768,
		ContextProjectionBytes: 2048,
	}
}

func writeTestFile(t *testing.T, path string, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func assertStrings(t *testing.T, got []string, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("strings len = %d, want %d (%#v)", len(got), len(want), got)
	}
	for idx := range want {
		if got[idx] != want[idx] {
			t.Fatalf("strings[%d] = %q, want %q", idx, got[idx], want[idx])
		}
	}
}
