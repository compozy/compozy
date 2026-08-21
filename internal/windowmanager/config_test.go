package windowmanager

// Suite: window-manager configuration
// Invariant: effective workspace overrides are isolated copies and every invalid behavioral setting is rejected uniformly.
// Boundary IN: default configuration, workspace overrides, and manager construction.
// Boundary OUT: TOML decoding and transport DTO validation.

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
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
		navStackLimit := 8
		closedEntryLimit := 9
		desktopTransition := DesktopTransitionInstant
		gaps := GapsConfig{Inner: 1, Top: 2, Right: 3, Bottom: 4, Left: 5}
		snap := SnapConfig{EdgeBand: 24, CornerReach: 120, ExitSlack: 12, RepeatRatios: []float64{0.4, 0.6}}
		bindings := BindingsConfig{TopCenter: "reserved", BottomCenter: configValueNone}
		shortcuts := map[string]ShortcutBinding{"layout.balance": {" Shift + Meta + KeyB "}}

		effective, err := effectiveConfig(defaults, WorkspaceConfig{
			NewWindowPolicy: &newWindowPolicy, SmallViewportPolicy: &smallViewportPolicy,
			FocusPolicy: &focusPolicy, FocusWrap: &focusWrap,
			FocusFollowsPointer: &focusFollowsPointer, RaiseOnFocus: &raiseOnFocus,
			DragAwayPolicy: &dragAwayPolicy, GroupMoveModifier: &groupMoveModifier,
			SwapModifier: &swapModifier,
			HistoryLimit: &historyLimit, NavStackLimit: &navStackLimit, ClosedEntryLimit: &closedEntryLimit,
			DesktopTransition: &desktopTransition,
			Gaps:              &gaps, Snap: &snap,
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
			effective.HistoryLimit != 7 || effective.NavStackLimit != 8 || effective.ClosedEntryLimit != 9 ||
			effective.DesktopTransition != DesktopTransitionInstant {
			t.Fatalf("effective behavioral config = %+v", effective)
		}
		if effective.Gaps != gaps || effective.Bindings != bindings || len(effective.Snap.RepeatRatios) != 2 ||
			!slices.Equal(effective.Shortcuts["layout.balance"], ShortcutBinding{"meta+shift+KeyB"}) {
			t.Fatalf("effective structural config = %+v", effective)
		}

		effective.Snap.RepeatRatios[0] = 0.2
		effective.Shortcuts["layout.balance"][0] = "changed"
		if snap.RepeatRatios[0] != 0.4 || shortcuts["layout.balance"][0] != " Shift + Meta + KeyB " {
			t.Fatal("effective config aliases caller-owned collections")
		}
		if defaults.NewWindowPolicy != NewWindowFloating || defaults.HistoryLimit != 50 {
			t.Fatalf("defaults were mutated = %+v", defaults)
		}
	})
}

func TestCanonicalShortcutsV2(t *testing.T) {
	t.Run("Should decode scalar and array bindings from JSON [UT-035,UT-036]", func(t *testing.T) {
		t.Parallel()

		for raw, want := range map[string]ShortcutBinding{
			`"meta+KeyW"`:              {"meta+KeyW"},
			`["meta+KeyQ","alt+KeyQ"]`: {"meta+KeyQ", "alt+KeyQ"},
		} {
			var binding ShortcutBinding
			if err := json.Unmarshal([]byte(raw), &binding); err != nil {
				t.Fatalf("json.Unmarshal(%s) error = %v", raw, err)
			}
			if !slices.Equal(binding, want) {
				t.Fatalf("json.Unmarshal(%s) = %#v, want %#v", raw, binding, want)
			}
		}

		for _, raw := range []string{`42`, `["meta+KeyR",42]`} {
			var binding ShortcutBinding
			if err := json.Unmarshal([]byte(raw), &binding); err == nil {
				t.Fatalf("json.Unmarshal(%s) error = nil", raw)
			}
		}
	})

	t.Run("Should canonicalize scalar and array bindings without aliasing [UT-035,UT-036]", func(t *testing.T) {
		t.Parallel()
		overrides := map[string]ShortcutBinding{
			"window.close": {" Shift + Alt + CONTROL + Meta + KeyQ ", "alt+KeyW"},
		}
		canonical, err := CanonicalShortcutsV2(overrides, DefaultBindableIDs())
		if err != nil {
			t.Fatalf("CanonicalShortcutsV2() error = %v", err)
		}
		want := ShortcutBinding{"meta+control+alt+shift+KeyQ", "alt+KeyW"}
		if !slices.Equal(canonical["window.close"], want) {
			t.Fatalf("canonical binding = %#v, want %#v", canonical["window.close"], want)
		}
		canonical["window.close"][0] = "changed"
		if overrides["window.close"][0] != " Shift + Alt + CONTROL + Meta + KeyQ " {
			t.Fatal("CanonicalShortcutsV2() aliases its input")
		}
	})

	t.Run("Should expand desktop ranges and keep tab jumps capped at eight [UT-037,UT-038]", func(t *testing.T) {
		t.Parallel()
		canonical, err := CanonicalShortcutsV2(map[string]ShortcutBinding{
			"desktop.switch": {"control+Digit1..9"},
		}, DefaultBindableIDs())
		if err != nil {
			t.Fatalf("CanonicalShortcutsV2(desktop range) error = %v", err)
		}
		if len(canonical) != 9 || !slices.Equal(canonical["desktop.switch.9"], ShortcutBinding{"control+Digit9"}) {
			t.Fatalf("expanded desktop range = %#v", canonical)
		}
		for action, binding := range map[string]ShortcutBinding{
			"window.close":    {"control+Digit1..9"},
			"window.tab.jump": {"control+alt+Digit1..9"},
		} {
			if _, rangeErr := CanonicalShortcutsV2(
				map[string]ShortcutBinding{action: binding},
				DefaultBindableIDs(),
			); !errors.Is(
				rangeErr,
				ErrInvalidCommand,
			) {
				t.Fatalf("CanonicalShortcutsV2(%s) error = %v", action, rangeErr)
			}
		}
	})

	t.Run("Should diagnose duplicate chords across and within arrays [UT-039,UT-040]", func(t *testing.T) {
		t.Parallel()
		across := map[string]ShortcutBinding{
			"window.close":    {"alt+KeyQ", "control+KeyQ"},
			"window.minimize": {"control+KeyQ"},
		}
		if _, err := CanonicalShortcutsV2(across, DefaultBindableIDs()); !errors.Is(err, ErrInvalidCommand) ||
			!stringsContainAll(err.Error(), "control+KeyQ", "window.close", "window.minimize") {
			t.Fatalf("cross-action duplicate error = %v", err)
		}
		within := map[string]ShortcutBinding{"window.close": {"alt+KeyQ", "ALT + KeyQ"}}
		if _, err := CanonicalShortcutsV2(within, DefaultBindableIDs()); !errors.Is(err, ErrInvalidCommand) ||
			!stringsContainAll(err.Error(), "window.close", "alt+KeyQ", "repeats") {
			t.Fatalf("same-action duplicate error = %v", err)
		}
	})

	t.Run("Should replace a family with an exact partial range [UT-041]", func(t *testing.T) {
		t.Parallel()
		effective, err := EffectiveKeymap(map[string]ShortcutBinding{
			"desktop.switch": {"meta+control+Digit1..4"},
		}, DefaultBindableIDs())
		if err != nil {
			t.Fatalf("EffectiveKeymap() error = %v", err)
		}
		count := 0
		for action := range effective {
			if family, ok := shortcutFamily(action); ok && family.action == "desktop.switch" {
				count++
			}
		}
		if count != 4 || !slices.Equal(effective["desktop.switch.4"], ShortcutBinding{"meta+control+Digit4"}) {
			t.Fatalf("partial effective range = %#v", effective)
		}
	})

	t.Run("Should name the exact expanded member in a shadowed collision [UT-042,UT-045]", func(t *testing.T) {
		t.Parallel()
		_, err := EffectiveKeymap(map[string]ShortcutBinding{
			"window.close": {"control+Digit3"},
		}, DefaultBindableIDs())
		if !errors.Is(err, ErrInvalidCommand) ||
			!stringsContainAll(err.Error(), "control+Digit3", "desktop.switch.3", "window.close") {
			t.Fatalf("EffectiveKeymap(shadowed member) error = %v", err)
		}
	})

	t.Run("Should own every v2 action in immutable daemon defaults [UT-043]", func(t *testing.T) {
		t.Parallel()
		defaults := DefaultKeymap()
		for _, action := range []string{
			"palette.open", "palette.view.sessions", "session.new", "scope.global.toggle",
			"window.nav.back", "session.cycle.next", "session.cycle.previous",
			"session.focus.attention", "workspace.picker", "workspace.cycle.next",
			"workspace.cycle.previous", "desktop.switch.9", "desktop.create", "sidebar.toggle",
			"window.focus.last", "window.tile.top", "window.tile.bottom", "shortcuts.cheatsheet",
		} {
			if _, ok := defaults[action]; !ok {
				t.Fatalf("DefaultKeymap() missing %q", action)
			}
		}
		defaults["palette.open"][0] = "changed"
		if DefaultKeymap()["palette.open"][0] != shortcutPaletteOpenChord {
			t.Fatal("DefaultKeymap() aliases daemon defaults")
		}
	})

	t.Run("Should reject unknown actions and allow explicit disables [UT-044,UT-046]", func(t *testing.T) {
		t.Parallel()
		if _, err := CanonicalShortcutsV2(map[string]ShortcutBinding{
			"window.teleport": {"meta+KeyT"},
		}, DefaultBindableIDs()); !errors.Is(err, ErrInvalidCommand) {
			t.Fatalf("CanonicalShortcutsV2(unknown) error = %v", err)
		}
		effective, err := EffectiveKeymap(map[string]ShortcutBinding{
			"window.close": {},
			"window.zoom":  {""},
		}, DefaultBindableIDs())
		if err != nil {
			t.Fatalf("EffectiveKeymap(disabled) error = %v", err)
		}
		if len(effective["window.close"]) != 0 || len(effective["window.zoom"]) != 0 {
			t.Fatalf("disabled actions = %#v", effective)
		}
	})

	t.Run("Should accept an extension command supplied by the catalog [UT-070]", func(t *testing.T) {
		t.Parallel()

		bindable := DefaultBindableIDs()
		bindable["ext.notes.capture"] = struct{}{}
		effective, err := EffectiveKeymap(map[string]ShortcutBinding{
			"ext.notes.capture": {"alt+shift+KeyN"},
		}, bindable)
		if err != nil {
			t.Fatalf("EffectiveKeymap() error = %v", err)
		}
		if !slices.Equal(effective["ext.notes.capture"], ShortcutBinding{"alt+shift+KeyN"}) {
			t.Fatalf("extension binding = %#v, want catalog-backed chord", effective["ext.notes.capture"])
		}
	})

	t.Run("Should bind free extension defaults and keep conflicts dormant [UT-075,UT-076]", func(t *testing.T) {
		t.Parallel()
		bindable := DefaultBindableIDs()
		bindable["ext.notes.capture"] = struct{}{}
		bindable["ext.tasks.capture"] = struct{}{}
		effective, statuses, _, err := TolerantEffectiveKeymapWithExtensionDefaults(
			nil,
			bindable,
			[]ExtensionDefaultShortcut{
				{CommandID: "ext.notes.capture", Chord: "alt+shift+KeyN", Source: "ext.notes", Active: true},
				{CommandID: "ext.tasks.capture", Chord: "alt+shift+KeyN", Source: "ext.tasks", Active: true},
			},
		)
		if err != nil {
			t.Fatalf("TolerantEffectiveKeymapWithExtensionDefaults() error = %v", err)
		}
		if !slices.Equal(effective["ext.notes.capture"], ShortcutBinding{"alt+shift+KeyN"}) ||
			len(effective["ext.tasks.capture"]) != 0 || len(statuses) != 2 ||
			statuses[0].Dormant || !statuses[1].Dormant || statuses[1].ConflictWith != "ext.notes.capture" {
			t.Fatalf("extension default state = %#v / %#v", effective, statuses)
		}
	})

	t.Run(
		"Should let core and user bindings win while preserving dormant defaults [UT-076,UT-077]",
		func(t *testing.T) {
			t.Parallel()
			bindable := DefaultBindableIDs()
			bindable["ext.notes.capture"] = struct{}{}
			effective, statuses, _, err := TolerantEffectiveKeymapWithExtensionDefaults(
				map[string]ShortcutBinding{"ext.notes.capture": {"alt+KeyC"}},
				bindable,
				[]ExtensionDefaultShortcut{
					{
						CommandID: "ext.notes.capture", Chord: shortcutPaletteOpenChord,
						Source: "ext.notes", Active: true,
					},
				},
			)
			if err != nil {
				t.Fatalf("TolerantEffectiveKeymapWithExtensionDefaults() error = %v", err)
			}
			if !slices.Equal(effective["ext.notes.capture"], ShortcutBinding{"alt+KeyC"}) ||
				len(statuses) != 1 || !statuses[0].Dormant || statuses[0].ConflictWith != "ext.notes.capture" {
				t.Fatalf("override state = %#v / %#v", effective, statuses)
			}

			withoutExtension := DefaultBindableIDs()
			disabled, diagnostics, err := TolerantEffectiveKeymap(
				map[string]ShortcutBinding{"ext.notes.capture": {"alt+KeyC"}}, withoutExtension,
			)
			if err != nil || len(disabled["ext.notes.capture"]) != 0 || len(diagnostics) != 1 {
				t.Fatalf("disabled extension state = %#v / %#v / %v", disabled, diagnostics, err)
			}

			reenabled, _, _, err := TolerantEffectiveKeymapWithExtensionDefaults(
				map[string]ShortcutBinding{"ext.notes.capture": {"alt+KeyC"}},
				bindable,
				[]ExtensionDefaultShortcut{
					{
						CommandID: "ext.notes.capture", Chord: shortcutPaletteOpenChord,
						Source: "ext.notes", Active: true,
					},
				},
			)
			if err != nil || !slices.Equal(reenabled["ext.notes.capture"], ShortcutBinding{"alt+KeyC"}) {
				t.Fatalf("re-enabled extension state = %#v / %v, want persisted operator override", reenabled, err)
			}
		},
	)

	t.Run("Should drop a conflicting dead stored command with a diagnostic [UT-074]", func(t *testing.T) {
		t.Parallel()

		effective, diagnostics, err := TolerantEffectiveKeymap(map[string]ShortcutBinding{
			"ext.removed.capture": {"meta+KeyN"},
		}, DefaultBindableIDs())
		if err != nil {
			t.Fatalf("TolerantEffectiveKeymap() error = %v", err)
		}
		if _, exists := effective["ext.removed.capture"]; exists {
			t.Fatalf("effective keymap retained dead command: %#v", effective)
		}
		if len(diagnostics) != 1 || diagnostics[0].CommandID != "ext.removed.capture" {
			t.Fatalf("diagnostics = %#v, want dead command", diagnostics)
		}
	})

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

func stringsContainAll(value string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(value, part) {
			return false
		}
	}
	return true
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
			name: "Should reject an empty member mixed with a chord",
			mutate: func(config *Config) {
				config.Shortcuts = map[string]ShortcutBinding{"layout.balance": {"meta+KeyQ", " "}}
			},
			wantError: `shortcut "layout.balance": binding member 1 is empty; use an empty string or array to disable the action: window manager invalid command`,
		},
		{
			name: "Should reject shortcut chord conflicts",
			mutate: func(config *Config) {
				config.Shortcuts = map[string]ShortcutBinding{
					"layout.balance": {"Shift+Meta+KeyB"},
					"layout.undo":    {" meta + shift + KeyB "},
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
		next.Shortcuts = map[string]ShortcutBinding{"layout.undo": {"meta+KeyZ"}}
		if err := manager.UpdateDefaults(next); err != nil {
			t.Fatalf("UpdateDefaults(valid) error = %v", err)
		}
		next.Shortcuts["layout.undo"][0] = "tampered"
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
