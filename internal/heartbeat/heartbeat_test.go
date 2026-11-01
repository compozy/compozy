package heartbeat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/compozy/compozy/internal/diagnostics"
)

func TestParseHeartbeatPolicy(t *testing.T) {
	t.Parallel()

	t.Run("Should resolve a valid policy deterministically", func(t *testing.T) {
		t.Parallel()

		workspaceRoot, sourcePath := heartbeatWorkspace(t)
		content := []byte(`---
version: 1
enabled: true
summary: "Reorient, inspect assignments, and claim only through Compozy task APIs."
preferences:
  min_interval: "30m"
  active_hours:
    - timezone: "America/Sao_Paulo"
      start: "08:00"
      end: "20:00"
  quiet_windows:
    - timezone: "America/Sao_Paulo"
      start: "22:00"
      end: "08:00"
context:
  include:
    - self
    - session_health
    - task
    - inbox_summary
---
# Wake Checklist

Inspect context first, then use compozy task next before doing task work.
`)

		first, err := Parse(t.Context(), ParseRequest{
			SourcePath:    sourcePath,
			WorkspaceRoot: workspaceRoot,
			Content:       content,
			Config:        compozyconfig.DefaultHeartbeatConfig(),
		})
		if err != nil {
			t.Fatalf("Parse(first) error = %v", err)
		}
		second, err := Parse(t.Context(), ParseRequest{
			SourcePath:    sourcePath,
			WorkspaceRoot: workspaceRoot,
			Content:       content,
			Config:        compozyconfig.DefaultHeartbeatConfig(),
		})
		if err != nil {
			t.Fatalf("Parse(second) error = %v", err)
		}

		if !first.Valid || !first.Present || !first.Active {
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
		if first.ConfigDigest == "" || first.ConfigDigest != second.ConfigDigest {
			t.Fatalf("ConfigDigest determinism mismatch: first=%q second=%q", first.ConfigDigest, second.ConfigDigest)
		}
		if got, want := first.SourcePath, ".compozy/agents/coder/HEARTBEAT.md"; got != want {
			t.Fatalf("SourcePath = %q, want %q", got, want)
		}
		if got, want := first.Preferences.MinInterval, 30*time.Minute; got != want {
			t.Fatalf("Preferences.MinInterval = %s, want %s", got, want)
		}
		if got, want := first.Prompt.Summary, first.Summary; got != want {
			t.Fatalf("Prompt.Summary = %q, want %q", got, want)
		}
		if got, want := first.Status.ConfigProvenance.Digest, first.ConfigDigest; got != want {
			t.Fatalf("Status.ConfigProvenance.Digest = %q, want %q", got, want)
		}
		wantContext := []string{"self", "session_health", "task", "inbox_summary"}
		if !slices.Equal(first.Preferences.Context.Include, wantContext) {
			t.Fatalf("Context include = %#v, want normalized include list", first.Preferences.Context.Include)
		}

		allowedAtNoon := mustTime(t, "America/Sao_Paulo", 2026, time.January, 2, 12, 0)
		allowed, err := first.Preferences.AllowsAt(allowedAtNoon)
		if err != nil {
			t.Fatalf("AllowsAt(noon) error = %v", err)
		}
		if !allowed {
			t.Fatal("AllowsAt(noon) = false, want true")
		}
		quietAtNight := mustTime(t, "America/Sao_Paulo", 2026, time.January, 2, 23, 0)
		allowed, err = first.Preferences.AllowsAt(quietAtNight)
		if err != nil {
			t.Fatalf("AllowsAt(night) error = %v", err)
		}
		if allowed {
			t.Fatal("AllowsAt(night) = true, want false for quiet window")
		}
	})

	t.Run("Should resolve body only policy with default preferences", func(t *testing.T) {
		t.Parallel()

		workspaceRoot, sourcePath := heartbeatWorkspace(t)
		resolved, err := Parse(t.Context(), ParseRequest{
			SourcePath:    sourcePath,
			WorkspaceRoot: workspaceRoot,
			Content:       []byte("Inspect context and then use official Compozy task APIs.\n\n"),
			Config:        compozyconfig.DefaultHeartbeatConfig(),
		})
		if err != nil {
			t.Fatalf("Parse(body only) error = %v", err)
		}
		if !resolved.Valid || !resolved.Active || resolved.Frontmatter.Version != 1 {
			t.Fatalf(
				"Parse(body only) = valid:%v active:%v version:%d, want valid active v1",
				resolved.Valid,
				resolved.Active,
				resolved.Frontmatter.Version,
			)
		}
		if got, want := resolved.Preferences.MinInterval, compozyconfig.DefaultHeartbeatConfig().DefaultInterval; got != want {
			t.Fatalf("Preferences.MinInterval = %s, want default %s", got, want)
		}
	})
}

func TestHeartbeatPreferences(t *testing.T) {
	t.Parallel()

	t.Run("Should clamp min interval preference to config lower bound", func(t *testing.T) {
		t.Parallel()

		workspaceRoot, sourcePath := heartbeatWorkspace(t)
		resolved, err := Parse(t.Context(), ParseRequest{
			SourcePath:    sourcePath,
			WorkspaceRoot: workspaceRoot,
			Content: []byte(`---
preferences:
  min_interval: "1m"
---
Wake gently.
`),
			Config: compozyconfig.DefaultHeartbeatConfig(),
		})
		if err != nil {
			t.Fatalf("Parse(clamped) error = %v", err)
		}
		if got, want := resolved.Preferences.MinInterval, compozyconfig.DefaultHeartbeatConfig().MinInterval; got != want {
			t.Fatalf("Preferences.MinInterval = %s, want clamped %s", got, want)
		}
		assertDiagnostic(t, resolved.Diagnostics, "heartbeat_preference_clamped")
	})

	t.Run("Should ignore active hours when config disables authored preferences", func(t *testing.T) {
		t.Parallel()

		workspaceRoot, sourcePath := heartbeatWorkspace(t)
		cfg := compozyconfig.DefaultHeartbeatConfig()
		cfg.AllowActiveHoursPreferences = false
		resolved, err := Parse(t.Context(), ParseRequest{
			SourcePath:    sourcePath,
			WorkspaceRoot: workspaceRoot,
			Content: []byte(`---
preferences:
  active_hours:
    - timezone: "America/Sao_Paulo"
      start: "08:00"
      end: "20:00"
  quiet_windows:
    - timezone: "America/Sao_Paulo"
      start: "22:00"
      end: "08:00"
---
Wake gently.
`),
			Config: cfg,
		})
		if err != nil {
			t.Fatalf("Parse(ignored active hours) error = %v", err)
		}
		if len(resolved.Preferences.ActiveHours) != 0 || len(resolved.Preferences.QuietWindows) != 0 {
			t.Fatalf(
				"Preferences windows = active:%#v quiet:%#v, want ignored",
				resolved.Preferences.ActiveHours,
				resolved.Preferences.QuietWindows,
			)
		}
		assertDiagnostic(t, resolved.Diagnostics, "heartbeat_preference_ignored")
	})

	t.Run("Should reject invalid timezone names", func(t *testing.T) {
		t.Parallel()

		workspaceRoot, sourcePath := heartbeatWorkspace(t)
		resolved, err := Parse(t.Context(), ParseRequest{
			SourcePath:    sourcePath,
			WorkspaceRoot: workspaceRoot,
			Content: []byte(`---
preferences:
  active_hours:
    - timezone: "UTC-3"
      start: "08:00"
      end: "20:00"
---
Wake gently.
`),
			Config: compozyconfig.DefaultHeartbeatConfig(),
		})
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("Parse(invalid timezone) error = %v, want ErrInvalid", err)
		}
		if resolved.Valid {
			t.Fatal("Parse(invalid timezone) Valid = true, want false")
		}
		assertDiagnostic(t, resolved.Diagnostics, "heartbeat_invalid_timezone")
	})

	t.Run("Should reject malformed time windows", func(t *testing.T) {
		t.Parallel()

		workspaceRoot, sourcePath := heartbeatWorkspace(t)
		resolved, err := Parse(t.Context(), ParseRequest{
			SourcePath:    sourcePath,
			WorkspaceRoot: workspaceRoot,
			Content: []byte(`---
preferences:
  active_hours:
    - timezone: "America/Sao_Paulo"
      start: "8am"
      end: "20:00"
---
Wake gently.
`),
			Config: compozyconfig.DefaultHeartbeatConfig(),
		})
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("Parse(malformed window) error = %v, want ErrInvalid", err)
		}
		assertDiagnostic(t, resolved.Diagnostics, "heartbeat_invalid_time_window")
	})
}

func TestHeartbeatRejectsAuthorityClaims(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		wantCode string
	}{
		{
			name: "Should reject ClaimNextRun frontmatter",
			content: `---
ClaimNextRun: true
---
Wake gently.
`,
			wantCode: "heartbeat_forbidden_field",
		},
		{
			name: "Should reject network presence frontmatter",
			content: `---
network:
  presence: "active"
---
Wake gently.
`,
			wantCode: "heartbeat_forbidden_field",
		},
		{
			name: "Should reject task run section",
			content: `---
summary: "ok"
---
# Task Runs
Create durable work here.
`,
			wantCode: "heartbeat_reserved_section",
		},
		{
			name: "Should reject queue body declaration",
			content: `---
summary: "ok"
---
queue: default
`,
			wantCode: "heartbeat_reserved_body_field",
		},
		{
			name: "Should reject lease body declaration",
			content: `---
summary: "ok"
---
lease_duration: 1h
`,
			wantCode: "heartbeat_reserved_body_field",
		},
		{
			name: "Should reject liveness body declaration",
			content: `---
summary: "ok"
---
liveness: alive
`,
			wantCode: "heartbeat_reserved_body_field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			workspaceRoot, sourcePath := heartbeatWorkspace(t)
			resolved, err := Parse(t.Context(), ParseRequest{
				SourcePath:    sourcePath,
				WorkspaceRoot: workspaceRoot,
				Content:       []byte(tt.content),
				Config:        compozyconfig.DefaultHeartbeatConfig(),
			})
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("Parse() error = %v, want ErrInvalid", err)
			}
			if resolved.Valid || resolved.Active {
				t.Fatalf("Parse() flags = valid:%v active:%v, want false/false", resolved.Valid, resolved.Active)
			}
			assertDiagnostic(t, resolved.Diagnostics, tt.wantCode)
		})
	}
}

func TestResolveHeartbeatPolicy(t *testing.T) {
	t.Parallel()

	t.Run("Should treat missing HEARTBEAT as optional inactive policy", func(t *testing.T) {
		t.Parallel()

		workspaceRoot, agentPath := agentWorkspace(t)
		resolved, err := Resolve(t.Context(), ResolveRequest{
			AgentPath:     agentPath,
			WorkspaceRoot: workspaceRoot,
			Config:        compozyconfig.DefaultHeartbeatConfig(),
		})
		if err != nil {
			t.Fatalf("Resolve(missing) error = %v", err)
		}
		if resolved.Present || resolved.Active || !resolved.Valid {
			t.Fatalf(
				"Resolve(missing) flags = present:%v active:%v valid:%v, want false false true",
				resolved.Present,
				resolved.Active,
				resolved.Valid,
			)
		}
		if resolved.ConfigDigest == "" || resolved.Status.ConfigDigest != resolved.ConfigDigest {
			t.Fatalf(
				"Resolve(missing) ConfigDigest = %q status=%q, want populated and mirrored",
				resolved.ConfigDigest,
				resolved.Status.ConfigDigest,
			)
		}
	})

	t.Run("Should resolve policy beside AGENT file without touching runtime authorities", func(t *testing.T) {
		t.Parallel()

		workspaceRoot, agentPath := agentWorkspace(t)
		heartbeatPath := filepath.Join(filepath.Dir(agentPath), FileName)
		if err := os.WriteFile(heartbeatPath, []byte("Inspect context only."), 0o600); err != nil {
			t.Fatalf("write HEARTBEAT.md: %v", err)
		}

		resolved, err := Resolve(t.Context(), ResolveRequest{
			AgentPath:     agentPath,
			WorkspaceRoot: workspaceRoot,
			Config:        compozyconfig.DefaultHeartbeatConfig(),
		})
		if err != nil {
			t.Fatalf("Resolve(existing) error = %v", err)
		}
		if !resolved.Present || !resolved.Active || !resolved.Valid {
			t.Fatalf(
				"Resolve(existing) flags = present:%v active:%v valid:%v, want true true true",
				resolved.Present,
				resolved.Active,
				resolved.Valid,
			)
		}
		if got, want := resolved.Status.MaxWakesPerCycle, compozyconfig.DefaultHeartbeatConfig().MaxWakesPerCycle; got != want {
			t.Fatalf("Status.MaxWakesPerCycle = %d, want %d", got, want)
		}
	})
}

func TestHeartbeatDigestsAndDiagnostics(t *testing.T) {
	t.Parallel()

	t.Run("Should change policy digest when resolved config subset changes", func(t *testing.T) {
		t.Parallel()

		workspaceRoot, sourcePath := heartbeatWorkspace(t)
		content := []byte(`---
summary: "ok"
---
Wake gently.
`)
		first, err := Parse(t.Context(), ParseRequest{
			SourcePath:    sourcePath,
			WorkspaceRoot: workspaceRoot,
			Content:       content,
			Config:        compozyconfig.DefaultHeartbeatConfig(),
		})
		if err != nil {
			t.Fatalf("Parse(first) error = %v", err)
		}
		cfg := compozyconfig.DefaultHeartbeatConfig()
		cfg.DefaultInterval = 45 * time.Minute
		second, err := Parse(t.Context(), ParseRequest{
			SourcePath:    sourcePath,
			WorkspaceRoot: workspaceRoot,
			Content:       content,
			Config:        cfg,
		})
		if err != nil {
			t.Fatalf("Parse(second) error = %v", err)
		}
		if first.ConfigDigest == second.ConfigDigest {
			t.Fatalf("ConfigDigest = %q for both configs, want change", first.ConfigDigest)
		}
		if first.Digest == second.Digest {
			t.Fatalf("Digest = %q for both configs, want config-bound digest change", first.Digest)
		}
	})

	t.Run("Should reject oversized content without leaking body", func(t *testing.T) {
		t.Parallel()

		workspaceRoot, sourcePath := heartbeatWorkspace(t)
		cfg := compozyconfig.DefaultHeartbeatConfig()
		cfg.MaxBodyBytes = 32
		cfg.ContextProjectionBytes = 16
		secretBody := strings.Repeat("x", 40) + " token=super-secret-token-123456"
		resolved, err := Parse(t.Context(), ParseRequest{
			SourcePath:    sourcePath,
			WorkspaceRoot: workspaceRoot,
			Content:       []byte(secretBody),
			Config:        cfg,
		})
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("Parse(oversized) error = %v, want ErrInvalid", err)
		}
		assertDiagnostic(t, resolved.Diagnostics, "heartbeat_oversized_body")
		if diagnosticsContain(resolved.Diagnostics, "super-secret-token-123456") {
			t.Fatalf("Diagnostics leaked oversized body secret: %#v", resolved.Diagnostics)
		}
	})

	t.Run("Should redact malformed frontmatter diagnostics", func(t *testing.T) {
		t.Parallel()

		workspaceRoot, sourcePath := heartbeatWorkspace(t)
		secret := "sk-heartbeat-secret-123456"
		cleanup := diagnostics.RegisterDynamicSecret(secret)
		t.Cleanup(cleanup)

		resolved, err := Parse(t.Context(), ParseRequest{
			SourcePath:    sourcePath,
			WorkspaceRoot: workspaceRoot,
			Content: []byte(`---
summary: "` + secret + `
---
Wake gently.
`),
			Config: compozyconfig.DefaultHeartbeatConfig(),
		})
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("Parse(malformed) error = %v, want ErrInvalid", err)
		}
		assertDiagnostic(t, resolved.Diagnostics, "heartbeat_malformed_frontmatter")
		if diagnosticsContain(resolved.Diagnostics, secret) {
			t.Fatalf("Diagnostics leaked registered secret: %#v", resolved.Diagnostics)
		}
	})

	t.Run("Should reject unsupported frontmatter", func(t *testing.T) {
		t.Parallel()

		workspaceRoot, sourcePath := heartbeatWorkspace(t)
		resolved, err := Parse(t.Context(), ParseRequest{
			SourcePath:    sourcePath,
			WorkspaceRoot: workspaceRoot,
			Content: []byte(`---
unknown: true
---
Wake gently.
`),
			Config: compozyconfig.DefaultHeartbeatConfig(),
		})
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("Parse(unsupported) error = %v, want ErrInvalid", err)
		}
		assertDiagnostic(t, resolved.Diagnostics, "heartbeat_unsupported_field")
	})
}

func TestHeartbeatParserEdgeCases(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		content  string
		wantCode string
	}{
		{
			name: "Should accept string schema version",
			content: `---
version: "1"
enabled: true
context:
  include:
    - self
    - ""
    - self
---
Wake gently.
`,
		},
		{
			name: "Should reject unsupported schema version",
			content: `---
version: 2
---
Wake gently.
`,
			wantCode: "heartbeat_invalid_field_type",
		},
		{
			name: "Should reject non boolean enabled",
			content: `---
enabled: "yes"
---
Wake gently.
`,
			wantCode: "heartbeat_invalid_field_type",
		},
		{
			name: "Should reject non string summary",
			content: `---
summary:
  text: "bad"
---
Wake gently.
`,
			wantCode: "heartbeat_invalid_field_type",
		},
		{
			name: "Should reject non mapping preferences",
			content: `---
preferences:
  - min_interval
---
Wake gently.
`,
			wantCode: "heartbeat_invalid_field_type",
		},
		{
			name: "Should reject unsupported nested preference",
			content: `---
preferences:
  scheduler: "every minute"
---
Wake gently.
`,
			wantCode: "heartbeat_invalid_field_type",
		},
		{
			name: "Should reject non mapping context",
			content: `---
context:
  - self
---
Wake gently.
`,
			wantCode: "heartbeat_invalid_field_type",
		},
		{
			name: "Should reject non string context include item",
			content: `---
context:
  include:
    - self
    - 7
---
Wake gently.
`,
			wantCode: "heartbeat_invalid_field_type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			workspaceRoot, sourcePath := heartbeatWorkspace(t)
			resolved, err := Parse(t.Context(), ParseRequest{
				SourcePath:    sourcePath,
				WorkspaceRoot: workspaceRoot,
				Content:       []byte(tt.content),
				Config:        compozyconfig.DefaultHeartbeatConfig(),
			})
			if tt.wantCode == "" {
				if err != nil {
					t.Fatalf("Parse() error = %v, want nil", err)
				}
				if !slices.Equal(resolved.Preferences.Context.Include, []string{"self"}) {
					t.Fatalf("Context.Include = %#v, want deduped self", resolved.Preferences.Context.Include)
				}
				return
			}
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("Parse() error = %v, want ErrInvalid", err)
			}
			assertDiagnostic(t, resolved.Diagnostics, tt.wantCode)
		})
	}
}

func TestHeartbeatRuntimeBoundaries(t *testing.T) {
	t.Parallel()

	t.Run("Should fail closed for source paths outside workspace", func(t *testing.T) {
		t.Parallel()

		workspaceRoot := t.TempDir()
		sourcePath := filepath.Join(t.TempDir(), FileName)
		resolved, err := Parse(t.Context(), ParseRequest{
			SourcePath:    sourcePath,
			WorkspaceRoot: workspaceRoot,
			Content:       []byte("Wake gently."),
			Config:        compozyconfig.DefaultHeartbeatConfig(),
		})
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("Parse(path escape) error = %v, want ErrInvalid", err)
		}
		assertDiagnostic(t, resolved.Diagnostics, "heartbeat_path_escape")
	})

	t.Run("Should report empty resolve source path as diagnostic error", func(t *testing.T) {
		t.Parallel()

		resolved, err := Resolve(t.Context(), ResolveRequest{
			AgentPath:     "",
			WorkspaceRoot: t.TempDir(),
			Config:        compozyconfig.DefaultHeartbeatConfig(),
		})
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("Resolve(empty agent path) error = %v, want ErrInvalid", err)
		}
		if resolved.Valid {
			t.Fatal("Resolve(empty agent path) Valid = true, want false")
		}
		if !strings.Contains(err.Error(), "heartbeat_invalid_source_path") {
			t.Fatalf("Diagnostic error text = %q, want code", err.Error())
		}
	})

	t.Run("Should honor canceled contexts before parsing", func(t *testing.T) {
		t.Parallel()

		workspaceRoot, sourcePath := heartbeatWorkspace(t)
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		_, err := Parse(ctx, ParseRequest{
			SourcePath:    sourcePath,
			WorkspaceRoot: workspaceRoot,
			Content:       []byte("Wake gently."),
			Config:        compozyconfig.DefaultHeartbeatConfig(),
		})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Parse(canceled) error = %v, want context.Canceled", err)
		}
	})

	t.Run("Should reject invalid config before digesting", func(t *testing.T) {
		t.Parallel()

		cfg := compozyconfig.DefaultHeartbeatConfig()
		cfg.MinInterval = time.Hour
		cfg.DefaultInterval = time.Minute
		_, err := ConfigProvenanceFor(cfg)
		if err == nil {
			t.Fatal("ConfigProvenanceFor(invalid) error = nil, want validation failure")
		}
	})
}

func TestHeartbeatTimeWindows(t *testing.T) {
	t.Parallel()

	t.Run("Should support cross midnight active windows", func(t *testing.T) {
		t.Parallel()

		preferences := Preferences{
			ActiveHours: []TimeWindow{{
				Timezone: "America/Sao_Paulo",
				Start:    "22:00",
				End:      "06:00",
			}},
		}
		late := mustTime(t, "America/Sao_Paulo", 2026, time.January, 2, 23, 30)
		allowed, err := preferences.AllowsAt(late)
		if err != nil {
			t.Fatalf("AllowsAt(late) error = %v", err)
		}
		if !allowed {
			t.Fatal("AllowsAt(late) = false, want true")
		}
		noon := mustTime(t, "America/Sao_Paulo", 2026, time.January, 2, 12, 0)
		allowed, err = preferences.AllowsAt(noon)
		if err != nil {
			t.Fatalf("AllowsAt(noon) error = %v", err)
		}
		if allowed {
			t.Fatal("AllowsAt(noon) = true, want false outside active hours")
		}
	})

	t.Run("Should return errors for invalid direct window evaluation", func(t *testing.T) {
		t.Parallel()

		window := TimeWindow{Timezone: "Invalid/Zone", Start: "08:00", End: "20:00"}
		_, err := window.Contains(time.Now())
		if err == nil {
			t.Fatal("Contains(invalid timezone) error = nil, want error")
		}
	})
}

func TestHeartbeatPromptProjection(t *testing.T) {
	t.Parallel()

	t.Run("Should bound compact prompt contribution", func(t *testing.T) {
		t.Parallel()

		workspaceRoot, sourcePath := heartbeatWorkspace(t)
		cfg := compozyconfig.DefaultHeartbeatConfig()
		cfg.ContextProjectionBytes = 256
		body := "Wake guidance. " + strings.Repeat("Keep inspecting context. ", 80)
		resolved, err := Parse(t.Context(), ParseRequest{
			SourcePath:    sourcePath,
			WorkspaceRoot: workspaceRoot,
			Content:       []byte(body),
			Config:        cfg,
		})
		if err != nil {
			t.Fatalf("Parse(long prompt) error = %v", err)
		}
		if !resolved.Prompt.Truncated {
			t.Fatal("Prompt.Truncated = false, want true")
		}
		if len([]byte(resolved.Prompt.GuidanceMarkdown)) >= len([]byte(body)) {
			t.Fatalf(
				"Prompt.GuidanceMarkdown length = %d, want shorter than source %d",
				len([]byte(resolved.Prompt.GuidanceMarkdown)),
				len([]byte(body)),
			)
		}
	})
}

func heartbeatWorkspace(t *testing.T) (string, string) {
	t.Helper()

	workspaceRoot, agentPath := agentWorkspace(t)
	return workspaceRoot, filepath.Join(filepath.Dir(agentPath), FileName)
}

func agentWorkspace(t *testing.T) (string, string) {
	t.Helper()

	workspaceRoot := t.TempDir()
	agentDir := filepath.Join(workspaceRoot, ".compozy", "agents", "coder")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatalf("create agent dir: %v", err)
	}
	agentPath := filepath.Join(agentDir, "AGENT.md")
	if err := os.WriteFile(agentPath, []byte("---\nname: coder\n---\nPrompt.\n"), 0o600); err != nil {
		t.Fatalf("write AGENT.md: %v", err)
	}
	return workspaceRoot, agentPath
}

func mustTime(
	t *testing.T,
	timezone string,
	year int,
	month time.Month,
	day int,
	hour int,
	minute int,
) time.Time {
	t.Helper()

	location, err := time.LoadLocation(timezone)
	if err != nil {
		t.Fatalf("LoadLocation(%q) error = %v", timezone, err)
	}
	return time.Date(year, month, day, hour, minute, 0, 0, location)
}

func assertDiagnostic(t *testing.T, list []Diagnostic, code string) {
	t.Helper()

	for _, diag := range list {
		if diag.Code == code {
			return
		}
	}
	t.Fatalf("diagnostics = %#v, want code %q", list, code)
}

func diagnosticsContain(list []Diagnostic, needle string) bool {
	for _, diag := range list {
		if strings.Contains(diag.Message, needle) ||
			strings.Contains(diag.Field, needle) ||
			strings.Contains(diag.Section, needle) ||
			strings.Contains(diag.SourcePath, needle) {
			return true
		}
	}
	return false
}

func TestCollectDigest(t *testing.T) {
	t.Run("Should use all five collectors when the policy omits context", func(t *testing.T) {
		t.Parallel()
		root, source := heartbeatWorkspace(t)
		policy, err := Parse(t.Context(), ParseRequest{SourcePath: source, WorkspaceRoot: root, Content: []byte("Check pending work."), Config: compozyconfig.DefaultHeartbeatConfig()})
		if err != nil {
			t.Fatal(err)
		}
		now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		scope := CollectorScope{ProfileID: "p", MainSessionID: "main", Now: now}
		facts := CollectorFacts{Tasks: []TaskFact{{ID: "task", ProfileID: "p", OwnerKind: "agent_session", OwnerSessionID: "main", Status: "blocked"}}, Sessions: []SessionFact{{ID: "session", ProfileID: "p", Health: "stale", LastEventAt: now.Add(-2 * time.Hour), LastSeq: 1}}, Automations: []AutomationFact{{ID: "auto", ProfileID: "p", Status: "failed", Cursor: "1"}}, Monitors: []MonitorFact{{ID: "monitor", SessionID: "main", Fired: true}}}
		digest, err := CollectDigest(scope, facts, policy.Preferences.Context.Include, "policy", "Check pending work.")
		if err != nil {
			t.Fatal(err)
		}
		keys := make([]string, 0, len(digest.Items))
		for _, item := range digest.Items {
			keys = append(keys, item.Key)
		}
		for _, key := range []string{"task:task:blocked", "health:session:stale", "idle:session:1", "auto:auto", "monitor:monitor"} {
			if !slices.Contains(keys, key) {
				t.Fatalf("missing %s in %v", key, keys)
			}
		}
	})
	t.Run("Should enforce scope and exclude the instance chain and hidden sessions", func(t *testing.T) {
		t.Parallel()
		now := time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC)
		scope := CollectorScope{ProfileID: "p", MainSessionID: "main", ChainSessionIDs: []string{"old"}, Now: now}
		facts := CollectorFacts{
			Tasks: []TaskFact{
				{ID: "owned", ProfileID: "p", OwnerKind: "agent_session", OwnerSessionID: "old", Status: "blocked"},
				{ID: "pool", ProfileID: "p", Status: "blocked"},
				{ID: "foreign", ProfileID: "q", OwnerKind: "agent_session", OwnerSessionID: "old", Status: "blocked"},
			},
			Sessions: []SessionFact{
				{ID: "main", ProfileID: "p", Health: "dead"},
				{ID: "old", ProfileID: "p", Health: "dead"},
				{ID: "hidden", ProfileID: "p", Health: "dead", Hidden: true},
				{ID: "visible", ProfileID: "p", Health: "stale", LastEventAt: now.Add(-2 * time.Hour), LastSeq: 4},
				{ID: "workspace", ProfileID: "p", WorkspaceID: "w", Health: "dead"},
			},
			Monitors: []MonitorFact{{ID: "own", SessionID: "main", Fired: true}, {ID: "other", SessionID: "old", Fired: true}},
		}
		got, err := CollectDigest(scope, facts, nil, "policy", "")
		if err != nil {
			t.Fatal(err)
		}
		want := []DigestItem{{Key: "health:visible:stale"}, {Key: "idle:visible:4"}, {Key: "monitor:own"}, {Key: "task:owned:blocked"}}
		if !slices.Equal(got.Items, want) {
			t.Fatalf("items=%v want=%v", got.Items, want)
		}
		scope.Unrestricted = true
		scope.ReachableWorkspaces = []string{"w"}
		expanded, err := CollectDigest(scope, facts, []string{"session_health", "unknown"}, "policy", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(expanded.Items) != 2 || len(expanded.Diagnostics) != 1 {
			t.Fatalf("expanded=%+v", expanded)
		}
		scope.WorkspaceID = "w"
		restricted, err := CollectDigest(scope, facts, []string{"session_health"}, "policy", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(restricted.Items) != 1 || restricted.Items[0].Key != "health:workspace:dead" {
			t.Fatalf("restricted=%+v", restricted)
		}
	})
	t.Run("Should cap failures oldest first and advance only through included facts", func(t *testing.T) {
		t.Parallel()
		scope := CollectorScope{ProfileID: "p"}
		facts := CollectorFacts{}
		for i := 15; i > 0; i-- {
			id := fmt.Sprintf("%02d", i)
			facts.Automations = append(facts.Automations, AutomationFact{ID: id, ProfileID: "p", Status: "failed", Cursor: id})
		}
		first, err := CollectDigest(scope, facts, nil, "p", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(first.Items) != 10 || first.AutomationCursor != "10" {
			t.Fatalf("first=%+v", first)
		}
		slices.Reverse(facts.Automations)
		reordered, err := CollectDigest(scope, facts, nil, "p", "")
		if err != nil {
			t.Fatal(err)
		}
		if first.Fingerprint != reordered.Fingerprint {
			t.Fatal("input ordering changed fingerprint")
		}
		scope.AutomationCursor = first.AutomationCursor
		second, err := CollectDigest(scope, facts, nil, "p", "")
		if err != nil {
			t.Fatal(err)
		}
		if len(second.Items) != 5 || second.AutomationCursor != "15" {
			t.Fatalf("second=%+v", second)
		}
		changed, err := CollectDigest(scope, facts, nil, "p", "new standing instruction")
		if err != nil {
			t.Fatal(err)
		}
		if changed.Fingerprint == second.Fingerprint {
			t.Fatal("instructions must change fingerprint")
		}
	})
}

func TestParseTriageReply(t *testing.T) {
	for _, tc := range []struct {
		name    string
		title   string
		body    string
		wantErr bool
	}{
		{"Should accept character limits with multibyte text", strings.Repeat("界", 120), strings.Repeat("界", 500), false},
		{"Should reject oversized titles", strings.Repeat("界", 121), "details", true},
		{"Should reject oversized bodies", "title", strings.Repeat("界", 501), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reply := `{"proposals":[{"title":"` + tc.title + `","body":"` + tc.body + `","item_keys":[]}]}`
			proposals, err := ParseTriageReply(reply, CheckDigest{}, 1)
			if tc.wantErr {
				if !errors.Is(err, ErrTriageOutputInvalid) {
					t.Fatalf("error=%v", err)
				}
				return
			}
			if err != nil || len(proposals) != 1 || proposals[0].Title != tc.title || proposals[0].Body != tc.body {
				t.Fatalf("proposals=%+v error=%v", proposals, err)
			}
		})
	}
	t.Run("Should discard malformed proposals beyond the limit", func(t *testing.T) {
		t.Parallel()
		for _, extra := range []string{`{}`, `{"title":42}`, `null`, `"invalid"`} {
			proposals, err := ParseTriageReply(
				`{"proposals":[{"title":"first","body":"details","item_keys":[]},`+extra+`]}`,
				CheckDigest{}, 1,
			)
			if err != nil || len(proposals) != 1 || proposals[0].Title != "first" {
				t.Fatalf("extra=%s proposals=%+v error=%v", extra, proposals, err)
			}
		}
	})
	t.Run("Should reject responses outside the proposal contract", func(t *testing.T) {
		t.Parallel()
		for _, reply := range []string{`{"proposals":[]}`, "```json\n{\"proposals\":[]}\n```", "", "nothing_to_report", "[]", "null", "{}", `{"proposals":null}`, `{"proposals":[{"title":"x"}]}`, `{"proposals":[],"extra":true}`, `{"proposals":[]} {"proposals":[]}`, "Explanation\nNOTHING_TO_REPORT", `{"proposals":[],"proposals":[]}`} {
			if _, err := ParseTriageReply(reply, CheckDigest{}, 2); !errors.Is(err, ErrTriageOutputInvalid) {
				t.Errorf("reply %q: %v", reply, err)
			}
		}
	})
	t.Run("Should accept the quiet sentinel", func(t *testing.T) {
		t.Parallel()
		for _, reply := range []string{"NOTHING_TO_REPORT", "  NOTHING_TO_REPORT\n"} {
			proposals, err := ParseTriageReply(reply, CheckDigest{}, 2)
			if err != nil || len(proposals) != 0 {
				t.Fatalf("proposals=%v error=%v", proposals, err)
			}
		}
	})
	t.Run("Should cap proposals and retain only unique digest keys", func(t *testing.T) {
		t.Parallel()
		digest := CheckDigest{Items: []DigestItem{{Key: "b"}, {Key: "a"}}}
		proposals, err := ParseTriageReply(`{"proposals":[{"title":"first","body":"details","item_keys":["b","foreign","a","b"]},{"title":"second","body":"ignored","item_keys":[]}]}`, digest, 1)
		if err != nil {
			t.Fatal(err)
		}
		if len(proposals) != 1 || proposals[0].Title != "first" || !slices.Equal(proposals[0].ItemKeys, []string{"a", "b"}) {
			t.Fatalf("proposals=%+v", proposals)
		}
	})
}

func TestTriageReplyOutcome(t *testing.T) {
	for _, tc := range []struct{ name, reply, result, reason string }{
		{"Should accept one JSON fence containing proposals", "```json\n{\"proposals\":[{\"title\":\"Review\",\"body\":\"Inspect task\",\"item_keys\":[]}]}\n```", "reported", "reported"},
		{"Should finish silence without a report", "NOTHING_TO_REPORT", "quiet", "quiet"},
		{"Should publish valid proposals", `{"proposals":[{"title":"Investigate","body":"A task is blocked.","item_keys":[]}]}`, "reported", "reported"},
		{"Should reject an empty proposal array as silence", `{"proposals":[]}`, "failed", "triage_output_invalid"},
		{"Should report malformed output as failure", "not json", "failed", "triage_output_invalid"},
		{"Should reject oversized report payloads", `{"proposals":[{"title":"Title","body":"` + strings.Repeat("x", 65536) + `","item_keys":[]}]}`, "failed", "triage_output_invalid"},
		{"Should reject empty proposal text before persistence", `{"proposals":[{"title":" ","body":"details","item_keys":[]}]}`, "failed", "triage_output_invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			outcome := TriageReplyOutcome(tc.reply, CheckDigest{}, 2)
			if outcome.Result != tc.result || outcome.Reason != tc.reason {
				t.Fatalf("outcome=%+v", outcome)
			}
			if tc.result == "quiet" {
				if outcome.Report != nil {
					t.Fatal("quiet result produced a report")
				}
				return
			}
			if outcome.Report == nil {
				t.Fatal("missing report")
			}
			outcome.Report.SourceKey = "occ:test"
			if err := outcome.Report.Validate(); err != nil {
				t.Fatalf("unpersistable report: %v", err)
			}
			if tc.result == "failed" && !strings.Contains(string(outcome.Report.Payload), tc.reason) {
				t.Fatalf("failure reason lost: %s", outcome.Report.Payload)
			}
		})
	}
}
