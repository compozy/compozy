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
	"reflect"
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
			effective.SwapModifier != dragModifierControl ||
			effective.HistoryLimit != 7 || effective.NavStackLimit != 8 || effective.ClosedEntryLimit != 9 ||
			effective.DesktopTransition != DesktopTransitionInstant {
			t.Fatalf("effective behavioral config = %+v", effective)
		}
		if effective.Gaps != gaps ||
			!slices.Equal(effective.Shortcuts["layout.balance"], ShortcutBinding{"meta+shift+KeyB"}) {
			t.Fatalf("effective structural config = %+v", effective)
		}

		effective.Shortcuts["layout.balance"][0] = "changed"
		if snap.RepeatRatios[0] != 0.4 || shortcuts["layout.balance"][0] != " Shift + Meta + KeyB " {
			t.Fatal("effective config aliases caller-owned collections")
		}
		if defaults.NewWindowPolicy != NewWindowTab || defaults.HistoryLimit != 50 {
			t.Fatalf("defaults were mutated = %+v", defaults)
		}
	})
}

func TestCanonicalShortcutsV2(t *testing.T) {
	t.Run("Should decode scalar and array bindings from JSON", func(t *testing.T) {
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

	t.Run("Should canonicalize scalar and array bindings without aliasing", func(t *testing.T) {
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

	t.Run("Should expand desktop ranges and keep tab jumps capped at eight", func(t *testing.T) {
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

	t.Run("Should diagnose duplicate chords across and within arrays", func(t *testing.T) {
		t.Parallel()
		across := map[string]ShortcutBinding{
			"window.close":      {"alt+KeyQ", "control+KeyQ"},
			"window.focus.left": {"control+KeyQ"},
		}
		if _, err := CanonicalShortcutsV2(across, DefaultBindableIDs()); !errors.Is(err, ErrInvalidCommand) ||
			!stringsContainAll(err.Error(), "control+KeyQ", "window.close", "window.focus.left") {
			t.Fatalf("cross-action duplicate error = %v", err)
		}
		within := map[string]ShortcutBinding{"window.close": {"alt+KeyQ", "ALT + KeyQ"}}
		if _, err := CanonicalShortcutsV2(within, DefaultBindableIDs()); !errors.Is(err, ErrInvalidCommand) ||
			!stringsContainAll(err.Error(), "window.close", "alt+KeyQ", "repeats") {
			t.Fatalf("same-action duplicate error = %v", err)
		}
	})

	t.Run("Should replace a family with an exact partial range", func(t *testing.T) {
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

	t.Run("Should name the exact expanded member in a shadowed collision", func(t *testing.T) {
		t.Parallel()
		_, err := EffectiveKeymap(map[string]ShortcutBinding{
			"window.close": {"control+Digit3"},
		}, DefaultBindableIDs())
		if !errors.Is(err, ErrInvalidCommand) ||
			!stringsContainAll(err.Error(), "control+Digit3", "desktop.switch.3", "window.close") {
			t.Fatalf("EffectiveKeymap(shadowed member) error = %v", err)
		}
	})

	t.Run("Should ship swap directions unbound so stored arrow overrides resolve", func(t *testing.T) {
		t.Parallel()
		swaps := []string{"window.swap.left", "window.swap.right", "window.swap.up", "window.swap.down"}
		defaults := DefaultKeymap()
		bindable := DefaultBindableIDs()
		for _, id := range swaps {
			binding, listed := defaults[id]
			if _, ok := bindable[id]; !ok || !listed || len(binding) != 0 {
				t.Fatalf("%s bindable=%v default=%v listed=%v", id, ok, binding, listed)
			}
		}
		// A keymap stored before the swap ids shipped, with another action on an arrow chord.
		stored := map[string]ShortcutBinding{"window.close": {"control+alt+shift+ArrowRight"}}
		effective, err := EffectiveKeymap(stored, bindable)
		if err != nil || !slices.Equal(effective["window.close"], stored["window.close"]) {
			t.Fatalf("stored arrow override effective=%v err=%v", effective["window.close"], err)
		}
		bound, err := EffectiveKeymap(
			map[string]ShortcutBinding{"window.swap.right": {"alt+shift+ArrowRight"}},
			bindable,
		)
		if err != nil || !slices.Equal(bound["window.swap.right"], ShortcutBinding{"alt+shift+ArrowRight"}) {
			t.Fatalf("bound swap effective=%v err=%v", bound["window.swap.right"], err)
		}
	})

	t.Run("Should own every supported action in immutable daemon defaults", func(t *testing.T) {
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

	t.Run("Should reject unknown actions and allow explicit disables", func(t *testing.T) {
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

	t.Run("Should accept an extension command supplied by the catalog", func(t *testing.T) {
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

	t.Run("Should bind free extension defaults and keep conflicts dormant", func(t *testing.T) {
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
		"Should let core and user bindings win while preserving dormant defaults",
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

	t.Run("Should drop a conflicting dead stored command with a diagnostic", func(t *testing.T) {
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
			name:      "Should reject floating in effective config outside boundary translation",
			mutate:    func(config *Config) { config.NewWindowPolicy = "floating" },
			wantError: `new window policy "floating": window manager invalid command`,
		},
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

// UT-056: workspace JSON translation has the same semantics as TOML and stays strict.
func TestRetiredWorkspaceOverrides(t *testing.T) {
	t.Run("Should ignore non-object retired tables and warn once per table", func(t *testing.T) {
		t.Parallel()
		for _, value := range []string{`false`, `42`, `"retired"`, `[]`, `null`} {
			warnings := map[string]int{}
			raw := []byte(`{"snap":` + value + `,"bindings":` + value + `,"focus_wrap":true}`)
			translated, err := translateWorkspaceConfigJSON(raw, func(key string, _ bool) { warnings[key]++ })
			if err != nil || string(translated) != `{"focus_wrap":true}` ||
				warnings["snap"] != 1 || warnings["bindings"] != 1 || len(warnings) != 2 {
				t.Fatalf("value=%s translated=%s warnings=%v error=%v", value, translated, warnings, err)
			}
		}
	})
	t.Run("Should translate retired overrides while rejecting unknown fields", func(t *testing.T) {
		t.Parallel()
		input := []byte(
			`{"new_window_policy":"floating","raise_on_focus":42,"drag_away_policy":false,"group_move_modifier":[],"snap":{"edge_band":false,"corner_reach":-5,"exit_slack":"bad","repeat_ratios":42},"bindings":{"top_center":true,"bottom_center":42}}`,
		)
		overrides, deprecations, err := DecodeWorkspaceConfigJSON(input)
		if err != nil {
			t.Fatal(err)
		}
		if len(deprecations) != 10 {
			t.Fatalf("deprecations=%v", deprecations)
		}
		warnings := map[string]int{}
		for _, entry := range deprecations {
			key := strings.TrimPrefix(entry.Surface, "window_manager.")
			warnings[key]++
			if want := RetiredConfigDeprecation(key, key == "new_window_policy"); entry != want {
				t.Fatalf("deprecation = %+v, want %+v", entry, want)
			}
		}
		for _, key := range []string{
			"new_window_policy", "raise_on_focus", "drag_away_policy", "group_move_modifier", "snap.edge_band",
			"snap.corner_reach", "snap.exit_slack", "snap.repeat_ratios", "bindings.top_center", "bindings.bottom_center",
		} {
			if warnings[key] != 1 {
				t.Fatalf("%s warned %d times", key, warnings[key])
			}
		}
		if want := (WorkspaceConfig{NewWindowPolicy: new(NewWindowTab)}); !reflect.DeepEqual(overrides, want) {
			t.Fatalf("effective override value = %+v, want %+v", overrides, want)
		}
		effective, err := effectiveConfig(DefaultConfig(), overrides)
		if err != nil || effective.NewWindowPolicy != NewWindowTab {
			t.Fatalf("effective=%+v err=%v", effective, err)
		}
		encoded, err := json.Marshal(overrides)
		if err != nil {
			t.Fatal(err)
		}
		if string(encoded) != `{"new_window_policy":"tab"}` {
			t.Fatalf("effective overrides retain retired keys: %s", encoded)
		}
		if _, storedDeprecations, decodeErr := DecodeWorkspaceConfigJSON(
			encoded,
		); decodeErr != nil ||
			len(storedDeprecations) != 0 {
			t.Fatalf("stored override deprecations = %+v, error = %v", storedDeprecations, decodeErr)
		}
		for _, invalid := range []string{`{"snap":{"unknown":true}}`, `{"unknown":true}`, `{"gaps":{"inner":1,"unknown":true}}`} {
			if _, _, err := DecodeWorkspaceConfigJSON([]byte(invalid)); err == nil {
				t.Fatalf("unknown field accepted: %s", invalid)
			}
		}
	})
}

// UT-057: stored retired bindings survive, but never resolve to executable actions.
func TestRetiredShortcutResolution(t *testing.T) {
	t.Run("Should preserve retired bindings without resolving an action", func(t *testing.T) {
		t.Parallel()
		bindings := map[string]ShortcutBinding{
			"window.minimize":       {"alt+KeyM"},
			"window.merge_all":      {"alt+KeyM"},
			"window.tab.detach":     {"alt+KeyM"},
			"window.tile.left":      {"alt+KeyH"},
			"window.unsplit":        {"alt+KeyU"},
			"window.tab.move_left":  {"alt+KeyJ"},
			"window.tab.move_right": {"alt+KeyL"},
		}
		stored, err := CanonicalShortcutsV2(bindings, DefaultBindableIDs())
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(stored["window.minimize"], bindings["window.minimize"]) {
			t.Fatal("retired stored binding lost")
		}
		effective, _, err := TolerantEffectiveKeymap(stored, DefaultBindableIDs())
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range []string{"window.minimize", "window.merge_all", "window.tab.detach"} {
			if _, active := effective[id]; active {
				t.Fatalf("retired shortcut %s resolves to an action", id)
			}
			if len(DefaultKeymap()[id]) != 0 {
				t.Fatalf("retired shortcut %s has a default", id)
			}
		}
		if !slices.Equal(effective["window.tile.left"], ShortcutBinding{"alt+KeyH"}) {
			t.Fatalf("root-edge binding=%v", effective)
		}
		for _, id := range []string{"window.unsplit", "window.tab.move_left", "window.tab.move_right"} {
			if len(DefaultKeymap()[id]) != 0 || len(effective[id]) != 1 {
				t.Fatalf("new binding %s: %v", id, effective[id])
			}
		}
		bindable := DefaultBindableIDs()
		bindable[string(CommandWindowTabReorder)] = struct{}{}
		alias, err := EffectiveKeymap(
			map[string]ShortcutBinding{string(CommandWindowStackReorder): {"alt+KeyR"}},
			bindable,
		)
		if err != nil || !slices.Equal(alias[string(CommandWindowTabReorder)], ShortcutBinding{"alt+KeyR"}) {
			t.Fatalf("alias=%v err=%v", alias, err)
		}
	})
}
