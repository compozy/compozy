package config

import (
	"testing"
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
		if err := editor.SetValueAtTarget(global, enabledPath, false); err != nil {
			t.Fatalf("SetValueAtTarget(false) error = %v", err)
		}
		if err := editor.SetValueAtTarget(global, enabledPath, true); err != nil {
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
		if err := editor.SetValueAtTarget(global, []string{"providers", "claude", "enabled"}, true); err != nil {
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
		if err := editor.SetValueAtTarget(workspace, enabledPath, true); err != nil {
			t.Fatalf("SetValueAtTarget(workspace) error = %v", err)
		}
		if !editor.HasPath(enabledPath) {
			t.Fatal("workspace overlay lost providers.gemini.enabled, want the key written for validation")
		}
	})
}
