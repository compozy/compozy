package windowmanager

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
)

const (
	shortcutPaletteOpenChord             = "meta+KeyK"
	shortcutPaletteOpenAction            = "palette.open"
	shortcutPaletteViewSessionsAction    = "palette.view.sessions"
	shortcutSessionNewAction             = "session.new"
	shortcutScopeGlobalToggleAction      = "scope.global.toggle"
	shortcutWindowNavBackAction          = "window.nav.back"
	shortcutSessionCycleNextAction       = "session.cycle.next"
	shortcutSessionCyclePreviousAction   = "session.cycle.previous"
	shortcutSessionFocusAttentionAction  = "session.focus.attention"
	shortcutWorkspacePickerAction        = "workspace.picker"
	shortcutWorkspaceCycleNextAction     = "workspace.cycle.next"
	shortcutWorkspaceCyclePreviousAction = "workspace.cycle.previous"
	shortcutSidebarToggleAction          = "sidebar.toggle"
	shortcutWindowFocusLastAction        = "window.focus.last"
	shortcutWindowTileTopAction          = "window.tile.top"
	shortcutWindowTileBottomAction       = "window.tile.bottom"
	shortcutCheatsheetAction             = "shortcuts.cheatsheet"
	shortcutWindowMinimizeAction         = "window.minimize"
	shortcutWindowTabNewAction           = "window.tab.new"
	shortcutWindowTabReopenAction        = "window.tab.reopen"
)

type shortcutRangeFamily struct {
	action string
	max    int
}

var shortcutRangeFamilies = []shortcutRangeFamily{
	{action: string(CommandDesktopSwitch), max: 9},
	{action: "window.tab.jump", max: 8},
}

var shortcutModifierOrder = []string{
	dragModifierMeta,
	dragModifierControl,
	dragModifierAlt,
	dragModifierShift,
}

// BindableIDs is the command-catalog snapshot accepted by keymap mutations.
type BindableIDs map[string]struct{}

// ShortcutDiagnostic reports a stored keymap entry omitted from the effective map.
type ShortcutDiagnostic struct {
	CommandID string `json:"command_id"`
	Message   string `json:"message"`
}

// ShortcutConflictError identifies both owners of one chord.
type ShortcutConflictError struct {
	Chord   string
	Owner   string
	Command string
}

func (e *ShortcutConflictError) Error() string {
	return fmt.Sprintf(
		"shortcut %q conflicts between %q and %q: %v",
		e.Chord,
		e.Owner,
		e.Command,
		ErrInvalidCommand,
	)
}

func (e *ShortcutConflictError) Unwrap() error { return ErrInvalidCommand }

// UnknownShortcutIDError reports an attempted binding outside the catalog.
type UnknownShortcutIDError struct {
	CommandID string
}

func (e *UnknownShortcutIDError) Error() string {
	return fmt.Sprintf("shortcut action %q is unsupported", e.CommandID)
}

func (e *UnknownShortcutIDError) Unwrap() error { return ErrInvalidCommand }

// NewBindableIDs builds an ownership-safe command-id set.
func NewBindableIDs(ids []string) BindableIDs {
	result := make(BindableIDs, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(id)
		if trimmed != "" {
			result[trimmed] = struct{}{}
		}
	}
	return result
}

// DefaultBindableIDs returns the ids carried by the daemon's shipped keymap.
func DefaultBindableIDs() BindableIDs {
	ids := make([]string, 0, len(defaultKeymap))
	for id := range defaultKeymap {
		ids = append(ids, id)
	}
	return NewBindableIDs(ids)
}

// CanonicalShortcutsV2 validates overrides, expands range families, and rejects
// collisions against both other overrides and daemon-owned defaults.
func CanonicalShortcutsV2(
	overrides map[string]ShortcutBinding,
	bindableIDs BindableIDs,
) (map[string]ShortcutBinding, error) {
	canonical, touchedFamilies, err := canonicalStoredShortcutOverrides(overrides)
	if err != nil {
		return nil, err
	}
	if err := validateBindableShortcutIDs(canonical, bindableIDs); err != nil {
		return nil, err
	}
	if _, err := effectiveKeymap(canonical, touchedFamilies); err != nil {
		return nil, err
	}
	return canonical, nil
}

// CanonicalStoredShortcutsV2 validates grammar and stored collisions without a live catalog.
func CanonicalStoredShortcutsV2(
	overrides map[string]ShortcutBinding,
) (map[string]ShortcutBinding, error) {
	canonical, _, err := canonicalStoredShortcutOverrides(overrides)
	if err != nil {
		return nil, err
	}
	if err := validateShortcutConflicts(canonical); err != nil {
		return nil, err
	}
	return canonical, nil
}

// CanonicalShortcutChord validates one manifest or config chord with the daemon grammar.
func CanonicalShortcutChord(chord string) (string, error) {
	return canonicalShortcutChord(chord)
}

// EffectiveKeymap merges override-wins bindings into daemon defaults and validates the whole map.
func EffectiveKeymap(
	overrides map[string]ShortcutBinding,
	bindableIDs BindableIDs,
) (map[string]ShortcutBinding, error) {
	canonical, touchedFamilies, err := canonicalStoredShortcutOverrides(overrides)
	if err != nil {
		return nil, err
	}
	if err := validateBindableShortcutIDs(canonical, bindableIDs); err != nil {
		return nil, err
	}
	return effectiveKeymap(canonical, touchedFamilies)
}

// EffectiveStoredKeymap resolves a trusted stored map without catalog membership checks.
func EffectiveStoredKeymap(
	overrides map[string]ShortcutBinding,
) (map[string]ShortcutBinding, error) {
	canonical, touchedFamilies, err := canonicalStoredShortcutOverrides(overrides)
	if err != nil {
		return nil, err
	}
	return effectiveKeymap(canonical, touchedFamilies)
}

// TolerantEffectiveKeymap drops dead stored ids while preserving live overrides.
func TolerantEffectiveKeymap(
	overrides map[string]ShortcutBinding,
	bindableIDs BindableIDs,
) (map[string]ShortcutBinding, []ShortcutDiagnostic, error) {
	canonical, touchedFamilies, err := canonicalStoredShortcutOverrides(overrides)
	if err != nil {
		return nil, nil, err
	}
	known := make(map[string]ShortcutBinding, len(canonical))
	diagnostics := make([]ShortcutDiagnostic, 0)
	for commandID, binding := range canonical {
		if _, exists := bindableIDs[commandID]; !exists {
			diagnostics = append(diagnostics, ShortcutDiagnostic{
				CommandID: commandID,
				Message:   "stored shortcut references a command that is no longer registered",
			})
			continue
		}
		known[commandID] = binding
	}
	sort.Slice(diagnostics, func(left, right int) bool {
		return diagnostics[left].CommandID < diagnostics[right].CommandID
	})
	effective, err := effectiveKeymap(known, touchedFamilies)
	return effective, diagnostics, err
}

func canonicalStoredShortcutOverrides(
	overrides map[string]ShortcutBinding,
) (map[string]ShortcutBinding, map[string]struct{}, error) {
	if overrides == nil {
		return nil, nil, nil
	}
	actions := make([]string, 0, len(overrides))
	for action := range overrides {
		actions = append(actions, action)
	}
	sort.Strings(actions)

	canonical := make(map[string]ShortcutBinding, len(overrides))
	touchedFamilies := make(map[string]struct{})
	for _, action := range actions {
		family, isFamily := shortcutFamily(action)
		if isFamily && action == family.action {
			touchedFamilies[family.action] = struct{}{}
			if err := canonicalShortcutRangeBinding(canonical, family, overrides[action]); err != nil {
				return nil, nil, fmt.Errorf("shortcut %q: %w", action, err)
			}
			continue
		}
		binding, err := canonicalShortcutBinding(action, overrides[action])
		if err != nil {
			return nil, nil, fmt.Errorf("shortcut %q: %w", action, err)
		}
		canonical[action] = binding
	}
	return canonical, touchedFamilies, nil
}

func validateBindableShortcutIDs(shortcuts map[string]ShortcutBinding, bindableIDs BindableIDs) error {
	for commandID := range shortcuts {
		if _, exists := bindableIDs[commandID]; !exists {
			return &UnknownShortcutIDError{CommandID: commandID}
		}
	}
	return nil
}

func canonicalShortcutBinding(action string, binding ShortcutBinding) (ShortcutBinding, error) {
	if len(binding) == 0 || (len(binding) == 1 && strings.TrimSpace(binding[0]) == "") {
		return ShortcutBinding{}, nil
	}
	canonical := make(ShortcutBinding, 0, len(binding))
	seen := make(map[string]struct{}, len(binding))
	for index, raw := range binding {
		if strings.TrimSpace(raw) == "" {
			return nil, fmt.Errorf(
				"binding member %d is empty; use an empty string or array to disable the action: %w",
				index,
				ErrInvalidCommand,
			)
		}
		if _, _, _, ok := parseShortcutRange(raw); ok {
			return nil, fmt.Errorf(
				"range binding is valid only for desktop.switch and window.tab.jump: %w",
				ErrInvalidCommand,
			)
		}
		chord, err := canonicalShortcutChord(raw)
		if err != nil {
			return nil, err
		}
		if _, duplicate := seen[chord]; duplicate {
			return nil, fmt.Errorf("action %q repeats chord %q: %w", action, chord, ErrInvalidCommand)
		}
		seen[chord] = struct{}{}
		canonical = append(canonical, chord)
	}
	return canonical, nil
}

func canonicalShortcutRangeBinding(
	output map[string]ShortcutBinding,
	family shortcutRangeFamily,
	binding ShortcutBinding,
) error {
	if len(binding) == 0 || (len(binding) == 1 && strings.TrimSpace(binding[0]) == "") {
		return nil
	}
	start, end := 0, 0
	for memberIndex, raw := range binding {
		prefix, memberStart, memberEnd, ok := parseShortcutRange(raw)
		if !ok {
			return fmt.Errorf(
				"range family %q requires bindings such as control+Digit1..%d: %w",
				family.action,
				family.max,
				ErrInvalidCommand,
			)
		}
		if memberStart < 1 || memberEnd > family.max || memberStart > memberEnd {
			return fmt.Errorf(
				"range %d..%d is outside %s.1..%d: %w",
				memberStart,
				memberEnd,
				family.action,
				family.max,
				ErrInvalidCommand,
			)
		}
		if memberIndex == 0 {
			start, end = memberStart, memberEnd
		} else if memberStart != start || memberEnd != end {
			return fmt.Errorf(
				"range alternates must cover the same members; got %d..%d and %d..%d: %w",
				start,
				end,
				memberStart,
				memberEnd,
				ErrInvalidCommand,
			)
		}
		for digit := memberStart; digit <= memberEnd; digit++ {
			action := family.action + "." + strconv.Itoa(digit)
			chord, err := canonicalShortcutChord(prefix + strconv.Itoa(digit))
			if err != nil {
				return err
			}
			if slicesContain(output[action], chord) {
				return fmt.Errorf("action %q repeats chord %q: %w", action, chord, ErrInvalidCommand)
			}
			output[action] = append(output[action], chord)
		}
	}
	return nil
}

func parseShortcutRange(value string) (prefix string, start int, end int, ok bool) {
	trimmed := strings.TrimSpace(value)
	separator := strings.LastIndex(trimmed, "..")
	if separator < 0 {
		return "", 0, 0, false
	}
	left, right := trimmed[:separator], trimmed[separator+2:]
	if len(left) < len("Digit1") || !strings.Contains(left, "Digit") {
		return "", 0, 0, false
	}
	startIndex := len(left) - 1
	if left[startIndex] < '1' || left[startIndex] > '9' || len(right) != 1 || right[0] < '1' || right[0] > '9' {
		return "", 0, 0, false
	}
	prefix = left[:startIndex]
	if !strings.HasSuffix(prefix, "Digit") {
		return "", 0, 0, false
	}
	return prefix, int(left[startIndex] - '0'), int(right[0] - '0'), true
}

func effectiveKeymap(
	canonical map[string]ShortcutBinding,
	touchedFamilies map[string]struct{},
) (map[string]ShortcutBinding, error) {
	effective := DefaultKeymap()
	for familyAction := range touchedFamilies {
		family, _ := shortcutFamily(familyAction)
		for index := 1; index <= family.max; index++ {
			delete(effective, family.action+"."+strconv.Itoa(index))
		}
	}
	for action, binding := range canonical {
		effective[action] = append(ShortcutBinding(nil), binding...)
	}
	if err := validateShortcutConflicts(effective); err != nil {
		return nil, err
	}
	return effective, nil
}

func validateShortcutConflicts(shortcuts map[string]ShortcutBinding) error {
	actions := make([]string, 0, len(shortcuts))
	for action := range shortcuts {
		actions = append(actions, action)
	}
	sort.Strings(actions)
	owners := make(map[string]string)
	for _, action := range actions {
		for _, chord := range shortcuts[action] {
			if owner, exists := owners[chord]; exists {
				return &ShortcutConflictError{Chord: chord, Owner: owner, Command: action}
			}
			owners[chord] = action
		}
	}
	return nil
}

func canonicalShortcutChord(chord string) (string, error) {
	tokens := strings.Split(strings.TrimSpace(chord), "+")
	modifiers := make(map[string]struct{}, len(shortcutModifierOrder))
	code := ""
	for _, raw := range tokens {
		token := strings.TrimSpace(raw)
		if token == "" {
			return "", fmt.Errorf("chord contains an empty token: %w", ErrInvalidCommand)
		}
		modifier := strings.ToLower(token)
		if isShortcutModifier(modifier) {
			if _, exists := modifiers[modifier]; exists {
				return "", fmt.Errorf("chord repeats modifier %q: %w", modifier, ErrInvalidCommand)
			}
			modifiers[modifier] = struct{}{}
			continue
		}
		if !validShortcutCode(token) {
			return "", fmt.Errorf("chord token %q is unsupported: %w", token, ErrInvalidCommand)
		}
		if code != "" {
			return "", fmt.Errorf("chord must contain exactly one KeyboardEvent.code: %w", ErrInvalidCommand)
		}
		code = token
	}
	if len(modifiers) == 0 {
		return "", fmt.Errorf("chord requires at least one modifier: %w", ErrInvalidCommand)
	}
	if code == "" {
		return "", fmt.Errorf("chord requires exactly one KeyboardEvent.code: %w", ErrInvalidCommand)
	}
	canonical := make([]string, 0, len(modifiers)+1)
	for _, modifier := range shortcutModifierOrder {
		if _, exists := modifiers[modifier]; exists {
			canonical = append(canonical, modifier)
		}
	}
	canonical = append(canonical, code)
	return strings.Join(canonical, "+"), nil
}

func isShortcutModifier(token string) bool {
	switch token {
	case dragModifierMeta, dragModifierControl, dragModifierAlt, dragModifierShift:
		return true
	default:
		return false
	}
}

func shortcutFamily(action string) (shortcutRangeFamily, bool) {
	for _, family := range shortcutRangeFamilies {
		if action == family.action || validShortcutRangeMember(action, family) {
			return family, true
		}
	}
	return shortcutRangeFamily{}, false
}

func validShortcutRangeMember(action string, family shortcutRangeFamily) bool {
	prefix := family.action + "."
	suffix := strings.TrimPrefix(action, prefix)
	if len(suffix) != 1 || suffix[0] < '1' || suffix[0] > '9' {
		return false
	}
	return int(suffix[0]-'0') <= family.max
}

func validShortcutCode(code string) bool {
	if len(code) == 4 && strings.HasPrefix(code, "Key") && code[3] >= 'A' && code[3] <= 'Z' {
		return true
	}
	if len(code) == 6 && strings.HasPrefix(code, "Digit") && code[5] >= '0' && code[5] <= '9' {
		return true
	}
	if len(code) == 2 && code[0] == 'F' && code[1] >= '1' && code[1] <= '9' {
		return true
	}
	if len(code) == 3 && strings.HasPrefix(code, "F1") && code[2] >= '0' && code[2] <= '2' {
		return true
	}
	switch code {
	case "ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown",
		"BracketLeft", "BracketRight",
		"Comma", "Period", "Slash", "Semicolon", "Quote", "Backquote", "Minus", "Equal", "Backslash",
		"Enter", "Space", "Tab", "Escape", "Backspace", "Delete", "Home", "End", "PageUp", "PageDown":
		return true
	default:
		return false
	}
}

func slicesContain(values []string, target string) bool {
	return slices.Contains(values, target)
}
