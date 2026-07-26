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
)

func TestEditConfigOverlayPreservesCommentsAndUntouchedSections(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	target, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeGlobal)
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
[network]
enabled = true
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
	for _, want := range []string{
		"# defaults block",
		"# keep this comment",
		"# untouched section",
		`provider = "claude"`,
		"[network]",
		"enabled = true",
		`agent = "general"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("config contents missing %q\n%s", want, text)
		}
	}
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
	target, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeGlobal)
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

	editor, err := newOverlayEditor(ConfigName, []byte("[memory]\nenabled = true\nglobal_dir = \"/tmp/memory\"\n"))
	if err != nil {
		t.Fatalf("newOverlayEditor() error = %v", err)
	}
	if err := editor.SetValue([]string{"memory", "enabled"}, false); err != nil {
		t.Fatalf("editor.SetValue() error = %v", err)
	}
	rendered, err := editor.Bytes()
	if err != nil {
		t.Fatalf("editor.Bytes() error = %v", err)
	}
	text := string(rendered)
	for _, want := range []string{
		"[memory]",
		"enabled = false",
		`global_dir = "/tmp/memory"`,
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
	target, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeGlobal)
	if err != nil {
		t.Fatalf("ResolveConfigWriteTarget() error = %v", err)
	}

	writeFile(t, homePaths.ConfigFile, `
[memory]
enabled = true
global_dir = "/tmp/memory"
`)

	cfg, err := EditConfigOverlay(homePaths, "", target, func(editor *OverlayEditor) error {
		return editor.SetValue([]string{"memory", "enabled"}, false)
	})
	if err != nil {
		t.Fatalf("EditConfigOverlay() error = %v", err)
	}
	if got, want := cfg.Memory.Enabled, false; got != want {
		t.Fatalf("EditConfigOverlay() Memory.Enabled = %v, want %v", got, want)
	}

	contents, err := os.ReadFile(homePaths.ConfigFile)
	if err != nil {
		t.Fatalf("ReadFile(config) error = %v", err)
	}
	text = string(contents)
	for _, want := range []string{
		"[memory]",
		"enabled = false",
		`global_dir = "/tmp/memory"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("config contents missing %q\n%s", want, text)
		}
	}
	if strings.Contains(text, "false[memory]") {
		t.Fatalf("config contents corrupted by boolean update\n%s", text)
	}
}

func TestEditConfigOverlayRejectsUnsupportedMutation(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	target, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeGlobal)
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

func TestEditConfigOverlayCreatesNestedSkillsSection(t *testing.T) {
	t.Parallel()

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	target, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeGlobal)
	if err != nil {
		t.Fatalf("ResolveConfigWriteTarget() error = %v", err)
	}

	writeFile(t, homePaths.ConfigFile, `
[daemon]
socket = "/tmp/agh.sock"

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
		{path: []string{"skills", "disabled_skills"}, value: []string{"agh"}},
		{path: []string{"skills", "poll_interval"}, value: "3s"},
		{path: []string{"skills", "marketplace", "registry"}, value: "clawhub"},
		{path: []string{"skills", "marketplace", "base_url"}, value: "https://skills.example"},
	}
	for _, update := range updates {
		if err := editor.SetValue(update.path, update.value); err != nil {
			rendered, _ := editor.Bytes()
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
		`disabled_skills = ["agh"]`,
		`poll_interval = "3s"`,
		"[skills.marketplace]",
		`registry = "clawhub"`,
		`base_url = "https://skills.example"`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("config contents missing %q\n%s", want, text)
		}
	}
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
			scope:    WriteScopeGlobal,
			wantKind: WriteTargetGlobalConfig,
			wantPath: homePaths.ConfigFile,
		},
		{
			name:     "global sidecar",
			scope:    WriteScopeGlobal,
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
				target, err = ResolveMCPSidecarWriteTarget(homePaths, root, tt.scope)
			} else {
				target, err = ResolveConfigWriteTarget(homePaths, root, tt.scope)
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

	homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
	if err != nil {
		t.Fatalf("ResolveHomePathsFrom() error = %v", err)
	}
	target, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeGlobal)
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
}

func TestWriteScopeValidationAndTargetScope(t *testing.T) {
	t.Parallel()

	for _, scope := range []WriteScope{WriteScopeGlobal, WriteScopeWorkspace} {
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

	globalTarget, err := ResolveConfigWriteTarget(homePaths, "", WriteScopeGlobal)
	if err != nil {
		t.Fatalf("ResolveConfigWriteTarget(global) error = %v", err)
	}
	if got, want := globalTarget.Scope(), WriteScopeGlobal; got != want {
		t.Fatalf("globalTarget.Scope() = %q, want %q", got, want)
	}

	workspaceTarget, err := ResolveMCPSidecarWriteTarget(homePaths, workspaceRoot, WriteScopeWorkspace)
	if err != nil {
		t.Fatalf("ResolveMCPSidecarWriteTarget(workspace) error = %v", err)
	}
	if got, want := workspaceTarget.Scope(), WriteScopeWorkspace; got != want {
		t.Fatalf("workspaceTarget.Scope() = %q, want %q", got, want)
	}
}

func TestOverlayEditorSetTableMutations(t *testing.T) {
	t.Parallel()

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
	if err := editor.Delete([]string{"network"}); err != nil {
		t.Fatalf("editor.Delete(network) error = %v", err)
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
