package config

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	burnttoml "github.com/BurntSushi/toml"
	"github.com/compozy/compozy/internal/fileutil"
)

func TestEditConfigOverlayInitialTimezone(t *testing.T) {
	// not parallel: each case controls the process TZ environment for first-config detection.
	for _, tc := range []struct {
		name     string
		tz       string
		existing *string
		want     string
	}{
		{name: "Should persist the host zone for a new user config", tz: "America/Sao_Paulo", want: "America/Sao_Paulo"},
		{name: "Should fall back to UTC for an invalid host zone", tz: "Invalid/Zone", want: "UTC"},
		{name: "Should preserve UTC for an existing empty config", tz: "America/Sao_Paulo", existing: new(""), want: "UTC"},
		{name: "Should preserve an explicit zone on reinstall", tz: "America/Sao_Paulo", existing: new("[automation]\ntimezone = \"Europe/London\"\n"), want: "Europe/London"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("TZ", tc.tz)
			homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
			if err != nil {
				t.Fatal(err)
			}
			if tc.existing != nil {
				writeFile(t, homePaths.ConfigFile, *tc.existing)
			}
			cfg, err := SaveBootstrapConfig(homePaths, "claude", "claude-sonnet-4-6")
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Automation.Timezone != tc.want {
				t.Fatalf("timezone = %q, want %q", cfg.Automation.Timezone, tc.want)
			}
			reloaded, err := LoadGlobalConfig(homePaths)
			if err != nil {
				t.Fatal(err)
			}
			if reloaded.Automation.Timezone != tc.want {
				t.Fatalf("persisted timezone = %q, want %q", reloaded.Automation.Timezone, tc.want)
			}
			if tc.existing == nil {
				contents, err := os.ReadFile(homePaths.ConfigFile)
				if err != nil {
					t.Fatal(err)
				}
				var persisted struct{ Automation struct{ Timezone string } }
				if _, err := burnttoml.Decode(string(contents), &persisted); err != nil {
					t.Fatal(err)
				}
				if persisted.Automation.Timezone != tc.want {
					t.Fatalf("explicit timezone = %q, want %q", persisted.Automation.Timezone, tc.want)
				}
			}
		})
	}
}

func TestHostTimezoneDetection(t *testing.T) {
	t.Run("Should accept only loadable IANA zone names", func(t *testing.T) {
		t.Parallel()
		for _, tc := range []struct{ input, want string }{
			{"America/Sao_Paulo", "America/Sao_Paulo"},
			{":Europe/London", "Europe/London"},
			{"/var/db/timezone/zoneinfo/America/New_York", "America/New_York"},
			{"UTC", "UTC"}, {"Japan", "Japan"}, {"Local", ""}, {"PST", ""}, {"Invalid/Zone", ""},
			{"/etc/localtime", ""}, {"../America/Sao_Paulo", ""},
		} {
			if got := hostIANATimezone(tc.input); got != tc.want {
				t.Errorf("hostIANATimezone(%q) = %q, want %q", tc.input, got, tc.want)
			}
		}
	})
	t.Run("Should detect a zoneinfo symlink when the local name is Local", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		zonePath := filepath.Join(root, "zoneinfo", "America", "Sao_Paulo")
		writeFile(t, zonePath, "")
		localPath := filepath.Join(root, "localtime")
		if err := os.Symlink(zonePath, localPath); err != nil {
			t.Fatal(err)
		}
		if got := detectHostTimezone(
			"",
			false,
			"Local",
			localPath,
			filepath.Join(root, "missing"),
		); got != "America/Sao_Paulo" {
			t.Fatalf("detected timezone = %q", got)
		}
		for _, tz := range []string{localPath, ":" + localPath} {
			if got := detectHostTimezone(tz, true, "Local", localPath, ""); got != "America/Sao_Paulo" {
				t.Fatalf("explicit TZ %q detected timezone = %q", tz, got)
			}
		}
	})
	t.Run("Should read the timezone file and tolerate missing host data", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		missing := filepath.Join(root, "missing")
		zonePath := filepath.Join(root, "timezone")
		writeFile(t, zonePath, "Europe/Paris\n")
		if got := detectHostTimezone("", false, "Local", missing, zonePath); got != "Europe/Paris" {
			t.Fatalf("detected timezone = %q", got)
		}
		if got := detectHostTimezone("", false, "Local", missing, missing); got != "" {
			t.Fatalf("undetectable timezone = %q", got)
		}
		if got := detectHostTimezone("Invalid/Zone", true, "Europe/Paris", missing, zonePath); got != "" {
			t.Fatalf("invalid TZ must not use a different host zone: %q", got)
		}
		if got := detectHostTimezone("", true, "Europe/Paris", missing, zonePath); got != "UTC" {
			t.Fatalf("empty TZ = %q, want UTC", got)
		}
		missingZone := filepath.Join(root, "zoneinfo", "Europe", "Paris")
		if got := detectHostTimezone(missingZone, true, "Europe/Paris", missing, zonePath); got != "" {
			t.Fatalf("missing explicit TZ file must not infer a zone from its path: %q", got)
		}
	})
}

func TestEditConfigOverlayPreservesCommentsAndUntouchedSections(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	target, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeUser, "")
	if err != nil {
		t.Fatalf("ResolveConfigWriteTarget() error = %v", err)
	}

	writeFile(t, homePaths.ConfigFile, `
# defaults block
[defaults]
# keep this comment
agent = "legacy"
provider = "claude"

# untouched section
[observability]
enabled = false
`)

	cfg, err := EditConfigOverlay(homePaths, "", target, func(editor *OverlayEditor) error {
		return editor.SetValue([]string{"defaults", "agent"}, "general")
	})
	if err != nil {
		t.Fatalf("EditConfigOverlay() error = %v", err)
	}
	if got, want := cfg.Defaults.Agent, "general"; got != want {
		t.Fatalf("EditConfigOverlay() Defaults.Agent = %q, want %q", got, want)
	}

	contents, err := os.ReadFile(homePaths.ConfigFile)
	if err != nil {
		t.Fatalf("ReadFile(config) error = %v", err)
	}
	text := string(contents)
	for _, fragment := range []string{"# defaults block", "# keep this comment", "# untouched section", "[observability]", "enabled = false", `provider = "claude"`, `agent = "general"`} {
		if !strings.Contains(text, fragment) {
			t.Fatalf("edited config lacks preserved %q: %s", fragment, text)
		}
	}
	assertPrivatePathMode(t, homePaths.ConfigFile, 0o600)
}

func TestEditConfigOverlayRejectsSymlinkWithoutReadingTarget(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("symlink permissions vary on Windows")
	}

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	target, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeUser, "")
	if err != nil {
		t.Fatalf("ResolveConfigWriteTarget() error = %v", err)
	}

	actualPath := filepath.Join(t.TempDir(), "actual-config.toml")
	before := "[defaults]\nagent = \"leaked-agent\"\nprovider = \"claude\"\n"
	if err := os.WriteFile(actualPath, []byte(before), 0o600); err != nil {
		t.Fatalf("os.WriteFile(actual config) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(target.path), 0o700); err != nil {
		t.Fatalf("os.MkdirAll(config dir) error = %v", err)
	}
	if err := os.Symlink(actualPath, target.path); err != nil {
		t.Fatalf("os.Symlink(config) error = %v", err)
	}

	_, err = EditConfigOverlay(homePaths, "", target, func(editor *OverlayEditor) error {
		return editor.SetValue([]string{"defaults", "agent"}, "general")
	})
	if err == nil {
		t.Fatal("EditConfigOverlay(symlink) error = nil, want symlink rejection")
	}
	if strings.Contains(err.Error(), "leaked-agent") {
		t.Fatalf("EditConfigOverlay(symlink) error leaked target content: %v", err)
	}
	after, err := os.ReadFile(actualPath)
	if err != nil {
		t.Fatalf("os.ReadFile(actual config after edit) error = %v", err)
	}
	if string(after) != before {
		t.Fatalf("symlink edit changed target config\nbefore:\n%s\nafter:\n%s", before, string(after))
	}
}

func TestEditConfigOverlayUpdatesExistingBooleanValue(t *testing.T) {
	t.Parallel()

	editor, err := newOverlayEditor(ConfigName, []byte("[observability]\nenabled = true\nretention_days = 21\n"))
	if err != nil {
		t.Fatalf("newOverlayEditor() error = %v", err)
	}
	if err := editor.SetValue([]string{"observability", "enabled"}, false); err != nil {
		t.Fatalf("editor.SetValue() error = %v", err)
	}
	rendered, err := editor.Bytes()
	if err != nil {
		t.Fatalf("editor.Bytes() error = %v", err)
	}
	text := string(rendered)
	for _, want := range []string{
		"[observability]",
		"enabled = false",
		`retention_days = 21`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered config missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "=\n") {
		t.Fatalf("rendered config corrupted by boolean update\n%s", text)
	}

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	target, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeUser, "")
	if err != nil {
		t.Fatalf("ResolveConfigWriteTarget() error = %v", err)
	}

	writeFile(t, homePaths.ConfigFile, `
[observability]
enabled = true
retention_days = 21
`)

	cfg, err := EditConfigOverlay(homePaths, "", target, func(editor *OverlayEditor) error {
		return editor.SetValue([]string{"observability", "enabled"}, false)
	})
	if err != nil {
		t.Fatalf("EditConfigOverlay() error = %v", err)
	}
	if got, want := cfg.Observability.Enabled, false; got != want {
		t.Fatalf("EditConfigOverlay() Observability.Enabled = %v, want %v", got, want)
	}

	contents, err := os.ReadFile(homePaths.ConfigFile)
	if err != nil {
		t.Fatalf("ReadFile(config) error = %v", err)
	}
	text = string(contents)
	for _, want := range []string{
		"[observability]",
		"enabled = false",
		`retention_days = 21`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("config contents missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "false[observability]") {
		t.Fatalf("config contents corrupted by boolean update\n%s", text)
	}
}

func TestEditConfigOverlayRejectsUnsupportedMutation(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	target, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeUser, "")
	if err != nil {
		t.Fatalf("ResolveConfigWriteTarget() error = %v", err)
	}

	writeFile(t, homePaths.ConfigFile, `
defaults = "legacy"
`)

	before, err := os.ReadFile(homePaths.ConfigFile)
	if err != nil {
		t.Fatalf("ReadFile(before) error = %v", err)
	}

	_, err = EditConfigOverlay(homePaths, "", target, func(editor *OverlayEditor) error {
		return editor.SetValue([]string{"defaults", "agent"}, "general")
	})
	if err == nil {
		t.Fatal("EditConfigOverlay() error = nil, want unsupported mutation failure")
	}
	if !errors.Is(err, ErrUnsupportedTOMLMutation) {
		t.Fatalf("EditConfigOverlay() error = %v, want ErrUnsupportedTOMLMutation", err)
	}
	if !strings.Contains(err.Error(), `defaults`) {
		t.Fatalf("EditConfigOverlay() error = %q, want path context", err.Error())
	}

	after, err := os.ReadFile(homePaths.ConfigFile)
	if err != nil {
		t.Fatalf("ReadFile(after) error = %v", err)
	}
	if !bytes.Equal(after, before) {
		t.Fatalf("config file changed on unsupported mutation\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestEditConfigOverlayCreatesNestedConfigSections(t *testing.T) {
	t.Run("Should persist new nested sections through current config keys", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		target, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeUser, "")
		if err != nil {
			t.Fatalf("ResolveConfigWriteTarget() error = %v", err)
		}

		writeFile(t, homePaths.ConfigFile, `
[daemon]
socket = "/tmp/compozy.sock"

[http]
host = "127.0.0.1"
port = 4317
`)

		initial, err := os.ReadFile(homePaths.ConfigFile)
		if err != nil {
			t.Fatalf("ReadFile(initial config) error = %v", err)
		}

		editor, err := newOverlayEditor(homePaths.ConfigFile, initial)
		if err != nil {
			t.Fatalf("newOverlayEditor() error = %v", err)
		}

		updates := []struct {
			path  []string
			value any
		}{
			{path: []string{"skills", "enabled"}, value: true},
			{path: []string{"skills", "disabled_skills"}, value: []string{"compozy"}},
			{path: []string{"skills", "poll_interval"}, value: "3s"},
			{path: []string{"extensions", "sources", "github", "enabled"}, value: true},
			{path: []string{"extensions", "sources", "github", "base_url"}, value: "https://github.example"},
		}
		for _, update := range updates {
			if err := editor.SetValue(update.path, update.value); err != nil {
				rendered, renderErr := editor.Bytes()
				if renderErr != nil {
					t.Fatalf("editor.SetValue(%v) error = %v; editor.Bytes() error = %v", update.path, err, renderErr)
				}
				t.Fatalf("editor.SetValue(%v) error = %v\n%s", update.path, err, rendered)
			}
		}

		rendered, err := editor.Bytes()
		if err != nil {
			t.Fatalf("editor.Bytes() error = %v", err)
		}
		if _, err := loadConfigOverlayBytes(rendered, homePaths.ConfigFile); err != nil {
			t.Fatalf("loadConfigOverlayBytes(rendered) error = %v\n%s", err, rendered)
		}

		_, err = EditConfigOverlay(homePaths, "", target, func(editor *OverlayEditor) error {
			for _, update := range updates {
				if err := editor.SetValue(update.path, update.value); err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			t.Fatalf("EditConfigOverlay() error = %v", err)
		}

		contents, err := os.ReadFile(homePaths.ConfigFile)
		if err != nil {
			t.Fatalf("ReadFile(config) error = %v", err)
		}
		if _, err := loadConfigOverlayBytes(contents, homePaths.ConfigFile); err != nil {
			t.Fatalf("loadConfigOverlayBytes() error = %v\n%s", err, contents)
		}

		text := string(contents)
		for _, want := range []string{
			"[skills]",
			"enabled = true",
			`disabled_skills = ["compozy"]`,
			`poll_interval = "3s"`,
			"[extensions.sources.github]",
			`enabled = true`,
			`base_url = "https://github.example"`,
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("config contents missing %q\n%s", want, text)
			}
		}
	})
}

func TestResolveWriteTargets(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")

	tests := []struct {
		name      string
		scope     WriteScope
		sidecar   bool
		wantKind  WriteTargetKind
		wantPath  string
		wantError string
	}{
		{
			name:     "global config",
			scope:    WriteScopeUser,
			wantKind: WriteTargetGlobalConfig,
			wantPath: homePaths.ConfigFile,
		},
		{
			name:     "global sidecar",
			scope:    WriteScopeUser,
			sidecar:  true,
			wantKind: WriteTargetGlobalMCPSidecar,
			wantPath: globalMCPJSONFile(homePaths),
		},
		{
			name:     "workspace config",
			scope:    WriteScopeWorkspace,
			wantKind: WriteTargetWorkspaceConfig,
			wantPath: workspaceConfigFile(workspaceRoot),
		},
		{
			name:     "workspace sidecar",
			scope:    WriteScopeWorkspace,
			sidecar:  true,
			wantKind: WriteTargetWorkspaceMCPSidecar,
			wantPath: workspaceMCPJSONFile(workspaceRoot),
		},
		{
			name:      "workspace requires root",
			scope:     WriteScopeWorkspace,
			wantError: "workspace write target requires a workspace root",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			root := workspaceRoot
			if tt.wantError != "" {
				root = ""
			}

			var (
				target WriteTarget
				err    error
			)
			if tt.sidecar {
				target, err = ResolveMCPSidecarWriteTarget(homePaths, root, tt.scope, "")
			} else {
				target, err = ResolveConfigWriteTarget(homePaths, root, tt.scope, "")
			}

			if tt.wantError != "" {
				if err == nil {
					t.Fatalf("resolve write target error = nil, want %q", tt.wantError)
				}
				if !strings.Contains(err.Error(), tt.wantError) {
					t.Fatalf("resolve write target error = %q, want %q", err.Error(), tt.wantError)
				}
				return
			}

			if err != nil {
				t.Fatalf("resolve write target error = %v", err)
			}
			if got, want := target.Kind(), tt.wantKind; got != want {
				t.Fatalf("target.Kind() = %q, want %q", got, want)
			}
			if got, want := target.path, tt.wantPath; got != want {
				t.Fatalf("target.path = %q, want %q", got, want)
			}
		})
	}
}

func TestEditConfigOverlayValidationBlocksInvalidWrite(t *testing.T) {
	t.Parallel()
	t.Run("Should reject invalid configuration before persisting", func(t *testing.T) {
		t.Parallel()

		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		target, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeUser, "")
		if err != nil {
			t.Fatalf("ResolveConfigWriteTarget() error = %v", err)
		}

		writeFile(t, homePaths.ConfigFile, `
[permissions]
mode = "approve-all"
`)

		before, err := os.ReadFile(homePaths.ConfigFile)
		if err != nil {
			t.Fatalf("ReadFile(before) error = %v", err)
		}

		_, err = EditConfigOverlay(homePaths, "", target, func(editor *OverlayEditor) error {
			return editor.SetValue([]string{"permissions", "mode"}, "invalid-mode")
		})
		if err == nil {
			t.Fatal("EditConfigOverlay() error = nil, want validation failure")
		}
		if !strings.Contains(err.Error(), "permissions.mode") {
			t.Fatalf("EditConfigOverlay() error = %q, want permissions.mode context", err.Error())
		}

		after, err := os.ReadFile(homePaths.ConfigFile)
		if err != nil {
			t.Fatalf("ReadFile(after) error = %v", err)
		}
		if !bytes.Equal(after, before) {
			t.Fatalf("config file changed after validation failure\nbefore:\n%s\nafter:\n%s", before, after)
		}
	})
	for _, existed := range []bool{false, true} {
		name := "Should restore the original overlay after runtime rejection"
		if !existed {
			name = "Should remove a newly created overlay after runtime rejection"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			home, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
			if err != nil {
				t.Fatal(err)
			}
			target, err := ResolveConfigWriteTarget(home, "", WriteScopeUser, "")
			if err != nil {
				t.Fatal(err)
			}
			original := "# preserve user comments\n[skills]\nenabled = true\n"
			if existed {
				writeFile(t, home.ConfigFile, original)
			}
			rejected := errors.New("runtime rejected source")
			_, err = EditConfigOverlayAndApply(home, "", target, func(editor *OverlayEditor) error {
				return editor.SetValue([]string{"skills", "enabled"}, false)
			}, func(cfg Config) error {
				if cfg.Skills.Enabled {
					t.Fatal("runtime received old config")
				}
				return rejected
			})
			if !errors.Is(err, rejected) {
				t.Fatalf("apply error = %v", err)
			}
			after, err := os.ReadFile(home.ConfigFile)
			if existed {
				if err != nil || string(after) != original {
					t.Fatalf("rollback changed original bytes: %q, %v", after, err)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("new config remains after rejection: %v", err)
			}
		})
	}
	t.Run("Should preserve the next serialized overlay edit after a rejected mutation", func(t *testing.T) {
		t.Parallel()
		home, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatal(err)
		}
		target, err := ResolveConfigWriteTarget(home, "", WriteScopeUser, "")
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, home.ConfigFile, "# keep this comment\n[skills]\nenabled = true\n")
		started, done := make(chan struct{}), make(chan error, 1)
		rejected := errors.New("runtime rejected source")
		_, err = EditConfigOverlayAndApply(home, "", target, func(editor *OverlayEditor) error {
			return editor.SetValue([]string{"skills", "enabled"}, false)
		}, func(Config) error {
			go func() {
				close(started)
				_, err := EditConfigOverlay(home, "", target, func(editor *OverlayEditor) error {
					return editor.SetValue([]string{"permissions", "mode"}, "approve-all")
				})
				done <- err
			}()
			<-started
			return rejected
		})
		secondErr := <-done
		if !errors.Is(err, rejected) || secondErr != nil {
			t.Fatalf("concurrent edits = %v, %v", err, secondErr)
		}
		content, err := os.ReadFile(home.ConfigFile)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"# keep this comment", "enabled = true", "approve-all"} {
			if !strings.Contains(string(content), want) {
				t.Fatalf("serialized overlay lost %q: %s", want, content)
			}
		}
	})
	t.Run("Should preserve an external replacement instead of overwriting it during rollback", func(t *testing.T) {
		t.Parallel()
		home, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatal(err)
		}
		target, err := ResolveConfigWriteTarget(home, "", WriteScopeUser, "")
		if err != nil {
			t.Fatal(err)
		}
		replacement := "# external edit\n[skills]\nenabled = true\n"
		rejected := errors.New("runtime rejected source")
		_, err = EditConfigOverlayAndApply(home, "", target, func(editor *OverlayEditor) error {
			return editor.SetValue([]string{"skills", "enabled"}, false)
		}, func(Config) error {
			writeFile(t, home.ConfigFile, replacement)
			return rejected
		})
		if !errors.Is(err, rejected) || !strings.Contains(err.Error(), "concurrent overlay edit") {
			t.Fatalf("concurrent edit diagnostic = %v", err)
		}
		content, err := os.ReadFile(home.ConfigFile)
		if err != nil || string(content) != replacement {
			t.Fatalf("external edit was lost: %q, %v", content, err)
		}
	})
}

func TestWriteScopeValidationAndTargetScope(t *testing.T) {
	t.Parallel()

	t.Run("Should reject gateway writes at workspace scope", func(t *testing.T) {
		t.Parallel()
		if err := ValidateConfigWriteScope(
			WriteScopeWorkspace,
			[]string{"gateway", "active_connection"},
		); err == nil || !strings.Contains(err.Error(), "global-only") {
			t.Fatalf("ValidateConfigWriteScope() error = %v, want global-only rejection", err)
		}
	})

	t.Run("Should reject shell preference writes at workspace scope", func(t *testing.T) {
		t.Parallel()
		if err := ValidateConfigWriteScope(
			WriteScopeWorkspace,
			[]string{"shell", "sessions", "sort"},
		); err == nil || !strings.Contains(err.Error(), "global-only") {
			t.Fatalf("ValidateConfigWriteScope() error = %v, want global-only rejection", err)
		}
	})

	t.Run("Should reject boot-scoped clarify timeout writes at workspace scope", func(t *testing.T) {
		t.Parallel()
		err := ValidateConfigWriteScope(
			WriteScopeWorkspace,
			[]string{"tools", "clarify", "timeout"},
		)
		if err == nil || !strings.Contains(err.Error(), "global-only") {
			t.Fatalf("ValidateConfigWriteScope() error = %v, want global-only rejection", err)
		}
		if !strings.Contains(err.Error(), "--scope user") {
			t.Fatalf("ValidateConfigWriteScope() error = %v, want user-scope guidance", err)
		}
		if err := ValidateConfigWriteScope(
			WriteScopeUser,
			[]string{"tools", "clarify", "timeout"},
		); err != nil {
			t.Fatalf("ValidateConfigWriteScope(user) error = %v, want nil", err)
		}
	})

	t.Run("Should reject non-source skill fields at workspace scope", func(t *testing.T) {
		t.Parallel()
		err := ValidateConfigWriteScope(
			WriteScopeWorkspace,
			[]string{"skills", "poll_interval"},
		)
		sourceErr, sourceErrOK := errors.AsType[*SkillSourceValidationError](err)
		if !sourceErrOK {
			t.Fatalf("ValidateConfigWriteScope() error = %v, want SkillSourceValidationError", err)
		}
		if sourceErr.Code != "workspace_scope_field_forbidden" || sourceErr.Field != "poll_interval" {
			t.Fatalf("ValidateConfigWriteScope() error = %#v, want forbidden poll_interval", sourceErr)
		}
	})

	for _, scope := range []WriteScope{WriteScopeUser, WriteScopeWorkspace} {
		if err := scope.Validate(); err != nil {
			t.Fatalf("WriteScope(%q).Validate() error = %v", scope, err)
		}
	}

	if err := WriteScope("invalid").Validate(); err == nil {
		t.Fatal(`WriteScope("invalid").Validate() error = nil, want failure`)
	} else if !strings.Contains(err.Error(), `invalid write scope "invalid"`) {
		t.Fatalf(`WriteScope("invalid").Validate() error = %q, want invalid scope context`, err.Error())
	}

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	workspaceRoot := filepath.Join(t.TempDir(), "workspace")

	globalTarget, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeUser, "")
	if err != nil {
		t.Fatalf("ResolveConfigWriteTarget(global) error = %v", err)
	}
	if got, want := globalTarget.Scope(), WriteScopeUser; got != want {
		t.Fatalf("globalTarget.Scope() = %q, want %q", got, want)
	}

	workspaceTarget, err := ResolveMCPSidecarWriteTarget(homePaths, workspaceRoot, WriteScopeWorkspace, "")
	if err != nil {
		t.Fatalf("ResolveMCPSidecarWriteTarget(workspace) error = %v", err)
	}
	if got, want := workspaceTarget.Scope(), WriteScopeWorkspace; got != want {
		t.Fatalf("workspaceTarget.Scope() = %q, want %q", got, want)
	}
}

func TestOverlayEditorSetTableMutations(t *testing.T) {
	t.Parallel()

	t.Run("Should preserve an explicitly empty table", func(t *testing.T) {
		t.Parallel()

		editor, err := newOverlayEditor(ConfigName, []byte{})
		if err != nil {
			t.Fatalf("newOverlayEditor() error = %v", err)
		}
		if err := editor.SetTable(
			[]string{"loops", "inputs", "release", "runtime"},
			map[string]any{},
		); err != nil {
			t.Fatalf("editor.SetTable(empty) error = %v", err)
		}
		rendered, err := editor.Bytes()
		if err != nil {
			t.Fatalf("editor.Bytes() error = %v", err)
		}
		if !strings.Contains(string(rendered), "[loops.inputs.release.runtime]") {
			t.Fatalf("rendered config = %q, want explicit empty runtime table", rendered)
		}
	})

	t.Run("Should replace an implicit parent represented only by a nested empty table", func(t *testing.T) {
		t.Parallel()

		editor, err := newOverlayEditor(ConfigName, []byte("[loops.inputs.release.runtime]\n"))
		if err != nil {
			t.Fatalf("newOverlayEditor() error = %v", err)
		}
		if err := editor.SetTable([]string{"loops", "inputs", "release"}, map[string]any{
			"runtime": map[string]any{"model": "gpt-5.6"},
		}); err != nil {
			t.Fatalf("editor.SetTable(implicit parent) error = %v", err)
		}
		rendered, err := editor.Bytes()
		if err != nil {
			t.Fatalf("editor.Bytes() error = %v", err)
		}
		if strings.Count(string(rendered), "[loops.inputs.release.runtime]") != 1 ||
			!strings.Contains(string(rendered), `model = "gpt-5.6"`) {
			t.Fatalf("rendered config = %q, want one populated runtime table", rendered)
		}
	})

	t.Run("Should replace existing table", func(t *testing.T) {
		t.Parallel()

		editor, err := newOverlayEditor(ConfigName, []byte(`
	# provider block
	[providers.openai]
	models = { default = "gpt-4o" }
	command = "openai"

[defaults]
agent = "general"
`))
		if err != nil {
			t.Fatalf("newOverlayEditor() error = %v", err)
		}

		err = editor.SetTable([]string{"providers", "openai"}, map[string]any{
			"models":  map[string]any{"default": "gpt-5"},
			"command": "openai-next",
		})
		if err != nil {
			t.Fatalf("editor.SetTable() error = %v", err)
		}

		rendered, err := editor.Bytes()
		if err != nil {
			t.Fatalf("editor.Bytes() error = %v", err)
		}
		text := string(rendered)

		for _, want := range []string{
			"[providers.openai]",
			`default = "gpt-5"`,
			`command = "openai-next"`,
			"[defaults]",
			`agent = "general"`,
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("rendered config missing %q\n%s", want, text)
			}
		}
		if strings.Contains(text, `default = "gpt-4o"`) {
			t.Fatalf("rendered config still contains old model\n%s", text)
		}
	})

	t.Run("Should replace table including nested subtables", func(t *testing.T) {
		t.Parallel()

		editor, err := newOverlayEditor(ConfigName, []byte(`
[providers.openai]
command = "openai"

[providers.openai.models]
default = "gpt-4o"
`))
		if err != nil {
			t.Fatalf("newOverlayEditor() error = %v", err)
		}

		err = editor.SetTable([]string{"providers", "openai"}, map[string]any{
			"command": "openai-next",
		})
		if err != nil {
			t.Fatalf("editor.SetTable() error = %v", err)
		}

		rendered, err := editor.Bytes()
		if err != nil {
			t.Fatalf("editor.Bytes() error = %v", err)
		}
		text := string(rendered)
		if !strings.Contains(text, `command = "openai-next"`) {
			t.Fatalf("rendered config missing replacement command:\n%s", text)
		}
		for _, stale := range []string{
			"[providers.openai.models]",
			`default = "gpt-4o"`,
		} {
			if strings.Contains(text, stale) {
				t.Fatalf("rendered config still contains stale nested value %q:\n%s", stale, text)
			}
		}
	})

	t.Run("Should render explicit empty array-of-tables values", func(t *testing.T) {
		t.Parallel()

		editor, err := newOverlayEditor(ConfigName, []byte{})
		if err != nil {
			t.Fatalf("newOverlayEditor() error = %v", err)
		}

		err = editor.SetTable([]string{"providers", "openai", "models"}, map[string]any{
			"curated": []map[string]any{},
		})
		if err != nil {
			t.Fatalf("editor.SetTable() error = %v", err)
		}

		rendered, err := editor.Bytes()
		if err != nil {
			t.Fatalf("editor.Bytes() error = %v", err)
		}
		text := string(rendered)
		for _, want := range []string{
			"[providers.openai.models]",
			`curated = []`,
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("rendered config missing %q\n%s", want, text)
			}
		}
	})
}

func TestOverlayEditorArrayTableMutations(t *testing.T) {
	t.Parallel()

	editor, err := newOverlayEditor(ConfigName, []byte(`
[[hooks.declarations]]
name = "alpha"
event = "session.start"
command = "old"

[[hooks.declarations]]
name = "gamma"
event = "session.end"
command = "keep"
`))
	if err != nil {
		t.Fatalf("newOverlayEditor() error = %v", err)
	}

	if !editor.HasPath([]string{"hooks", "declarations"}) {
		t.Fatal(`editor.HasPath(["hooks","declarations"]) = false, want true`)
	}

	if err := editor.UpsertArrayTableItem(
		[]string{"hooks", "declarations"},
		"name",
		"alpha",
		map[string]any{
			"command": "updated",
			"event":   "session.start",
			"args":    []string{"--debug"},
		},
	); err != nil {
		t.Fatalf("editor.UpsertArrayTableItem(replace) error = %v", err)
	}

	if err := editor.UpsertArrayTableItem(
		[]string{"hooks", "declarations"},
		"name",
		"beta",
		map[string]any{
			"command": "beta",
			"event":   "tool.start",
		},
	); err != nil {
		t.Fatalf("editor.UpsertArrayTableItem(append) error = %v", err)
	}

	deleted, err := editor.DeleteArrayTableItem([]string{"hooks", "declarations"}, "name", "gamma")
	if err != nil {
		t.Fatalf("editor.DeleteArrayTableItem(gamma) error = %v", err)
	}
	if !deleted {
		t.Fatal("editor.DeleteArrayTableItem(gamma) deleted = false, want true")
	}

	deleted, err = editor.DeleteArrayTableItem([]string{"hooks", "declarations"}, "name", "missing")
	if err != nil {
		t.Fatalf("editor.DeleteArrayTableItem(missing) error = %v", err)
	}
	if deleted {
		t.Fatal("editor.DeleteArrayTableItem(missing) deleted = true, want false")
	}

	rendered, err := editor.Bytes()
	if err != nil {
		t.Fatalf("editor.Bytes() error = %v", err)
	}
	text := string(rendered)

	if got, want := strings.Count(text, "[[hooks.declarations]]"), 2; got != want {
		t.Fatalf("strings.Count(array tables) = %d, want %d\n%s", got, want, text)
	}
	for _, want := range []string{
		`name = "alpha"`,
		`command = "updated"`,
		`args = ["--debug"]`,
		`name = "beta"`,
		`command = "beta"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered hooks config missing %q\n%s", want, text)
		}
	}
	for _, unwanted := range []string{
		`command = "old"`,
		`name = "gamma"`,
		`command = "keep"`,
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered hooks config still contains %q\n%s", unwanted, text)
		}
	}
}

func TestOverlayEditorDeleteArrayTableItemRemovesNestedSubtables(t *testing.T) {
	t.Parallel()

	editor, err := newOverlayEditor(ConfigName, []byte(`
[[hooks.declarations]]
name = "alpha"
event = "tool.pre_call"

[hooks.declarations.executor]
command = "/bin/alpha"
args = ["--json"]

[[hooks.declarations]]
name = "beta"
event = "tool.post_call"
command = "/bin/beta"
`))
	if err != nil {
		t.Fatalf("newOverlayEditor() error = %v", err)
	}

	deleted, err := editor.DeleteArrayTableItem([]string{"hooks", "declarations"}, "name", "alpha")
	if err != nil {
		t.Fatalf("editor.DeleteArrayTableItem(alpha) error = %v", err)
	}
	if !deleted {
		t.Fatal("editor.DeleteArrayTableItem(alpha) deleted = false, want true")
	}

	rendered, err := editor.Bytes()
	if err != nil {
		t.Fatalf("editor.Bytes() error = %v", err)
	}
	text := string(rendered)
	for _, unwanted := range []string{
		`name = "alpha"`,
		"[hooks.declarations.executor]",
		`command = "/bin/alpha"`,
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered config still contains %q\n%s", unwanted, text)
		}
	}
	for _, want := range []string{
		`name = "beta"`,
		`command = "/bin/beta"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered config missing %q\n%s", want, text)
		}
	}
}

func TestOverlayEditorDeleteAndHasPath(t *testing.T) {
	t.Parallel()

	editor, err := newOverlayEditor(ConfigName, []byte(`
[defaults]
agent = "general"
provider = "openai"

	[providers.openai]
	models = { default = "gpt-4o" }
	command = "openai"
	`))
	if err != nil {
		t.Fatalf("newOverlayEditor() error = %v", err)
	}

	for _, path := range [][]string{
		{"defaults", "provider"},
		{"providers", "openai"},
	} {
		if !editor.HasPath(path) {
			t.Fatalf("editor.HasPath(%v) = false, want true", path)
		}
	}
	if editor.HasPath([]string{"providers", "missing"}) {
		t.Fatal(`editor.HasPath(["providers","missing"]) = true, want false`)
	}

	if err := editor.Delete([]string{"defaults", "provider"}); err != nil {
		t.Fatalf("editor.Delete(defaults.provider) error = %v", err)
	}
	if err := editor.Delete([]string{"providers", "openai"}); err != nil {
		t.Fatalf("editor.Delete(providers.openai) error = %v", err)
	}

	if editor.HasPath([]string{"defaults", "provider"}) {
		t.Fatal(`editor.HasPath(["defaults","provider"]) = true after delete`)
	}
	if editor.HasPath([]string{"providers", "openai"}) {
		t.Fatal(`editor.HasPath(["providers","openai"]) = true after delete`)
	}

	rendered, err := editor.Bytes()
	if err != nil {
		t.Fatalf("editor.Bytes() error = %v", err)
	}
	text := string(rendered)
	for _, want := range []string{
		"[defaults]",
		`agent = "general"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("rendered config missing %q\n%s", want, text)
		}
	}
	for _, unwanted := range []string{
		`provider = "openai"`,
		"[providers.openai]",
		`default = "gpt-4o"`,
		`command = "openai"`,
	} {
		if strings.Contains(text, unwanted) {
			t.Fatalf("rendered config still contains %q\n%s", unwanted, text)
		}
	}
}

// TestOverlayEditorDeleteImplicitTableSubtree verifies implicit parent table deletion.
func TestOverlayEditorDeleteImplicitTableSubtree(t *testing.T) {
	t.Parallel()

	t.Run("Should delete descendant tables when parent table is implicit", func(t *testing.T) {
		t.Parallel()

		editor, err := newOverlayEditor(ConfigName, []byte(`
	[defaults]
	provider = "codex"

	[providers.codex.models]
	default = "gpt-5.5"

	[[providers.codex.models.curated]]
	id = "gpt-5.4"

	[providers.claude.models]
	default = "sonnet"
	`))
		if err != nil {
			t.Fatalf("newOverlayEditor() error = %v", err)
		}

		if err := editor.Delete([]string{"providers", "codex"}); err != nil {
			t.Fatalf("editor.Delete(providers.codex) error = %v", err)
		}

		rendered, err := editor.Bytes()
		if err != nil {
			t.Fatalf("editor.Bytes() error = %v", err)
		}
		text := string(rendered)
		for _, unwanted := range []string{
			"[providers.codex.models]",
			"[[providers.codex.models.curated]]",
			`default = "gpt-5.5"`,
			`id = "gpt-5.4"`,
		} {
			if strings.Contains(text, unwanted) {
				t.Fatalf("rendered config still contains %q\n%s", unwanted, text)
			}
		}
		for _, want := range []string{
			"[defaults]",
			`provider = "codex"`,
			"[providers.claude.models]",
			`default = "sonnet"`,
		} {
			if !strings.Contains(text, want) {
				t.Fatalf("rendered config missing %q\n%s", want, text)
			}
		}
	})
}

func TestNormalizeTOMLValue(t *testing.T) {
	t.Parallel()

	supported := []struct {
		name  string
		input any
		want  any
	}{
		{name: "string", input: "value", want: "value"},
		{name: "bool", input: true, want: true},
		{name: "int", input: int(3), want: int64(3)},
		{name: "int8", input: int8(4), want: int64(4)},
		{name: "int16", input: int16(5), want: int64(5)},
		{name: "int32", input: int32(6), want: int64(6)},
		{name: "int64", input: int64(7), want: int64(7)},
		{name: "uint", input: uint(8), want: uint64(8)},
		{name: "uint8", input: uint8(9), want: uint64(9)},
		{name: "uint16", input: uint16(10), want: uint64(10)},
		{name: "uint32", input: uint32(11), want: uint64(11)},
		{name: "uint64", input: uint64(12), want: uint64(12)},
		{name: "float32", input: float32(1.5), want: float64(1.5)},
		{name: "float64", input: 2.5, want: 2.5},
		{name: "string slice", input: []string{"a", "b"}, want: []string{"a", "b"}},
		{name: "bool slice", input: []bool{true, false}, want: []bool{true, false}},
		{name: "int slice", input: []int{1, 2}, want: []int64{1, 2}},
		{name: "int64 slice", input: []int64{3, 4}, want: []int64{3, 4}},
		{name: "uint64 slice", input: []uint64{5, 6}, want: []uint64{5, 6}},
		{name: "float64 slice", input: []float64{7.5, 8.5}, want: []float64{7.5, 8.5}},
		{name: "any slice", input: []any{"a", 2, true}, want: []any{"a", int64(2), true}},
	}

	for _, tt := range supported {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := normalizeTOMLValue(tt.input)
			if err != nil {
				t.Fatalf("normalizeTOMLValue(%T) error = %v", tt.input, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("normalizeTOMLValue(%T) = %#v, want %#v", tt.input, got, tt.want)
			}
		})
	}

	rejected := []struct {
		name      string
		input     any
		wantError string
	}{
		{name: "nil", input: nil, wantError: "nil TOML values are not supported"},
		{name: "table map", input: map[string]any{"value": "x"}, wantError: "table helpers"},
		{name: "string map", input: map[string]string{"value": "x"}, wantError: "table helpers"},
		{name: "array table maps", input: []map[string]any{{"name": "alpha"}}, wantError: "table helpers"},
		{name: "any slice with table", input: []any{map[string]any{"value": "x"}}, wantError: "table helpers"},
		{name: "unsupported type", input: struct{}{}, wantError: "unsupported TOML value type"},
	}

	for _, tt := range rejected {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := normalizeTOMLValue(tt.input)
			if err == nil {
				t.Fatalf("normalizeTOMLValue(%T) error = nil, want %q", tt.input, tt.wantError)
			}
			if !strings.Contains(err.Error(), tt.wantError) {
				t.Fatalf("normalizeTOMLValue(%T) error = %q, want %q", tt.input, err.Error(), tt.wantError)
			}
		})
	}
}

func TestPersistenceHelperMapsAndStringDecoding(t *testing.T) {
	t.Parallel()

	converted := stringMapToAny(map[string]string{"TOKEN": "value"})
	if got, want := converted["TOKEN"], "value"; got != want {
		t.Fatalf("stringMapToAny()[TOKEN] = %#v, want %q", got, want)
	}

	original := map[string]any{"enabled": true}
	cloned := cloneStringAnyMap(original)
	cloned["enabled"] = false
	if got, want := original["enabled"], true; got != want {
		t.Fatalf("cloneStringAnyMap() mutated original value = %#v, want %v", got, want)
	}

	if got, ok := decodeStringValue([]byte(`"alpha"`)); !ok || got != "alpha" {
		t.Fatalf("decodeStringValue(valid) = (%q, %v), want (%q, true)", got, ok, "alpha")
	}
	if _, ok := decodeStringValue([]byte(`123`)); ok {
		t.Fatal("decodeStringValue(non-string) ok = true, want false")
	}
}

func TestLoadConfigArchivesRetiredSkillAcquisition(t *testing.T) {
	// Invariant: retirement cannot publish over a file or parent replaced during validation.
	// Owner: persisted config migration; canonical suite: TestLoadConfigArchivesRetiredSkillAcquisition.
	t.Run("Should reject a save made after the migration temporary file is synced", func(t *testing.T) {
		t.Parallel()
		root := t.TempDir()
		path := filepath.Join(root, "config.toml")
		body := "[skills.marketplace]\nregistry = 'clawhub'\n"
		saved := "[agents.always_on]\nmax_proposals = 9\n"
		writeFile(t, path, body)
		directory, err := fileutil.OpenDirectory(root)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := directory.Close(); err != nil {
				t.Error(err)
			}
		})
		contents, info, err := directory.ReadRegularFile("config.toml")
		if err != nil {
			t.Fatal(err)
		}
		migrated, err := archiveRetiredSkillMarketplace(contents, path)
		if err != nil {
			t.Fatal(err)
		}
		err = publishRetiredConfigArchiveWithHook(
			directory,
			path,
			info,
			contents,
			migrated,
			func() { writeFile(t, path, saved) },
			nil,
		)
		if err == nil || !strings.Contains(err.Error(), "config changed during retirement migration") {
			t.Fatalf("publish error = %v", err)
		}
		after, err := os.ReadFile(path)
		if err != nil || string(after) != saved {
			t.Fatalf("concurrent save lost: %q %v", after, err)
		}
		entries, err := os.ReadDir(root)
		if err != nil || len(entries) != 1 {
			t.Fatalf("temporary migration file leaked: %v %v", entries, err)
		}
	})

	t.Run("Should serialize a config save after the retirement comparison", func(t *testing.T) {
		// Not parallel: this case proves ownership of the process-wide config write mutex.
		home, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, home.ConfigFile, "[skills.marketplace]\nregistry = 'clawhub'\n")
		directory, err := fileutil.OpenDirectory(home.HomeDir)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := directory.Close(); err != nil {
				t.Error(err)
			}
		})
		contents, info, err := directory.ReadRegularFile("config.toml")
		if err != nil {
			t.Fatal(err)
		}
		rendered, err := archiveRetiredSkillMarketplace(contents, home.ConfigFile)
		if err != nil {
			t.Fatal(err)
		}
		compared, release := make(chan struct{}), make(chan struct{})
		published := make(chan error, 1)
		go func() {
			published <- publishRetiredConfigArchiveWithHook(directory, home.ConfigFile, info, contents, rendered, nil, func() { close(compared); <-release })
		}()
		<-compared
		unlocked := configOverlayWriteMu.TryLock()
		if unlocked {
			configOverlayWriteMu.Unlock()
		}
		target, err := ResolveConfigWriteTarget(home, "", WriteScopeUser, "")
		if err != nil {
			close(release)
			<-published
			t.Fatal(err)
		}
		saved := make(chan error, 1)
		go func() {
			_, err := EditConfigOverlay(home, "", target, func(editor *OverlayEditor) error {
				return editor.SetValue([]string{"agents", "always_on", "max_proposals"}, int64(9))
			})
			saved <- err
		}()
		close(release)
		if err := <-published; err != nil {
			t.Fatal(err)
		}
		if err := <-saved; err != nil {
			t.Fatal(err)
		}
		if unlocked {
			t.Fatal("migration did not hold the config write lease after comparison")
		}
		cfg, err := LoadForHome(home)
		if err != nil || cfg.Agents.AlwaysOn.MaxProposals != 9 {
			t.Fatalf("saved value lost: %+v %v", cfg.Agents.AlwaysOn, err)
		}
	})

	for _, replacement := range []string{"file", "parent"} {
		t.Run("Should reject a changed "+replacement+" before retirement commits", func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			parent := filepath.Join(root, "config")
			path := filepath.Join(parent, "config.toml")
			const retired = "[skills.marketplace]\nregistry = 'clawhub'\n"
			const current = "[skills]\nenabled = false\n"
			writeFile(t, path, retired)
			_, err := loadPersistedConfigOverlay(path, func(content []byte, source string) (configOverlay, error) {
				overlay, decodeErr := loadConfigOverlayBytes(content, source)
				if decodeErr != nil {
					return overlay, decodeErr
				}
				if replacement == "parent" {
					if err := os.Rename(parent, filepath.Join(root, "original")); err != nil {
						t.Fatal(err)
					}
				}
				writeFile(t, path, current)
				return overlay, nil
			})
			if err == nil {
				t.Fatal("retirement overwrote a concurrently replaced config")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != current {
				t.Fatalf("replacement changed: %q, %v", got, err)
			}
			if replacement == "parent" {
				got, err = os.ReadFile(filepath.Join(root, "original", "config.toml"))
				if err != nil || string(got) != retired {
					t.Fatalf("held original changed: %q, %v", got, err)
				}
			}
		})
	}

	for _, tc := range []struct {
		name    string
		content string
	}{
		{name: "Should archive table keys", content: "[skills]\nenabled = false\nallowed_marketplace_hooks = ['kept']\n[skills.marketplace]\nregistry = 'clawhub'\nbase_url = 'https://registry.example/api'\n"},
		{name: "Should archive dotted keys", content: "skills.enabled = false\nskills.allowed_marketplace_hooks = ['kept']\nskills.marketplace.registry = 'clawhub'\nskills.marketplace.base_url = 'https://registry.example/api'\n"},
		{name: "Should archive an inline marketplace table", content: "[skills]\nenabled = false\nallowed_marketplace_hooks = ['kept']\nmarketplace = {registry = 'clawhub', base_url = 'https://registry.example/api'}\n"},
		{name: "Should preserve current values in an inline skills table", content: "skills = {enabled = false, allowed_marketplace_hooks = ['kept'], marketplace = {registry = 'clawhub', base_url = 'https://registry.example/api'}}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			home, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
			if err != nil {
				t.Fatal(err)
			}
			preserved := "\n# Keep extension trust exactly as written.\n[extensions.trust]\nallow_unverified = true # local choice\n"
			writeFile(t, home.ConfigFile, tc.content+preserved)
			cfg, err := LoadForHome(home)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Skills.Enabled || !reflect.DeepEqual(cfg.Skills.AllowedMarketplaceHooks, []string{"kept"}) ||
				!cfg.Extensions.Trust.AllowUnverified {
				t.Fatalf("current settings changed: skills=%#v extensions=%#v", cfg.Skills, cfg.Extensions)
			}
			first, err := os.ReadFile(home.ConfigFile)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Contains(first, []byte(preserved)) {
				t.Fatalf("unrelated block changed: %s", first)
			}
			active, archived := map[string]any{}, map[string]any{}
			if _, err := burnttoml.Decode(string(first), &active); err != nil {
				t.Fatal(err)
			}
			if _, exists := active["skills"].(map[string]any)["marketplace"]; exists {
				t.Fatalf("retired settings remain active: %s", first)
			}
			var archive strings.Builder
			for line := range strings.SplitSeq(string(first), "\n") {
				if value, ok := strings.CutPrefix(line, "# "); ok {
					archive.WriteString(value + "\n")
				}
			}
			// Decode only the archived TOML after its explanatory comment.
			_, archivedTOML, found := strings.Cut(archive.String(), "these values are inactive.\n")
			if !found {
				t.Fatalf("missing archive: %s", first)
			}
			if _, err := burnttoml.Decode(archivedTOML, &archived); err != nil {
				t.Fatal(err)
			}
			want := map[string]any{"registry": "clawhub", "base_url": "https://registry.example/api"}
			if !reflect.DeepEqual(archived["skills"].(map[string]any)["marketplace"], want) {
				t.Fatalf("archive = %#v", archived)
			}
			if _, err := LoadForHome(home); err != nil {
				t.Fatal(err)
			}
			second, err := os.ReadFile(home.ConfigFile)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(first, second) {
				t.Fatal("second load changed the archive")
			}
			info, err := os.Stat(home.ConfigFile)
			if err != nil {
				t.Fatal(err)
			}
			// Windows exposes writable/read-only mode bits; access control is verified by the ACL suite.
			if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
				t.Fatalf("permissions = %o", info.Mode().Perm())
			}
		})
	}
	t.Run("Should leave invalid config untouched", func(t *testing.T) {
		t.Parallel()
		for _, content := range []string{
			"[skills.marketplace]\nregistry = 'clawhub'\nunknown = true\n",
			"[skills.marketplace]\nregistry = 'clawhub'\n[unknown]\nvalue = true\n",
			"skills = {marketplace = {registry = 'clawhub'}, unknown = true}\n",
			"[skills]\nallowed_marketplace_mcp = true\n",
			"[skills]\nallowed_marketplace_mcp = [1, 2]\n",
		} {
			path := filepath.Join(t.TempDir(), "config.toml")
			writeFile(t, path, content)
			if _, err := loadConfigOverlayFile(path); err == nil {
				t.Fatal("expected invalid config rejection")
			}
			actual, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if string(actual) != content {
				t.Fatalf("invalid config changed: %s", actual)
			}
		}
	})
	t.Run("Should archive retired keys together with a current config write", func(t *testing.T) {
		t.Parallel()
		home, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, home.ConfigFile, "[skills.marketplace]\nregistry = 'clawhub'\n")
		target, err := ResolveConfigWriteTarget(home, "", WriteScopeUser, "")
		if err != nil {
			t.Fatal(err)
		}
		cfg, err := EditConfigOverlay(home, "", target, func(editor *OverlayEditor) error {
			return editor.SetValue([]string{"skills", "poll_interval"}, "2m")
		})
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Skills.PollInterval != 2*time.Minute {
			t.Fatalf("poll interval = %v", cfg.Skills.PollInterval)
		}
		actual, err := os.ReadFile(home.ConfigFile)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(actual, []byte("Archived retired skill acquisition")) {
			t.Fatal("archive missing after write")
		}
		reopened, err := LoadForHome(home)
		if err != nil {
			t.Fatal(err)
		}
		if reopened.Skills.PollInterval != cfg.Skills.PollInterval {
			t.Fatal("written config did not survive reopen")
		}
	})
	t.Run("Should reject new writes of retired keys", func(t *testing.T) {
		t.Parallel()
		home, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatal(err)
		}
		original := "[skills]\nenabled = false\n"
		writeFile(t, home.ConfigFile, original)
		target, err := ResolveConfigWriteTarget(home, "", WriteScopeUser, "")
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"registry", "base_url"} {
			_, err := EditConfigOverlay(home, "", target, func(editor *OverlayEditor) error {
				return editor.SetValue([]string{"skills", "marketplace", key}, "retired")
			})
			if err == nil || !strings.Contains(err.Error(), "unknown config keys") {
				t.Fatalf("new key %s: error = %v", key, err)
			}
		}
		actual, err := os.ReadFile(home.ConfigFile)
		if err != nil {
			t.Fatal(err)
		}
		if string(actual) != original {
			t.Fatalf("rejected write changed file: %s", actual)
		}
	})
	t.Run("Should validate workspace scope before archiving retired config", func(t *testing.T) {
		t.Parallel()
		for _, denied := range []bool{false, true} {
			path := filepath.Join(t.TempDir(), "config.toml")
			content := "[skills.marketplace]\nregistry = 'clawhub'\n"
			if denied {
				content += "[marketplace.catalog]\nttl = '30m'\n"
			}
			writeFile(t, path, content)
			cfg := Config{}
			err := applyWorkspaceConfigOverlayFile(path, &cfg)
			if (err != nil) != denied {
				t.Fatalf("denied=%t error=%v", denied, err)
			}
			actual, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if denied && string(actual) != content {
				t.Fatal("denied workspace was rewritten")
			}
			if !denied && !bytes.Contains(actual, []byte("Archived retired skill acquisition")) {
				t.Fatal("workspace was not archived")
			}
		}
	})
	t.Run("Should archive profile config with profile restrictions intact", func(t *testing.T) {
		t.Parallel()
		for _, denied := range []bool{false, true} {
			path := filepath.Join(t.TempDir(), "config.toml")
			content := "[skills.marketplace]\nregistry = 'clawhub'\n"
			if denied {
				content += "[http]\nport = 8080\n"
			}
			writeFile(t, path, content)
			cfg := Config{}
			err := applyProfileConfigOverlayFile(path, &cfg, RoleFieldSourceProfile)
			if (err != nil) != denied {
				t.Fatalf("denied=%t error=%v", denied, err)
			}
			actual, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if denied && string(actual) != content {
				t.Fatal("denied profile was rewritten")
			}
			if !denied && !bytes.Contains(actual, []byte("Archived retired skill acquisition")) {
				t.Fatal("profile not archived")
			}
		}
	})
}

func TestLoadConfigArchivesRetiredSkillMCP(t *testing.T) {
	t.Parallel()
	for _, content := range []string{
		"[skills]\nenabled = false\nallowed_marketplace_mcp = ['clawhub:@team/kept', 'hash-kept']\nallowed_marketplace_hooks = ['kept']\n",
		"skills.enabled = false\nskills.allowed_marketplace_mcp = ['clawhub:@team/kept', 'hash-kept']\nskills.allowed_marketplace_hooks = ['kept']\n",
		"skills = {enabled = false, allowed_marketplace_mcp = ['clawhub:@team/kept', 'hash-kept'], allowed_marketplace_hooks = ['kept']}\n",
	} {
		t.Run(
			"Should archive only the retired MCP policy and preserve active settings across reload",
			func(t *testing.T) {
				t.Parallel()
				home, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
				if err != nil {
					t.Fatal(err)
				}
				preserved := "\n# Operator extension policy stays unchanged.\n[extensions.trust]\nallow_unverified = true\n"
				writeFile(t, home.ConfigFile, content+preserved)
				cfg, err := LoadForHome(home)
				if err != nil {
					t.Fatal(err)
				}
				if cfg.Skills.Enabled || !reflect.DeepEqual(cfg.Skills.AllowedMarketplaceHooks, []string{"kept"}) ||
					!cfg.Extensions.Trust.AllowUnverified {
					t.Fatalf("active settings changed: %+v", cfg.Skills)
				}
				first, err := os.ReadFile(home.ConfigFile)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Contains(first, []byte(preserved)) {
					t.Fatalf("unrelated config changed: %s", first)
				}
				var archive strings.Builder
				for line := range strings.SplitSeq(string(first), "\n") {
					if value, ok := strings.CutPrefix(line, "# "); ok {
						archive.WriteString(value + "\n")
					}
				}
				_, archivedTOML, found := strings.Cut(archive.String(), "these values are inactive.\n")
				if !found {
					t.Fatal("retired values were not archived")
				}
				var archived struct {
					Skills struct {
						Allowed []string `toml:"allowed_marketplace_mcp"`
					} `toml:"skills"`
				}
				if _, err := burnttoml.Decode(archivedTOML, &archived); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(archived.Skills.Allowed, []string{"clawhub:@team/kept", "hash-kept"}) {
					t.Fatalf("archived policy = %+v", archived)
				}
				if _, err := LoadForHome(home); err != nil {
					t.Fatal(err)
				}
				second, err := os.ReadFile(home.ConfigFile)
				if err != nil {
					t.Fatal(err)
				}
				if !bytes.Equal(first, second) {
					t.Fatal("reload changed archived or unrelated settings")
				}
			},
		)
	}
}

func assertPrivatePathMode(t *testing.T, path string, want os.FileMode) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("os.Stat(%q) error = %v", path, err)
	}
	if got := info.Mode().Perm(); got != want {
		t.Fatalf("permissions for %q = %o, want %o", path, got, want)
	}
}

func TestOverlayEditorKeepsSessionRuntimeDefaultsWithTheirProvider(t *testing.T) {
	t.Parallel()

	const saved = "[defaults]\nagent = \"general\"\nprovider = \"codex\"\n" +
		"model = \"gpt-5.6-terra\"\nreasoning_effort = \"high\"\n"
	for _, testCase := range []struct {
		name           string
		provider       string
		wantModel      bool
		removeProvider bool
	}{
		{name: "Should drop the saved model when the provider is deleted", removeProvider: true},
		{name: "Should drop the saved model when the provider changes", provider: "claude"},
		{name: "Should drop the saved model when the provider clears", provider: ""},
		{name: "Should keep the saved model when the provider is rewritten unchanged", provider: "codex", wantModel: true},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			editor, err := newOverlayEditor(ConfigName, []byte(saved))
			if err != nil {
				t.Fatalf("newOverlayEditor() error = %v", err)
			}
			if testCase.removeProvider {
				if err := editor.Delete(defaultsProviderPath); err != nil {
					t.Fatalf("editor.Delete(defaults.provider) error = %v", err)
				}
			} else if err := editor.SetValue(defaultsProviderPath, testCase.provider); err != nil {
				t.Fatalf("editor.SetValue(defaults.provider) error = %v", err)
			}
			for _, path := range [][]string{defaultsModelPath, defaultsReasoningEffortPath} {
				if got := editor.HasPath(path); got != testCase.wantModel {
					t.Fatalf("HasPath(%v) = %t, want %t", path, got, testCase.wantModel)
				}
			}
		})
	}
}

func TestEditConfigOverlayUnsetAlwaysOnAlias(t *testing.T) {
	t.Parallel()
	for _, path := range [][]string{{"agents", "stays_on"}, {"agents", "stays_on", "max_proposals"}} {
		t.Run("Should reject removed unset "+strings.Join(path, "."), func(t *testing.T) {
			t.Parallel()
			editor := OverlayEditor{}
			if err := editor.Delete(path); err == nil || !strings.Contains(err.Error(), "agents.always_on") {
				t.Fatalf("delete error = %v", err)
			}
		})
	}
	for _, path := range [][]string{{"agents", "always_on"}, {"agents", "always_on", "max_proposals"}} {
		t.Run("Should unset canonical path "+strings.Join(path, "."), func(t *testing.T) {
			t.Parallel()
			home, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
			if err != nil {
				t.Fatal(err)
			}
			target, err := ResolveConfigWriteTarget(home, "", WriteScopeUser, "")
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, home.ConfigFile, "[agents.always_on]\nmax_proposals = 5\n")
			cfg, err := EditConfigOverlay(
				home,
				"",
				target,
				func(editor *OverlayEditor) error { return editor.Delete(path) },
			)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Agents.AlwaysOn.MaxProposals != DefaultWithHome(home).Agents.AlwaysOn.MaxProposals {
				t.Fatalf("effective MaxProposals = %d", cfg.Agents.AlwaysOn.MaxProposals)
			}
			body, err := os.ReadFile(home.ConfigFile)
			if err != nil {
				t.Fatal(err)
			}
			var persisted map[string]any
			meta, err := burnttoml.Decode(string(body), &persisted)
			if err != nil {
				t.Fatal(err)
			}
			if meta.IsDefined(path...) {
				t.Fatalf("deleted path remains in config: %s", body)
			}
		})
	}
}

// Invariant: expired config is rejected without mutating persisted user state.
func TestPersistedRetiredConfigRejected(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ body, key string }{
		{"[agents.stays_on]\nmax_proposals = 7\n", "agents.stays_on"},
		{"[memory]\nenabled = true\n", "memory"},
		{"[roles.memory_extractor]\nenabled = true\n", "roles.memory_extractor"},
		{"[session.compaction]\nenabled = true\n", "session.compaction"},
		{"[permissions]\nprovider_full_access = true\n", "permissions.provider_full_access"},
		{"[[hooks.declarations]]\nname = 'old'\nevent = 'context.pre_compact'\ncommand = 'echo'\n" +
			"[hooks.declarations.matcher]\ncompaction_reason = 'token_limit'\n", "compaction_reason"},
	} {
		t.Run("Should reject expired config without changing bytes "+tc.key, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "config.toml")
			writeFile(t, path, tc.body)
			if _, err := loadConfigOverlayFile(path); err == nil || !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("load error = %v, want rejected key %s", err, tc.key)
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != tc.body {
				t.Fatalf("retired config changed: %q, %v", after, err)
			}
		})
	}
}
