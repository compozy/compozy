package windowmanager

import (
	"fmt"
	"sort"
	"strings"
)

const shortcutWindowMinimizeAction = "window.minimize"

var shortcutModifierOrder = []string{
	dragModifierMeta,
	dragModifierControl,
	dragModifierAlt,
	dragModifierShift,
}

// CanonicalShortcuts validates action IDs and returns canonical, conflict-free chords.
func CanonicalShortcuts(shortcuts map[string]string) (map[string]string, error) {
	if shortcuts == nil {
		return nil, nil
	}
	actions := make([]string, 0, len(shortcuts))
	for action := range shortcuts {
		actions = append(actions, action)
	}
	sort.Strings(actions)

	canonical := make(map[string]string, len(shortcuts))
	owners := make(map[string]string, len(shortcuts))
	for _, action := range actions {
		if !validShortcutAction(action) {
			return nil, fmt.Errorf("shortcut action %q is unsupported: %w", action, ErrInvalidCommand)
		}
		chord, err := canonicalShortcutChord(shortcuts[action])
		if err != nil {
			return nil, fmt.Errorf("shortcut %q: %w", action, err)
		}
		if owner, exists := owners[chord]; exists {
			return nil, fmt.Errorf(
				"shortcut %q conflicts between %q and %q: %w",
				chord,
				owner,
				action,
				ErrInvalidCommand,
			)
		}
		owners[chord] = action
		canonical[action] = chord
	}
	return canonical, nil
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

func validShortcutAction(action string) bool {
	switch action {
	case string(CommandWindowClose),
		shortcutWindowMinimizeAction,
		string(CommandWindowZoom),
		string(CommandWindowToggleFloating),
		"window.tile.left",
		"window.tile.right",
		"window.tile.top",
		"window.tile.bottom",
		"window.tile.top-left",
		"window.tile.top-right",
		"window.tile.bottom-left",
		"window.tile.bottom-right",
		"window.focus.left",
		"window.focus.right",
		"window.focus.up",
		"window.focus.down",
		"desktop.switch.previous",
		"desktop.switch.next",
		"desktop.overview",
		"layout.arrange.two-up",
		"layout.arrange.grid",
		string(CommandLayoutBalance),
		string(CommandLayoutUndo),
		string(CommandLayoutRedo):
		return true
	default:
		return false
	}
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
