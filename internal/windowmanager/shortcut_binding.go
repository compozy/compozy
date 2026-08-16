package windowmanager

import "fmt"

// ShortcutBinding is one action's ordered shortcut list. An empty binding disables the action.
type ShortcutBinding []string

// UnmarshalTOML accepts the v2 untagged string-or-array shape.
func (b *ShortcutBinding) UnmarshalTOML(value any) error {
	if b == nil {
		return fmt.Errorf("shortcut binding target is nil: %w", ErrInvalidCommand)
	}
	switch typed := value.(type) {
	case string:
		*b = ShortcutBinding{typed}
		return nil
	case []any:
		result := make(ShortcutBinding, len(typed))
		for index, member := range typed {
			chord, ok := member.(string)
			if !ok {
				return fmt.Errorf(
					"shortcut binding member %d must be a string, got %T: %w",
					index,
					member,
					ErrInvalidCommand,
				)
			}
			result[index] = chord
		}
		*b = result
		return nil
	default:
		return fmt.Errorf(
			"shortcut binding must be a string or array of strings, got %T: %w",
			value,
			ErrInvalidCommand,
		)
	}
}

// CloneShortcutMap returns an ownership-safe copy of a shortcut map and its bindings.
func CloneShortcutMap(source map[string]ShortcutBinding) map[string]ShortcutBinding {
	if source == nil {
		return nil
	}
	cloned := make(map[string]ShortcutBinding, len(source))
	for action, binding := range source {
		cloned[action] = append(ShortcutBinding(nil), binding...)
	}
	return cloned
}
