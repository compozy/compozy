package config

import (
	"errors"
	"strings"
	"testing"
)

func TestDesktopConfigValidate(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		command string
		wantErr string
	}{
		{name: "Should accept an empty command", command: ""},
		{name: "Should accept a template with every placeholder", command: "code -g {path}:{line}:{col}"},
		{
			name:    "Should accept a quoted executable with spaces",
			command: `'/Applications/My Editor.app/Contents/MacOS/ed' {path} +{line}`,
		},
		{name: "Should accept surrounding whitespace", command: "  zed {path}  "},
		{name: "Should reject a template without path", command: "code -g", wantErr: "must contain {path}"},
		{
			name:    "Should reject an unknown placeholder",
			command: "code {file}",
			wantErr: "unknown placeholder {file}; allowed: {path}, {line}, {col}",
		},
		{name: "Should accept a placeholder outside the executable", command: "code {path}"},
		{
			name:    "Should reject a placeholder as the executable",
			command: "{path}",
			wantErr: "the executable cannot contain a placeholder",
		},
		{
			name:    "Should reject a quoted placeholder as the executable",
			command: `"{path}" --wait`,
			wantErr: "the executable cannot contain a placeholder",
		},
		{name: "Should reject an unbalanced quote", command: `code "{path}`, wantErr: "unbalanced quote"},
		{
			name:    "Should reject a command longer than 1024 bytes",
			command: "ed {path} " + strings.Repeat("x", 1015),
			wantErr: "must be at most 1024 bytes",
		},
		{name: "Should reject control characters", command: "code {path}\x07", wantErr: "control characters"},
		{name: "Should reject invalid UTF-8", command: "code {path} \xff", wantErr: "valid UTF-8"},
		{
			name:    "Should reject an empty quoted executable",
			command: "'' code {path}",
			wantErr: "the executable is empty",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := DesktopConfig{EditorCommand: tc.command}.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("Validate(%q) error = %v, want nil", tc.command, err)
				}
				return
			}
			validationErr, ok := errors.AsType[ValidationError](err)
			if !ok || validationErr.Path != desktopEditorCommandPath ||
				!strings.Contains(validationErr.Message, tc.wantErr) {
				t.Fatalf("Validate(%q) error = %#v, want path %q containing %q",
					tc.command, err, desktopEditorCommandPath, tc.wantErr)
			}
		})
	}

	t.Run("Should reject a Windows batch shim only on Windows", func(t *testing.T) {
		t.Parallel()
		for _, command := range []string{
			`"C:\Users\d\AppData\Local\Programs\Microsoft VS Code\bin\code.cmd" -g {path}`,
			`C:\tools\EDIT.BAT {path}`,
		} {
			err := DesktopConfig{EditorCommand: command}.validateForOS("windows")
			validationErr, ok := errors.AsType[ValidationError](err)
			if !ok || validationErr.Path != desktopEditorCommandPath ||
				validationErr.Message != "use the editor's .exe path, not a .cmd or .bat shim" {
				t.Fatalf("validateForOS(windows, %q) error = %#v, want batch shim error", command, err)
			}
			if err := (DesktopConfig{EditorCommand: command}).validateForOS("darwin"); err != nil {
				t.Fatalf("validateForOS(darwin, %q) error = %v, want nil", command, err)
			}
		}
		exe := `"C:\Program Files\Microsoft VS Code\Code.exe" -g {path}`
		if err := (DesktopConfig{EditorCommand: exe}).validateForOS("windows"); err != nil {
			t.Fatalf("validateForOS(windows, %q) error = %v, want nil", exe, err)
		}
	})

	t.Run("Should count the 1025-byte boundary exactly", func(t *testing.T) {
		t.Parallel()
		atLimit := "ed {path} " + strings.Repeat("x", 1014)
		if err := (DesktopConfig{EditorCommand: atLimit}).Validate(); err != nil {
			t.Fatalf("Validate(1024 bytes) error = %v, want nil", err)
		}
	})
}
