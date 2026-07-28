package windowmanager

// Suite: window-manager configuration
// Invariant: effective workspace overrides are isolated copies and every invalid behavioral setting is rejected uniformly.
// Boundary IN: default configuration, workspace overrides, and manager construction.
// Boundary OUT: TOML decoding and transport DTO validation.

import (
	"errors"
	"fmt"
	"math"
	"testing"
)

func TestEffectiveConfig(t *testing.T) {
	t.Run("Should apply every workspace override without aliasing caller-owned collections", func(t *testing.T) {
		t.Parallel()
		defaults := DefaultConfig()
		newWindowPolicy := NewWindowInsert
		smallViewportPolicy := SmallViewportReject
		focusPolicy := FocusDirectional
		focusWrap := true
		focusFollowsPointer := true
		raiseOnFocus := false
		dragAwayPolicy := DragAwayGroup
		groupMoveModifier := dragModifierMeta
		swapModifier := dragModifierControl
		historyLimit := 7
		desktopTransition := DesktopTransitionInstant
		gaps := GapsConfig{Inner: 1, Top: 2, Right: 3, Bottom: 4, Left: 5}
		snap := SnapConfig{EdgeBand: 24, CornerReach: 120, ExitSlack: 12, RepeatRatios: []float64{0.4, 0.6}}
		bindings := BindingsConfig{TopCenter: "reserved", BottomCenter: configValueNone}
		shortcuts := map[string]string{"layout.balance": " Shift + Meta + KeyB "}

		effective, err := effectiveConfig(defaults, WorkspaceConfig{
			NewWindowPolicy: &newWindowPolicy, SmallViewportPolicy: &smallViewportPolicy,
			FocusPolicy: &focusPolicy, FocusWrap: &focusWrap,
			FocusFollowsPointer: &focusFollowsPointer, RaiseOnFocus: &raiseOnFocus,
			DragAwayPolicy: &dragAwayPolicy, GroupMoveModifier: &groupMoveModifier,
			SwapModifier: &swapModifier,
			HistoryLimit: &historyLimit, DesktopTransition: &desktopTransition,
			Gaps: &gaps, Snap: &snap,
			Bindings: &bindings, Shortcuts: shortcuts,
		})
		if err != nil {
			t.Fatalf("effectiveConfig() error = %v", err)
		}
		if effective.NewWindowPolicy != NewWindowInsert ||
			effective.SmallViewportPolicy != SmallViewportReject ||
			effective.FocusPolicy != FocusDirectional || !effective.FocusWrap || !effective.FocusFollowsPointer ||
			effective.RaiseOnFocus ||
			effective.DragAwayPolicy != DragAwayGroup ||
			effective.GroupMoveModifier != dragModifierMeta ||
			effective.SwapModifier != dragModifierControl ||
			effective.HistoryLimit != 7 || effective.DesktopTransition != DesktopTransitionInstant {
			t.Fatalf("effective behavioral config = %+v", effective)
		}
		if effective.Gaps != gaps || effective.Bindings != bindings || len(effective.Snap.RepeatRatios) != 2 ||
			effective.Shortcuts["layout.balance"] != "meta+shift+KeyB" {
			t.Fatalf("effective structural config = %+v", effective)
		}

		effective.Snap.RepeatRatios[0] = 0.2
		effective.Shortcuts["layout.balance"] = "changed"
		if snap.RepeatRatios[0] != 0.4 || shortcuts["layout.balance"] != " Shift + Meta + KeyB " {
			t.Fatal("effective config aliases caller-owned collections")
		}
		if defaults.NewWindowPolicy != NewWindowFloating || defaults.HistoryLimit != 50 {
			t.Fatalf("defaults were mutated = %+v", defaults)
		}
	})
}

func TestCanonicalShortcuts(t *testing.T) {
	t.Run("Should accept every action and canonicalize modifier order before conflict checks", func(t *testing.T) {
		t.Parallel()
		actions := []string{
			"window.close", "window.minimize", "window.zoom", "window.toggle_floating",
			"window.tile.left", "window.tile.right", "window.tile.top", "window.tile.bottom",
			"window.tile.top-left", "window.tile.top-right", "window.tile.bottom-left",
			"window.tile.bottom-right", "window.focus.left", "window.focus.right",
			"window.focus.up", "window.focus.down", "desktop.switch.previous",
			"desktop.switch.next", "desktop.overview", "layout.arrange.two-up",
			"layout.arrange.grid", "layout.balance", "layout.undo", "layout.redo",
		}
		shortcuts := make(map[string]string, len(actions))
		for index, action := range actions {
			shortcuts[action] = fmt.Sprintf("meta+Key%c", 'A'+index)
		}
		shortcuts["layout.balance"] = " Shift + Alt + CONTROL + Meta + KeyV "

		canonical, err := CanonicalShortcuts(shortcuts)
		if err != nil {
			t.Fatalf("CanonicalShortcuts() error = %v", err)
		}
		if len(canonical) != len(actions) ||
			canonical["layout.balance"] != "meta+control+alt+shift+KeyV" {
			t.Fatalf("canonical shortcuts = %#v", canonical)
		}
		canonical["layout.balance"] = "changed"
		if shortcuts["layout.balance"] != " Shift + Alt + CONTROL + Meta + KeyV " {
			t.Fatal("CanonicalShortcuts() aliases its input")
		}
	})

	tests := []struct {
		name      string
		shortcuts map[string]string
		wantError string
	}{
		{
			name:      "Should reject an unknown action",
			shortcuts: map[string]string{"window.teleport": "meta+KeyT"},
			wantError: `shortcut action "window.teleport" is unsupported: window manager invalid command`,
		},
		{
			name:      "Should reject an unknown token",
			shortcuts: map[string]string{"window.close": "meta+KeyAA"},
			wantError: `shortcut "window.close": chord token "KeyAA" is unsupported: window manager invalid command`,
		},
		{
			name:      "Should reject a duplicate modifier",
			shortcuts: map[string]string{"window.close": "meta+META+KeyW"},
			wantError: `shortcut "window.close": chord repeats modifier "meta": window manager invalid command`,
		},
		{
			name:      "Should require a modifier",
			shortcuts: map[string]string{"window.close": "KeyW"},
			wantError: `shortcut "window.close": chord requires at least one modifier: window manager invalid command`,
		},
		{
			name:      "Should require exactly one KeyboardEvent code",
			shortcuts: map[string]string{"window.close": "meta+KeyW+KeyQ"},
			wantError: `shortcut "window.close": chord must contain exactly one KeyboardEvent.code: window manager invalid command`,
		},
		{
			name: "Should reject a collision after canonicalization",
			shortcuts: map[string]string{
				"window.close":    "Shift+Meta+KeyW",
				"window.minimize": "meta + shift + KeyW",
			},
			wantError: `shortcut "meta+shift+KeyW" conflicts between "window.close" and "window.minimize": window manager invalid command`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := CanonicalShortcuts(test.shortcuts); !errors.Is(err, ErrInvalidCommand) ||
				err.Error() != test.wantError {
				t.Fatalf("CanonicalShortcuts() error = %v, want %q wrapping ErrInvalidCommand", err, test.wantError)
			}
		})
	}

	t.Run("Should accept exactly the KeyboardEvent code families in the contract", func(t *testing.T) {
		t.Parallel()
		accepted := []string{
			"ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown",
			"BracketLeft", "BracketRight",
			"Comma", "Period", "Slash", "Semicolon", "Quote", "Backquote", "Minus", "Equal", "Backslash",
			"Enter", "Space", "Tab", "Escape", "Backspace", "Delete", "Home", "End", "PageUp", "PageDown",
		}
		for code := 'A'; code <= 'Z'; code++ {
			accepted = append(accepted, fmt.Sprintf("Key%c", code))
		}
		for digit := 0; digit <= 9; digit++ {
			accepted = append(accepted, fmt.Sprintf("Digit%d", digit))
		}
		for function := 1; function <= 12; function++ {
			accepted = append(accepted, fmt.Sprintf("F%d", function))
		}
		for _, code := range accepted {
			if !validShortcutCode(code) {
				t.Fatalf("validShortcutCode(%q) = false", code)
			}
		}
		for _, code := range []string{"Keya", "KeyAA", "Digit10", "F0", "F13", "Right", "Command"} {
			if validShortcutCode(code) {
				t.Fatalf("validShortcutCode(%q) = true", code)
			}
		}
	})
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(*Config)
		wantError string
	}{
		{
			name:      "Should reject an unknown new-window policy",
			mutate:    func(config *Config) { config.NewWindowPolicy = "cascade" },
			wantError: `new window policy "cascade": window manager invalid command`,
		},
		{
			name:      "Should reject an unknown small-viewport policy",
			mutate:    func(config *Config) { config.SmallViewportPolicy = "shrink" },
			wantError: `small viewport policy "shrink": window manager invalid command`,
		},
		{
			name:      "Should reject an unknown focus policy",
			mutate:    func(config *Config) { config.FocusPolicy = "wrap" },
			wantError: `focus policy "wrap": window manager invalid command`,
		},
		{
			name:      "Should reject an unknown drag-away policy",
			mutate:    func(config *Config) { config.DragAwayPolicy = "desktop" },
			wantError: `drag away policy "desktop": window manager invalid command`,
		},
		{
			name:      "Should reject a non-positive history limit",
			mutate:    func(config *Config) { config.HistoryLimit = 0 },
			wantError: "history limit 0 must be between 1 and 500: window manager invalid command",
		},
		{
			name:      "Should reject an excessive history limit",
			mutate:    func(config *Config) { config.HistoryLimit = 501 },
			wantError: "history limit 501 must be between 1 and 500: window manager invalid command",
		},
		{
			name:      "Should reject an unknown desktop transition",
			mutate:    func(config *Config) { config.DesktopTransition = "flip" },
			wantError: `desktop transition "flip": window manager invalid command`,
		},
		{
			name:      "Should reject an unknown group modifier",
			mutate:    func(config *Config) { config.GroupMoveModifier = "command" },
			wantError: `group move modifier "command": window manager invalid command`,
		},
		{
			name:      "Should reject an unknown swap modifier",
			mutate:    func(config *Config) { config.SwapModifier = "command" },
			wantError: `swap modifier "command": window manager invalid command`,
		},
		{
			name:      "Should reject a negative gap",
			mutate:    func(config *Config) { config.Gaps.Left = -1 },
			wantError: "gaps must be finite and non-negative: window manager invalid command",
		},
		{
			name:      "Should reject a non-finite gap",
			mutate:    func(config *Config) { config.Gaps.Inner = math.NaN() },
			wantError: "gaps must be finite and non-negative: window manager invalid command",
		},
		{
			name:      "Should reject invalid snap thresholds",
			mutate:    func(config *Config) { config.Snap.EdgeBand = 0 },
			wantError: "snap thresholds are invalid: window manager invalid command",
		},
		{
			name:      "Should require repeat ratios",
			mutate:    func(config *Config) { config.Snap.RepeatRatios = nil },
			wantError: "repeat ratios are required: window manager invalid command",
		},
		{
			name:      "Should reject an out-of-range repeat ratio",
			mutate:    func(config *Config) { config.Snap.RepeatRatios = []float64{0.05} },
			wantError: "repeat ratio 0.05 is invalid: window manager invalid command",
		},
		{
			name:      "Should reject duplicate repeat ratios",
			mutate:    func(config *Config) { config.Snap.RepeatRatios = []float64{0.5, 0.5} },
			wantError: "repeat ratio 0.5 is duplicated: window manager invalid command",
		},
		{
			name:      "Should reject an unknown edge binding",
			mutate:    func(config *Config) { config.Bindings.TopCenter = "tile" },
			wantError: `binding "tile": window manager invalid command`,
		},
		{
			name:      "Should reject the unsupported stack edge binding",
			mutate:    func(config *Config) { config.Bindings.TopCenter = "stack" },
			wantError: `binding "stack": window manager invalid command`,
		},
		{
			name:      "Should reject an empty shortcut",
			mutate:    func(config *Config) { config.Shortcuts = map[string]string{"layout.balance": " "} },
			wantError: `shortcut "layout.balance": chord contains an empty token: window manager invalid command`,
		},
		{
			name: "Should reject shortcut chord conflicts",
			mutate: func(config *Config) {
				config.Shortcuts = map[string]string{
					"layout.balance": "Shift+Meta+KeyB",
					"layout.undo":    " meta + shift + KeyB ",
				}
			},
			wantError: `shortcut "meta+shift+KeyB" conflicts between "layout.balance" and "layout.undo": window manager invalid command`,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			config := DefaultConfig()
			test.mutate(&config)
			err := validateConfig(config)
			if !errors.Is(err, ErrInvalidCommand) || err.Error() != test.wantError {
				t.Fatalf("validateConfig() error = %v, want %q wrapping ErrInvalidCommand", err, test.wantError)
			}
		})
	}
}

func TestServiceConstruction(t *testing.T) {
	t.Run("Should reject missing dependencies, invalid defaults, and invalid options", func(t *testing.T) {
		t.Parallel()
		repository := NewMemoryRepository()
		resolver := NewMemoryWorkspaceResolver("workspace-a")
		tests := []struct {
			name      string
			construct func() (*Manager, error)
			wantError string
		}{
			{
				name:      "Should reject a nil repository",
				construct: func() (*Manager, error) { return NewService(nil, resolver, nil, DefaultConfig()) },
				wantError: "window manager repository is required",
			},
			{
				name:      "Should reject a nil workspace resolver",
				construct: func() (*Manager, error) { return NewService(repository, nil, nil, DefaultConfig()) },
				wantError: "window manager workspace resolver is required",
			},
			{
				name: "Should reject a nil clock",
				construct: func() (*Manager, error) {
					return NewService(repository, resolver, nil, DefaultConfig(), WithClock(nil))
				},
				wantError: "window manager clock is required",
			},
			{
				name: "Should reject a nil ID generator",
				construct: func() (*Manager, error) {
					return NewService(repository, resolver, nil, DefaultConfig(), WithIDGenerator(nil))
				},
				wantError: "window manager ID generator is required",
			},
			{
				name: "Should reject a non-positive subscription buffer",
				construct: func() (*Manager, error) {
					return NewService(repository, resolver, nil, DefaultConfig(), WithSubscriptionBuffer(0))
				},
				wantError: "window manager subscription buffer must be positive",
			},
			{
				name: "Should reject a nil event observer",
				construct: func() (*Manager, error) {
					return NewService(repository, resolver, nil, DefaultConfig(), WithEventObserver(nil))
				},
				wantError: "window manager event observer is required",
			},
			{
				name: "Should reject a nil workspace config resolver",
				construct: func() (*Manager, error) {
					return NewService(repository, resolver, nil, DefaultConfig(), WithWorkspaceConfigResolver(nil))
				},
				wantError: "window manager workspace config resolver is required",
			},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				t.Parallel()
				_, err := test.construct()
				if err == nil || err.Error() != test.wantError {
					t.Fatalf("NewService() error = %v, want %q", err, test.wantError)
				}
			})
		}
		invalid := DefaultConfig()
		invalid.HistoryLimit = 0
		if _, err := NewService(repository, resolver, nil, invalid); !errors.Is(err, ErrInvalidCommand) {
			t.Fatalf("NewService(invalid defaults) error = %v", err)
		}
	})
}

func TestServiceDefaultConfigLifecycle(t *testing.T) {
	t.Run("Should atomically apply valid defaults and retain known-good defaults after rejection", func(t *testing.T) {
		t.Parallel()
		manager := newTestManagerWithRegistry(t, nil)
		invalid := DefaultConfig()
		invalid.HistoryLimit = 0
		if err := manager.UpdateDefaults(invalid); !errors.Is(err, ErrInvalidCommand) {
			t.Fatalf("UpdateDefaults(invalid) error = %v", err)
		}
		first := executeTestCommand(
			t,
			manager,
			"workspace-a",
			nil,
			CreateDesktopCommand{DesktopID: "desktop-two", Name: "Two"},
		)
		if len(first.Snapshot.History.Undo) != 1 {
			t.Fatalf("history after rejected defaults = %d, want 1", len(first.Snapshot.History.Undo))
		}

		next := DefaultConfig()
		next.HistoryLimit = 1
		next.Shortcuts = map[string]string{"layout.undo": "meta+KeyZ"}
		if err := manager.UpdateDefaults(next); err != nil {
			t.Fatalf("UpdateDefaults(valid) error = %v", err)
		}
		next.Shortcuts["layout.undo"] = "tampered"
		second := executeTestCommand(
			t,
			manager,
			"workspace-a",
			nil,
			CreateDesktopCommand{DesktopID: "desktop-three", Name: "Three"},
		)
		if len(second.Snapshot.History.Undo) != 1 ||
			second.Snapshot.History.Undo[0].CommandID != CommandDesktopCreate {
			t.Fatalf("history after live defaults = %+v, want one create entry", second.Snapshot.History.Undo)
		}
	})
}
