package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	burnttoml "github.com/BurntSushi/toml"
)

func TestSetValueAtTargetProviderReEnable(t *testing.T) {
	t.Parallel()

	const original = "[providers.gemini]\ndisplay_name = \"Gemini\"\n\n[defaults]\nprovider = \"codex\"\n"
	global := WriteTarget{kind: WriteTargetGlobalConfig, scope: WriteScopeUser}
	enabledPath := []string{"providers", "gemini", "enabled"}

	t.Run("Should restore the original bytes after disable then re-enable", func(t *testing.T) {
		t.Parallel()
		editor, err := newOverlayEditor(ConfigName, []byte(original))
		if err != nil {
			t.Fatalf("newOverlayEditor() error = %v", err)
		}
		if err := editor.SetValueAtTarget(HomePaths{}, global, enabledPath, false); err != nil {
			t.Fatalf("SetValueAtTarget(false) error = %v", err)
		}
		if err := editor.SetValueAtTarget(HomePaths{}, global, enabledPath, true); err != nil {
			t.Fatalf("SetValueAtTarget(true) error = %v", err)
		}
		if got, _ := editor.Bytes(); string(got) != original {
			t.Fatalf("overlay after round trip = %q, want %q", got, original)
		}
	})

	t.Run("Should remove a provider block that only held enabled", func(t *testing.T) {
		t.Parallel()
		editor, err := newOverlayEditor(ConfigName, []byte("[providers.claude]\nenabled = false\n"))
		if err != nil {
			t.Fatalf("newOverlayEditor() error = %v", err)
		}
		if err := editor.SetValueAtTarget(
			HomePaths{},
			global,
			[]string{"providers", "claude", "enabled"},
			true,
		); err != nil {
			t.Fatalf("SetValueAtTarget(true) error = %v", err)
		}
		if editor.HasPath([]string{"providers", "claude"}) {
			got, _ := editor.Bytes()
			t.Fatalf("overlay = %q, want the emptied block removed", got)
		}
	})

	t.Run("Should keep the plain write in a workspace layer so validation rejects it", func(t *testing.T) {
		t.Parallel()
		editor, err := newOverlayEditor(ConfigName, []byte(original))
		if err != nil {
			t.Fatalf("newOverlayEditor() error = %v", err)
		}
		workspace := WriteTarget{kind: WriteTargetWorkspaceConfig, scope: WriteScopeWorkspace}
		if err := editor.SetValueAtTarget(HomePaths{}, workspace, enabledPath, true); err != nil {
			t.Fatalf("SetValueAtTarget(workspace) error = %v", err)
		}
		if !editor.HasPath(enabledPath) {
			t.Fatal("workspace overlay lost providers.gemini.enabled, want the key written for validation")
		}
	})
	for _, tc := range []struct {
		name   string
		global string
		path   []string
	}{
		{
			name:   "Should keep an explicit profile true over a global disable",
			global: "[providers.gemini]\nenabled = false\n",
			path:   enabledPath,
		},
		{
			name:   "Should keep an explicit profile true over a global disable under a non-canonical name",
			global: "[providers.OPENCODE]\nenabled = false\n",
			path:   []string{"providers", "opencode", "enabled"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			homePaths := HomePaths{ConfigFile: filepath.Join(t.TempDir(), ConfigName)}
			if err := os.WriteFile(homePaths.ConfigFile, []byte(tc.global), 0o600); err != nil {
				t.Fatalf("write global config: %v", err)
			}
			profile := WriteTarget{kind: WriteTargetProfileConfig, scope: WriteScopeProfile}
			editor, err := newOverlayEditor(ConfigName, []byte(""))
			if err != nil {
				t.Fatalf("newOverlayEditor() error = %v", err)
			}
			if err := editor.SetValueAtTarget(homePaths, profile, tc.path, true); err != nil {
				t.Fatalf("SetValueAtTarget(profile, true) error = %v", err)
			}
			got, err := editor.Bytes()
			if err != nil {
				t.Fatalf("Bytes() error = %v", err)
			}
			var decoded map[string]map[string]map[string]any
			if _, err := burnttoml.Decode(string(got), &decoded); err != nil {
				t.Fatalf("decode profile overlay %q: %v", got, err)
			}
			if enabled := decoded["providers"][tc.path[1]]["enabled"]; enabled != true {
				t.Fatalf("profile overlay = %q, want an explicit enabled = true overriding the global disable", got)
			}
		})
	}

	t.Run("Should toggle the provider table under its authored name", func(t *testing.T) {
		t.Parallel()
		const authored = "[providers.OPENCODE]\ndisplay_name = \"Opencode\"\n"
		opencodePath := []string{"providers", "opencode", "enabled"}
		editor, err := newOverlayEditor(ConfigName, []byte(authored))
		if err != nil {
			t.Fatalf("newOverlayEditor() error = %v", err)
		}
		if err := editor.SetValueAtTarget(HomePaths{}, global, opencodePath, false); err != nil {
			t.Fatalf("SetValueAtTarget(false) error = %v", err)
		}
		if !editor.HasPath([]string{"providers", "OPENCODE", "enabled"}) ||
			editor.HasPath([]string{"providers", "opencode"}) {
			got, _ := editor.Bytes()
			t.Fatalf("overlay after disable = %q, want enabled written under [providers.OPENCODE]", got)
		}
		if err := editor.SetValueAtTarget(HomePaths{}, global, opencodePath, true); err != nil {
			t.Fatalf("SetValueAtTarget(true) error = %v", err)
		}
		if got, _ := editor.Bytes(); string(got) != authored {
			t.Fatalf("overlay after re-enable = %q, want %q", got, authored)
		}
	})

	t.Run("Should refuse a re-enable it cannot apply inside an inline table", func(t *testing.T) {
		t.Parallel()
		for _, src := range []string{
			"providers.gemini = { enabled = false }\n",
			"[providers]\ngemini = { enabled = false, display_name = \"Gemini\" }\n",
		} {
			editor, err := newOverlayEditor(ConfigName, []byte(src))
			if err != nil {
				t.Fatalf("newOverlayEditor() error = %v", err)
			}
			err = editor.SetValueAtTarget(HomePaths{}, global, enabledPath, true)
			if !errors.Is(err, ErrUnsupportedTOMLMutation) {
				t.Fatalf("SetValueAtTarget(%q, true) error = %v, want ErrUnsupportedTOMLMutation", src, err)
			}
		}
	})

	t.Run("Should remove a profile key when the global layer leaves the provider enabled", func(t *testing.T) {
		t.Parallel()
		homePaths := HomePaths{ConfigFile: filepath.Join(t.TempDir(), ConfigName)}
		profile := WriteTarget{kind: WriteTargetProfileConfig, scope: WriteScopeProfile}
		editor, err := newOverlayEditor(ConfigName, []byte("[providers.gemini]\nenabled = false\n"))
		if err != nil {
			t.Fatalf("newOverlayEditor() error = %v", err)
		}
		if err := editor.SetValueAtTarget(homePaths, profile, enabledPath, true); err != nil {
			t.Fatalf("SetValueAtTarget(profile, true) error = %v", err)
		}
		if editor.HasPath([]string{"providers", "gemini"}) {
			got, _ := editor.Bytes()
			t.Fatalf("profile overlay = %q, want the key and emptied block removed", got)
		}
	})
}
