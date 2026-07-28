package config

// Suite: window-manager configuration lifecycle.
// Invariant: valid global/workspace overlays merge predictably and any invalid behavior rejects the whole config.
// Boundary IN: config defaults, TOML overlays, and validation.
// Boundary OUT: runtime command behavior, owned by internal/windowmanager.

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestWindowManagerConfig(t *testing.T) {
	t.Parallel()

	t.Run("Should expose the complete production defaults", func(t *testing.T) {
		t.Parallel()
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}

		got := DefaultWithHome(homePaths).WindowManager
		if err := got.Validate(); err != nil {
			t.Fatalf("WindowManager.Validate() error = %v", err)
		}
		if got.NewWindowPolicy != WindowNewPolicyFloating ||
			got.SmallViewportPolicy != WindowSmallViewportStack ||
			got.HistoryLimit != 50 {
			t.Fatalf("WindowManager = %#v, want floating/stack/history 50", got)
		}
		wantRatios := []float64{0.5, 0.666667, 0.333333}
		if !slices.Equal(got.Snap.RepeatRatios, wantRatios) {
			t.Fatalf("RepeatRatios = %v, want %v", got.Snap.RepeatRatios, wantRatios)
		}
	})

	t.Run("Should merge global and workspace behavior without losing defaults", func(t *testing.T) {
		t.Parallel()
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		workspaceRoot := t.TempDir()
		writeFile(t, homePaths.ConfigFile, `
[window_manager]
new_window_policy = "beside_focus"
history_limit = 80

[window_manager.gaps]
inner = 12

[window_manager.snap]
repeat_ratios = [0.5, 0.75, 0.25]
`)
		writeFile(t, filepath.Join(workspaceRoot, DirName, ConfigName), `
[window_manager]
history_limit = 25
desktop_transition = "crossfade"

[window_manager.gaps]
right = 14

[window_manager.bindings]
top_center = "none"
`)

		got, err := LoadForHome(homePaths, WithWorkspaceRoot(workspaceRoot))
		if err != nil {
			t.Fatalf("LoadForHome() error = %v", err)
		}
		windowManager := got.WindowManager
		if windowManager.NewWindowPolicy != WindowNewPolicyBesideFocus ||
			windowManager.HistoryLimit != 25 ||
			windowManager.DesktopTransition != WindowDesktopTransitionCrossfade {
			t.Fatalf("WindowManager = %#v, want merged behavior", windowManager)
		}
		if windowManager.Gaps.Inner != 12 || windowManager.Gaps.Right != 14 ||
			windowManager.Gaps.Left != 10 {
			t.Fatalf("WindowManager.Gaps = %#v, want merged 12/14 with default left 10", windowManager.Gaps)
		}
		if windowManager.Bindings.TopCenter != "none" || windowManager.Bindings.BottomCenter != "reserved" {
			t.Fatalf("WindowManager.Bindings = %#v, want none/reserved", windowManager.Bindings)
		}
	})

	t.Run("Should apply only the workspace window manager overlay onto active defaults", func(t *testing.T) {
		t.Parallel()
		base := DefaultWindowManagerConfig()
		base.NewWindowPolicy = WindowNewPolicyBesideFocus
		base.HistoryLimit = 80
		base.Snap.RepeatRatios = []float64{0.5, 0.75, 0.25}
		base.Shortcuts = map[string]string{"window.focus.left": "alt+KeyH"}
		path := filepath.Join(t.TempDir(), ConfigName)
		writeFile(t, path, `
[window_manager]
history_limit = 25

[window_manager.gaps]
right = 14

[window_manager.shortcuts]
"desktop.switch.next" = "alt+ArrowRight"
`)

		got, err := ApplyWindowManagerOverlayFile(path, base)
		if err != nil {
			t.Fatalf("ApplyWindowManagerOverlayFile() error = %v", err)
		}
		if got.NewWindowPolicy != WindowNewPolicyBesideFocus || got.HistoryLimit != 25 {
			t.Fatalf("window manager overlay = %#v, want active policy and workspace history", got)
		}
		if got.Gaps.Right != 14 || got.Gaps.Left != base.Gaps.Left {
			t.Fatalf("window manager gaps = %#v, want workspace right and active left", got.Gaps)
		}
		if len(got.Shortcuts) != 1 || got.Shortcuts["desktop.switch.next"] != "alt+ArrowRight" {
			t.Fatalf("window manager shortcuts = %#v, want workspace replacement", got.Shortcuts)
		}

		got.Snap.RepeatRatios[0] = 0.4
		got.Shortcuts["desktop.switch.next"] = "meta+ArrowRight"
		if base.Snap.RepeatRatios[0] != 0.5 || base.Shortcuts["window.focus.left"] != "alt+KeyH" {
			t.Fatalf("ApplyWindowManagerOverlayFile() aliased active defaults: %#v", base)
		}
	})

	t.Run("Should reject an invalid workspace window manager overlay", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), ConfigName)
		writeFile(t, path, "[window_manager]\nhistory_limit = 0\n")

		_, err := ApplyWindowManagerOverlayFile(path, DefaultWindowManagerConfig())
		assertWindowManagerValidationPath(t, err, "window_manager.history_limit")
	})

	t.Run("Should reject a duplicate shortcut chord", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultWindowManagerConfig()
		cfg.Shortcuts = map[string]string{
			"desktop.switch.next": "Shift+Meta+ArrowRight",
			"window.focus.right":  "meta+shift+ArrowRight",
		}

		err := cfg.Validate()
		assertWindowManagerValidationPath(t, err, "window_manager.shortcuts")
	})

	t.Run("Should reject a non-finite or duplicate repeat ratio", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultWindowManagerConfig()
		cfg.Snap.RepeatRatios = []float64{0.5, 0.5}

		err := cfg.Validate()
		assertWindowManagerValidationPath(t, err, "window_manager.snap.repeat_ratios")
	})

	t.Run("Should reject stack without an edge-center target", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultWindowManagerConfig()
		cfg.Bindings.BottomCenter = "stack"

		err := cfg.Validate()
		assertWindowManagerValidationPath(t, err, "window_manager.bindings.bottom_center")
	})

	t.Run("Should reject the removed desktop-state configuration surface", func(t *testing.T) {
		t.Parallel()
		homePaths, err := ResolveHomePathsFrom(filepath.Join(t.TempDir(), "home"))
		if err != nil {
			t.Fatalf("ResolveHomePathsFrom() error = %v", err)
		}
		writeFile(t, homePaths.ConfigFile, `
[desktop_state]
max_value_kib = 256
`)

		_, err = LoadForHome(homePaths)
		if err == nil || !strings.Contains(err.Error(), "desktop_state") {
			t.Fatalf("LoadForHome() error = %v, want removed desktop_state rejection", err)
		}
	})
}

func assertWindowManagerValidationPath(t *testing.T, err error, want string) {
	t.Helper()
	if err == nil {
		t.Fatalf("Validate() error = nil, want path %q", want)
	}
	var validationError ValidationError
	if !errors.As(err, &validationError) {
		t.Fatalf("Validate() error = %T %v, want ValidationError", err, err)
	}
	if validationError.Path != want || !strings.Contains(err.Error(), want) {
		t.Fatalf("Validate() error = %v path=%q, want %q", err, validationError.Path, want)
	}
}
