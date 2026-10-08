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
)

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

func TestArchiveRetiredMemorySettings(t *testing.T) {
	t.Parallel()
	t.Run("Should archive memory tables losslessly and preserve live settings", func(t *testing.T) {
		t.Parallel()
		live := "[roles.auto_title]\nenabled = true\n"
		contents := []byte("[memory]\nenabled = false\n[memory.dream]\nmin_hours = 2\n" + live)
		rendered, archived, err := archiveRetiredMemorySettings(contents, "config.toml")
		if err != nil {
			t.Fatal(err)
		}
		active, archivedText, found := strings.Cut(string(rendered), retiredMemoryArchiveHeader)
		if !found || !strings.Contains(active, live) {
			t.Fatalf("live settings or archive missing: %s", rendered)
		}
		if !reflect.DeepEqual(archived, []string{"memory", "memory.dream"}) {
			t.Fatalf("archived = %v", archived)
		}
		var original, removed map[string]any
		if _, err := burnttoml.Decode(string(contents), &original); err != nil {
			t.Fatal(err)
		}
		if _, err := burnttoml.Decode(uncommentMemoryArchive(t, archivedText), &removed); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(original["memory"], removed["memory"]) {
			t.Fatalf("archive lost values: %#v", removed)
		}
		var remaining map[string]any
		if _, err := burnttoml.Decode(active, &remaining); err != nil {
			t.Fatal(err)
		}
		if _, exists := remaining["memory"]; exists {
			t.Fatal("memory table remains active")
		}
		if _, err := loadConfigOverlayBytes([]byte(active), "config.toml"); err != nil {
			t.Fatal(err)
		}
		again, names, err := archiveRetiredMemorySettings(rendered, "config.toml")
		if err != nil || len(names) != 0 || !bytes.Equal(rendered, again) {
			t.Fatalf("second archive changed contents: %v %v", names, err)
		}
	})
	cases := []struct {
		name, retired, live string
		names               []string
	}{
		{
			"Should archive dream role",
			"[roles.dream]\nenabled = true\n",
			"[roles.auto_title]\nenabled = true\n",
			[]string{"roles.dream"},
		},
		{
			"Should archive checkpoint role",
			"[roles.checkpoint_summary]\nenabled = true\n",
			"[roles.auto_title]\nenabled = true\n",
			[]string{"roles.checkpoint_summary"},
		},
		{
			"Should archive extractor role",
			"[roles.memory_extractor]\nenabled = true\n",
			"[roles.auto_title]\nenabled = true\n",
			[]string{"roles.memory_extractor"},
		},
		{
			"Should archive controller role",
			"[roles.memory_controller]\nenabled = true\n",
			"[roles.auto_title]\nenabled = true\n",
			[]string{"roles.memory_controller"},
		},
		{
			"Should archive compaction",
			"[session.compaction]\nenabled = true\nthreshold = 0.85\n",
			"[session.derive]\nmax_replay_bytes = 8192\n",
			[]string{"session.compaction"},
		},
		{
			"Should archive consolidated triggers",
			"[[automation.triggers]]\nname = 'retired'\nevent = 'memory.consolidated'\nworkspace = 'workspace'\n[automation.triggers.matcher]\nlabels = ['old']\n",
			"[[automation.triggers]]\nname = 'kept'\nevent = 'session.stopped'\nworkspace = 'workspace'\n",
			[]string{"automation.triggers"},
		},
		{
			"Should retain hooks after archiving matcher keys",
			"[[hooks.declarations]]\nname = 'audit'\nevent = 'session.compaction'\n[hooks.declarations.matcher]\ncompaction_reason = 'pressure'\ncompaction_strategy = 'summary'\n[hooks.declarations.executor]\ncommand = '/bin/echo'\n",
			"[roles.auto_title]\nenabled = true\n",
			[]string{"hooks.declarations.matcher.compaction_reason", "hooks.declarations.matcher.compaction_strategy"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			contents := []byte(tc.retired + "\n# keep this section\n" + tc.live)
			rendered, archived, err := archiveRetiredMemorySettings(contents, "config.toml")
			if err != nil {
				t.Fatal(err)
			}
			active, archive, found := strings.Cut(string(rendered), retiredMemoryArchiveHeader)
			if !found || !strings.Contains(active, tc.live) || !strings.Contains(active, "# keep this section") {
				t.Fatalf("live settings changed: %s", rendered)
			}
			if !reflect.DeepEqual(archived, tc.names) {
				t.Fatalf("archived = %v, want %v", archived, tc.names)
			}
			if _, err := loadConfigOverlayBytes([]byte(active), "config.toml"); err != nil {
				t.Fatal(err)
			}
			var original, removed map[string]any
			if _, err := burnttoml.Decode(string(contents), &original); err != nil {
				t.Fatal(err)
			}
			if _, err := burnttoml.Decode(uncommentMemoryArchive(t, archive), &removed); err != nil {
				t.Fatal(err)
			}
			_, expected, _ := splitRetiredMemoryValues(original, nil)
			expected = canonicalMemoryArchiveValues(t, expected)
			if !reflect.DeepEqual(expected, removed) {
				t.Fatalf("removed values differ: got %#v want %#v", removed, expected)
			}
			again, names, err := archiveRetiredMemorySettings(rendered, "config.toml")
			if err != nil || len(names) != 0 || !bytes.Equal(rendered, again) {
				t.Fatalf("not idempotent: %v %v", names, err)
			}
		})
	}
	t.Run("Should leave live files byte identical", func(t *testing.T) {
		t.Parallel()
		contents := []byte("# untouched\n[roles.auto_title]\nenabled = true\n")
		rendered, archived, err := archiveRetiredMemorySettings(contents, "config.toml")
		if err != nil || len(archived) != 0 || !bytes.Equal(contents, rendered) {
			t.Fatalf("unexpected migration: %v %v %s", archived, err, rendered)
		}
	})
	t.Run("Should retain the TOML decoder error for invalid input", func(t *testing.T) {
		t.Parallel()
		contents := []byte("[memory\nenabled = false\n")
		var values map[string]any
		_, want := burnttoml.Decode(string(contents), &values)
		_, _, err := archiveRetiredMemorySettings(contents, "config.toml")
		if err == nil || err.Error() != want.Error() {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})
}

func TestArchiveRetiredMemorySettingsTOMLForms(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, contents, preserved string }{
		{
			"Should archive dotted memory leaves",
			"memory.enabled = false\nmemory.dream.min_hours = 2\nroles.auto_title.enabled = true\n",
			"roles.auto_title.enabled = true",
		},
		{
			"Should archive noncontiguous memory tables",
			"[memory]\nenabled = false\n[roles.auto_title]\nenabled = true\n[memory.dream]\nmin_hours = 2\n",
			"[roles.auto_title]\nenabled = true",
		},
		{
			"Should archive an implicit memory parent",
			"[memory.dream]\nmin_hours = 2\n[roles.auto_title]\nenabled = true\n",
			"[roles.auto_title]\nenabled = true",
		},
		{
			"Should preserve adjacent inline roles",
			"roles = { dream = { enabled = true }, auto_title = { enabled = true } }\n",
			"auto_title = { enabled = true }",
		},
		{
			"Should preserve inline role before retired role",
			"roles = { auto_title = { enabled = true }, dream = { enabled = true } }\n",
			"auto_title = { enabled = true }",
		},
		{
			"Should archive consecutive inline roles",
			"roles = { dream = { enabled = true }, memory_extractor = { enabled = true }, auto_title = { enabled = true } }\n",
			"auto_title = { enabled = true }",
		},
		{
			"Should archive nested inline memory",
			"memory = { enabled = false, dream = { min_hours = 2 } }\nroles.auto_title.enabled = true\n",
			"roles.auto_title.enabled = true",
		},
		{
			"Should preserve inline compaction siblings",
			"session = { compaction = { enabled = true }, derive = { max_replay_bytes = 8192 } }\n",
			"derive = { max_replay_bytes = 8192 }",
		},
		{
			"Should preserve inline trigger siblings",
			"automation.triggers = [{name='old',event='memory.consolidated',workspace='workspace'}, {name='kept',event='session.stopped',workspace='workspace'}]\n",
			"{name='kept',event='session.stopped',workspace='workspace'}",
		},
		{
			"Should preserve trigger before retired inline entry",
			"automation.triggers = [{name='kept',event='session.stopped',workspace='workspace'}, {name='old',event='memory.consolidated',workspace='workspace'}]\n",
			"{name='kept',event='session.stopped',workspace='workspace'}",
		},
		{
			"Should preserve live hook matcher",
			"[[hooks.declarations]]\nname='audit'\nevent='session.compaction'\nmatcher = { compaction_reason = 'pressure', compaction_strategy = 'summary', session_id = 'keep' }\nexecutor = { command = '/bin/echo' }\n",
			"session_id = 'keep'",
		},
		{
			"Should archive nested inline hooks",
			"hooks = { declarations = [{ name='audit', event='session.compaction', matcher={compaction_reason='pressure', session_id='keep'}, executor={command='/bin/echo'} }] }\n",
			"session_id='keep'",
		},
		{
			"Should archive every retired inline matcher key",
			"[[hooks.declarations]]\nname='audit'\nevent='session.compaction'\nmatcher = { compaction_reason = 'pressure', compaction_strategy = 'summary' }\nexecutor = { command = '/bin/echo' }\n",
			"executor = { command = '/bin/echo' }",
		},
		{
			"Should archive consecutive retired trigger entries",
			"automation.triggers = [{name='old',event='memory.consolidated'}, {name='also-old',event='memory.consolidated'}, {name='kept',event='session.stopped',workspace='workspace'}]\n",
			"{name='kept',event='session.stopped',workspace='workspace'}",
		},
		{
			"Should archive all inline triggers with a trailing comma",
			"automation.triggers = [{name='old',event='memory.consolidated'},]\n",
			"automation.triggers = [",
		},
		{
			"Should archive spec hook shape",
			"[[hooks]]\nname='audit'\nmatcher = { compaction_reason = 'pressure', session_id = 'keep' }\n",
			"session_id = 'keep'",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			contents := []byte(tc.contents + "\n# untouched trailing section\n[defaults]\nagent = 'general'\n")
			rendered, archived, err := archiveRetiredMemorySettings(contents, "config.toml")
			if err != nil || len(archived) == 0 {
				t.Fatalf("archive error: %v %v", archived, err)
			}
			active, archive, found := strings.Cut(string(rendered), retiredMemoryArchiveHeader)
			if !found || !strings.Contains(active, tc.preserved) ||
				!strings.Contains(active, "# untouched trailing section\n[defaults]\nagent = 'general'\n") {
				t.Fatalf("changed live bytes: %s", rendered)
			}
			var original, kept, removed map[string]any
			if _, err := burnttoml.Decode(string(contents), &original); err != nil {
				t.Fatal(err)
			}
			if _, err := burnttoml.Decode(active, &kept); err != nil {
				t.Fatalf("invalid active TOML: %v\n%s", err, active)
			}
			if _, err := burnttoml.Decode(uncommentMemoryArchive(t, archive), &removed); err != nil {
				t.Fatal(err)
			}
			_, expectedRemoved, _ := splitRetiredMemoryValues(original, nil)
			expectedRemoved = canonicalMemoryArchiveValues(t, expectedRemoved)
			if !reflect.DeepEqual(expectedRemoved, removed) {
				t.Fatalf("archive changed values: got %#v want %#v", removed, expectedRemoved)
			}
			_, _, remaining := splitRetiredMemoryValues(kept, nil)
			if len(remaining) != 0 {
				t.Fatalf("retired active values = %v", remaining)
			}
			again, names, err := archiveRetiredMemorySettings(rendered, "config.toml")
			if err != nil || len(names) != 0 || !bytes.Equal(rendered, again) {
				t.Fatalf("not idempotent: %v %v", names, err)
			}
		})
	}
}

func uncommentMemoryArchive(t *testing.T, text string) string {
	t.Helper()
	var result strings.Builder
	for line := range strings.SplitSeq(strings.TrimSuffix(text, "\n"), "\n") {
		value, ok := strings.CutPrefix(line, "# ")
		if !ok {
			t.Fatalf("archive line is not commented: %q", line)
		}
		result.WriteString(value + "\n")
	}
	return result.String()
}

func canonicalMemoryArchiveValues(t *testing.T, values any) map[string]any {
	t.Helper()
	var encoded bytes.Buffer
	if err := burnttoml.NewEncoder(&encoded).Encode(values); err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if _, err := burnttoml.Decode(encoded.String(), &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

// Invariant: every persisted overlay archives retired keys once, preserves concurrent edits,
// and applies the retained role and replay configuration. Owner: config persistence (UT-004/UT-007).
func TestLoadPersistedConfigArchivesRetiredMemory(t *testing.T) {
	t.Parallel()
	for _, layer := range []string{"global", "profile", "workspace"} {
		t.Run("Should archive and reload the "+layer+" overlay", func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), layer, ConfigName)
			writeFile(
				t,
				path,
				"[memory]\nenabled = true\n[roles.auto_title]\nmodel = 'title-model'\n[roles.coordinator]\nmax_children = 3\n",
			)
			_, err := loadPersistedConfigOverlay(path, loadConfigOverlayBytes)
			if err != nil {
				t.Fatal(err)
			}
			homePaths, err := ResolveHomePathsFrom(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			homePaths.ConfigFile = path
			cfg, err := LoadForHome(homePaths, withoutDotEnv())
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Roles.AutoTitle.Model != "title-model" || cfg.Roles.Coordinator.MaxChildren != 3 {
				t.Fatalf("retained config was not applied: %#v", cfg)
			}
			first, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(first), retiredMemoryArchiveHeader) {
				t.Fatal("retired config was not archived")
			}
			if _, err := loadPersistedConfigOverlay(path, loadConfigOverlayBytes); err != nil {
				t.Fatal(err)
			}
			second, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(first, second) {
				t.Fatal("second load rewrote archived config")
			}
		})
		t.Run("Should preserve a concurrent edit to the "+layer+" overlay", func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), layer, ConfigName)
			writeFile(t, path, "[memory]\nenabled = true\n")
			const edited = "[roles.auto_title]\nmodel = 'edited-model'\n"
			_, err := loadPersistedConfigOverlay(path, func(content []byte, source string) (configOverlay, error) {
				overlay, err := loadConfigOverlayBytes(content, source)
				if err != nil {
					return overlay, err
				}
				writeFile(t, path, edited)
				return overlay, nil
			})
			if err == nil || !strings.Contains(err.Error(), "config changed during retirement migration") {
				t.Fatalf("race error = %v", err)
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != edited {
				t.Fatalf("concurrent edit overwritten: %q, %v", got, err)
			}
		})
	}
	t.Run("Should load retained role and replay settings", func(t *testing.T) {
		t.Parallel()
		homePaths, err := ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		writeFile(
			t,
			homePaths.ConfigFile,
			"[roles.auto_title]\nmodel = 'title-model'\n[roles.coordinator]\nmax_children = 3\n[session.derive]\nmax_replay_bytes = 8192\nmax_message_bytes = 4096\n",
		)
		cfg, err := LoadForHome(homePaths, withoutDotEnv())
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Roles.AutoTitle.Model != "title-model" || cfg.Roles.Coordinator.MaxChildren != 3 ||
			cfg.Session.Derive.MaxReplayBytes != 8192 || cfg.Session.Derive.MaxMessageBytes != 4096 {
			t.Fatalf("retained config was not applied: %#v", cfg)
		}
	})
}
