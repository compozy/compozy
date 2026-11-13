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

	"github.com/compozy/compozy/internal/windowmanager"
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
		if got.NewWindowPolicy != WindowNewPolicyTab ||
			got.SmallViewportPolicy != WindowSmallViewportStack ||
			got.HistoryLimit != 50 || got.NavStackLimit != 50 || got.ClosedEntryLimit != 20 {
			t.Fatalf("WindowManager = %#v, want tab/stack/history/nav/closed defaults", got)
		}
		if got.Gaps != (WindowManagerGapsConfig{}) {
			t.Fatalf("WindowManager.Gaps = %#v, want flush zero gaps", got.Gaps)
		}
		if got.GlobalShortcuts[windowmanager.DefaultGlobalSummonCommandID] !=
			windowmanager.DefaultGlobalSummonChord {
			t.Fatalf("GlobalShortcuts = %#v, want summon default", got.GlobalShortcuts)
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
new_window_policy = "floating"
history_limit = 80
nav_stack_limit = 120
closed_entry_limit = 40

[window_manager.gaps]
inner = 12

[window_manager.snap]
repeat_ratios = [0.5, 0.75, 0.25]
`)
		writeFile(t, filepath.Join(workspaceRoot, DirName, ConfigName), `
[window_manager]
history_limit = 25
nav_stack_limit = 75
closed_entry_limit = 15
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
		if windowManager.NewWindowPolicy != WindowNewPolicyTab ||
			windowManager.HistoryLimit != 25 ||
			windowManager.NavStackLimit != 75 || windowManager.ClosedEntryLimit != 15 ||
			windowManager.DesktopTransition != WindowDesktopTransitionCrossfade {
			t.Fatalf("WindowManager = %#v, want merged behavior", windowManager)
		}
		if windowManager.Gaps.Inner != 12 || windowManager.Gaps.Right != 14 ||
			windowManager.Gaps.Left != 0 {
			t.Fatalf("WindowManager.Gaps = %#v, want merged 12/14 with default left 0", windowManager.Gaps)
		}
	})

	t.Run("Should apply only the workspace window manager overlay onto active defaults", func(t *testing.T) {
		t.Parallel()
		base := DefaultWindowManagerConfig()
		base.NewWindowPolicy = WindowNewPolicyFloating
		base.Gaps.Left = 6
		base.HistoryLimit = 80
		base.NavStackLimit = 90
		base.ClosedEntryLimit = 30
		base.Shortcuts = map[string]windowmanager.ShortcutBinding{"window.focus.left": {"alt+KeyH"}}
		base.GlobalShortcuts = map[string]string{
			windowmanager.DefaultGlobalSummonCommandID: windowmanager.DefaultGlobalSummonChord,
		}
		path := filepath.Join(t.TempDir(), ConfigName)
		writeFile(t, path, `
[window_manager]
history_limit = 25
nav_stack_limit = 70
closed_entry_limit = 10

[window_manager.gaps]
right = 14

[window_manager.shortcuts]
"desktop.switch.next" = ["alt+ArrowRight", "alt+KeyL"]
"window.focus.left" = "alt+KeyH"

[window_manager.global_shortcuts]
"palette.summon.global" = "meta+alt+Space"
`)

		got, err := ApplyWindowManagerOverlayFile(path, base)
		if err != nil {
			t.Fatalf("ApplyWindowManagerOverlayFile() error = %v", err)
		}
		if got.NewWindowPolicy != WindowNewPolicyTab || got.HistoryLimit != 25 ||
			got.NavStackLimit != 70 || got.ClosedEntryLimit != 10 {
			t.Fatalf("window manager overlay = %#v, want active policy and workspace history", got)
		}
		if got.Gaps.Right != 14 || got.Gaps.Left != base.Gaps.Left {
			t.Fatalf("window manager gaps = %#v, want workspace right and active left", got.Gaps)
		}
		if len(got.Shortcuts) != 2 ||
			!slices.Equal(
				got.Shortcuts["desktop.switch.next"],
				windowmanager.ShortcutBinding{"alt+ArrowRight", "alt+KeyL"},
			) ||
			!slices.Equal(got.Shortcuts["window.focus.left"], windowmanager.ShortcutBinding{"alt+KeyH"}) {
			t.Fatalf("window manager shortcuts = %#v, want workspace replacement", got.Shortcuts)
		}
		if got.GlobalShortcuts[windowmanager.DefaultGlobalSummonCommandID] != "meta+alt+Space" {
			t.Fatalf("window manager global shortcuts = %#v", got.GlobalShortcuts)
		}

		got.Shortcuts["desktop.switch.next"][0] = "meta+ArrowRight"
		got.GlobalShortcuts[windowmanager.DefaultGlobalSummonCommandID] = "meta+KeyK"
		if base.Shortcuts["window.focus.left"][0] != "alt+KeyH" {
			t.Fatalf("ApplyWindowManagerOverlayFile() aliased active defaults: %#v", base)
		}
		if base.GlobalShortcuts[windowmanager.DefaultGlobalSummonCommandID] !=
			windowmanager.DefaultGlobalSummonChord {
			t.Fatalf("ApplyWindowManagerOverlayFile() aliased global shortcuts: %#v", base)
		}
	})

	t.Run("Should reject an invalid workspace window manager overlay", func(t *testing.T) {
		t.Parallel()
		path := filepath.Join(t.TempDir(), ConfigName)
		writeFile(t, path, "[window_manager]\nhistory_limit = 0\n")

		_, err := ApplyWindowManagerOverlayFile(path, DefaultWindowManagerConfig())
		assertWindowManagerValidationPath(t, err, "window_manager.history_limit")
	})

	t.Run(
		"Should validate navigation and closed-entry ranges with exact paths [UT-140][UT-141][UT-142]",
		func(t *testing.T) {
			t.Parallel()
			cases := []struct {
				name    string
				path    string
				content string
			}{
				{
					name: "Should reject a zero navigation stack limit",
					path: windowManagerNavStackLimitPath, content: "[window_manager]\nnav_stack_limit = 0\n",
				},
				{
					name: "Should reject a navigation stack limit above the maximum",
					path: windowManagerNavStackLimitPath, content: "[window_manager]\nnav_stack_limit = 201\n",
				},
				{
					name: "Should reject a zero closed entry limit",
					path: windowManagerClosedEntryLimitPath, content: "[window_manager]\nclosed_entry_limit = 0\n",
				},
				{
					name: "Should reject a closed entry limit above the maximum",
					path: windowManagerClosedEntryLimitPath, content: "[window_manager]\nclosed_entry_limit = 101\n",
				},
			}
			for _, testCase := range cases {
				t.Run(testCase.name, func(t *testing.T) {
					t.Parallel()
					path := filepath.Join(t.TempDir(), ConfigName)
					writeFile(t, path, testCase.content)
					_, err := ApplyWindowManagerOverlayFile(path, DefaultWindowManagerConfig())
					assertWindowManagerValidationPath(t, err, testCase.path)
				})
			}
		},
	)

	t.Run("Should reject a duplicate shortcut chord", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultWindowManagerConfig()
		cfg.Shortcuts = map[string]windowmanager.ShortcutBinding{
			"desktop.switch.next": {"Shift+Meta+ArrowRight"},
			"window.focus.right":  {"meta+shift+ArrowRight"},
		}

		err := cfg.Validate()
		assertWindowManagerValidationPath(t, err, "window_manager.shortcuts")
	})

	t.Run("Should reject duplicate desktop-global shortcut chords", func(t *testing.T) {
		t.Parallel()
		cfg := DefaultWindowManagerConfig()
		cfg.GlobalShortcuts = map[string]string{
			windowmanager.DefaultGlobalSummonCommandID: "meta+shift+Space",
			"session.new": "shift+meta+Space",
		}

		assertWindowManagerValidationPath(t, cfg.Validate(), "window_manager.global_shortcuts")
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

	validationError, validationErrorMatched := errors.AsType[ValidationError](err)
	if !validationErrorMatched {
		t.Fatalf("Validate() error = %T %v, want ValidationError", err, err)
	}
	if validationError.Path != want || !strings.Contains(err.Error(), want) {
		t.Fatalf("Validate() error = %v path=%q, want %q", err, validationError.Path, want)
	}
}

// UT-056: the config loader owns warn-and-accept and strict unknown-key rejection.
func TestWindowManagerRetiredConfigKeys(t *testing.T) {
	t.Run("Should warn and accept every retired config key", func(t *testing.T) {
		t.Parallel()
		home, err := ResolveHomePathsFrom(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		writeFile(t, home.ConfigFile, `
[window_manager]
new_window_policy = "floating"
raise_on_focus = "invalid formerly boolean"
drag_away_policy = 42
group_move_modifier = false
[window_manager.snap]
edge_band = "invalid formerly integer"
corner_reach = -1
exit_slack = ["invalid"]
repeat_ratios = "invalid formerly array"
[window_manager.bindings]
top_center = false
bottom_center = [42]
`)
		warnings := map[string]int{}
		cfg, err := LoadForHome(home, WithWindowManagerWarning(func(key string, _ bool) { warnings[key]++ }))
		if err != nil {
			t.Fatal(err)
		}
		for _, key := range []string{"new_window_policy", "raise_on_focus", "drag_away_policy", "group_move_modifier", "snap.edge_band", "snap.corner_reach", "snap.exit_slack", "snap.repeat_ratios", "bindings.top_center", "bindings.bottom_center"} {
			if warnings[key] != 1 {
				t.Fatalf("warnings[%s]=%d, want one", key, warnings[key])
			}
		}
		effective := RedactedConfigMap(&cfg)["window_manager"].(map[string]any)
		if effective["new_window_policy"] != "tab" {
			t.Fatalf("policy=%v", effective["new_window_policy"])
		}
		for _, key := range []string{"raise_on_focus", "drag_away_policy", "group_move_modifier", "snap", "bindings"} {
			if _, present := effective[key]; present {
				t.Fatalf("retired key %s in effective config: %v", key, effective)
			}
		}
		writeFile(t, home.ConfigFile, "[window_manager.snap]\nunknown_threshold = 12\n")
		if _, err := LoadForHome(home); err == nil {
			t.Fatal("unknown retired-table key accepted")
		}
	})
}
