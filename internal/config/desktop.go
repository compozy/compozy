package config

import (
	"fmt"
	"runtime"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	desktopConfigKey             = "desktop"
	desktopEditorCommandPath     = "desktop.editor_command"
	desktopEditorCommandMaxBytes = 1024
	desktopEditorPathPlaceholder = "path"
	desktopEditorAllowedGuidance = "{path}, {line}, {col}"
)

// DesktopConfig holds machine-only preferences consumed by the desktop app.
type DesktopConfig struct {
	// EditorCommand is an argv template with {path}, {line}, {col}; empty uses the OS text editor.
	EditorCommand string `toml:"editor_command"`
}

type desktopOverlay struct {
	EditorCommand *string `toml:"editor_command"`
}

func (o *desktopOverlay) Apply(dst *DesktopConfig) {
	if o.EditorCommand != nil {
		dst.EditorCommand = *o.EditorCommand
	}
}

// Validate rejects editor command templates the desktop opener cannot split and expand safely.
// The daemon runs on the desktop's machine, so its OS decides the Windows-only rules.
func (c DesktopConfig) Validate() error {
	return c.validateForOS(runtime.GOOS)
}

func (c DesktopConfig) validateForOS(goos string) error {
	if !utf8.ValidString(c.EditorCommand) {
		return desktopEditorCommandError("must be valid UTF-8")
	}
	value := strings.TrimSpace(c.EditorCommand)
	if value == "" {
		return nil
	}
	if len(value) > desktopEditorCommandMaxBytes {
		return desktopEditorCommandError(fmt.Sprintf("must be at most %d bytes", desktopEditorCommandMaxBytes))
	}
	if strings.ContainsFunc(value, unicode.IsControl) {
		return desktopEditorCommandError("must not contain control characters")
	}
	executable, ok := desktopEditorExecutable(value)
	if !ok {
		return desktopEditorCommandError("unbalanced quote")
	}
	if err := validateDesktopEditorPlaceholders(value); err != nil {
		return err
	}
	if executable == "" {
		return desktopEditorCommandError("the executable is empty")
	}
	if strings.ContainsRune(executable, '{') {
		return desktopEditorCommandError("the executable cannot contain a placeholder")
	}
	// Node refuses to spawn a batch file without a shell, and the opener never uses one.
	if goos == "windows" {
		lower := strings.ToLower(executable)
		if strings.HasSuffix(lower, ".cmd") || strings.HasSuffix(lower, ".bat") {
			return desktopEditorCommandError("use the editor's .exe path, not a .cmd or .bat shim")
		}
	}
	return nil
}

func validateDesktopEditorPlaceholders(value string) error {
	hasPath := false
	rest := value
	for {
		start := strings.IndexByte(rest, '{')
		if start < 0 {
			break
		}
		end := strings.IndexByte(rest[start:], '}')
		if end < 0 {
			break
		}
		name := rest[start+1 : start+end]
		switch name {
		case desktopEditorPathPlaceholder:
			hasPath = true
		case "line", "col":
		default:
			return desktopEditorCommandError(
				fmt.Sprintf("unknown placeholder {%s}; allowed: %s", name, desktopEditorAllowedGuidance),
			)
		}
		rest = rest[start+end+1:]
	}
	if !hasPath {
		return desktopEditorCommandError("must contain {path}")
	}
	return nil
}

// desktopEditorExecutable mirrors the desktop splitter and returns argv[0]: whitespace separates
// arguments, single and double quotes group (a quote of the other kind inside an open group is
// literal), and there are no escapes. ok is false when a quote is left open.
func desktopEditorExecutable(value string) (executable string, ok bool) {
	var (
		open    rune
		current strings.Builder
		started bool
		done    bool
	)
	for _, char := range value {
		switch {
		case open != 0:
			if char == open {
				open = 0
			} else if !done {
				current.WriteRune(char)
			}
		case char == '\'' || char == '"':
			open = char
			started = true
		case unicode.IsSpace(char):
			if started {
				done = true
			}
		default:
			if !done {
				current.WriteRune(char)
			}
			started = true
		}
	}
	return current.String(), open == 0
}

func desktopEditorCommandError(message string) error {
	return ValidationError{Path: desktopEditorCommandPath, Message: message}
}
