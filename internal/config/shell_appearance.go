package config

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	shellAppearanceUIFontPath   = "shell.appearance.ui_font"
	shellAppearanceMonoFontPath = "shell.appearance.mono_font"

	// ShellFontFamilyMaxLen caps a shell font family name, in runes.
	ShellFontFamilyMaxLen = 128
)

// ShellAppearanceConfig persists operator text choices for every shell client.
// An empty family, size or spacing selects the built-in default.
type ShellAppearanceConfig struct {
	UIFont                string             `toml:"ui_font"`
	MonoFont              string             `toml:"mono_font"`
	TextSize              ShellTextSize      `toml:"text_size"`
	LetterSpacing         ShellLetterSpacing `toml:"letter_spacing"`
	UIFontExplicit        bool               `toml:"-"              json:"-"`
	MonoFontExplicit      bool               `toml:"-"              json:"-"`
	TextSizeExplicit      bool               `toml:"-"              json:"-"`
	LetterSpacingExplicit bool               `toml:"-"              json:"-"`
}

// Validate rejects font family names that are not a single safe family name
// and text sizes or letter spacings outside their named steps.
func (c ShellAppearanceConfig) Validate() error {
	if err := validateShellFontFamily(shellAppearanceUIFontPath, c.UIFont); err != nil {
		return err
	}
	if err := validateShellFontFamily(shellAppearanceMonoFontPath, c.MonoFont); err != nil {
		return err
	}
	if err := c.TextSize.validate(); err != nil {
		return err
	}
	return c.LetterSpacing.validate()
}

func validateShellFontFamily(path string, family string) error {
	if family == "" {
		return nil
	}
	if !utf8.ValidString(family) {
		return shellValidationError(path, "must be valid UTF-8")
	}
	if strings.TrimSpace(family) != family {
		return shellValidationError(path, "must not have leading or trailing whitespace")
	}
	if utf8.RuneCountInString(family) > ShellFontFamilyMaxLen {
		return shellValidationError(path, fmt.Sprintf("must be at most %d characters", ShellFontFamilyMaxLen))
	}
	for _, r := range family {
		if !shellFontFamilyRuneAllowed(r) {
			return shellValidationError(
				path,
				fmt.Sprintf(
					"must be a single font family name using letters, digits, spaces and -_.' only: %q",
					family,
				),
			)
		}
	}
	return nil
}

func shellFontFamilyRuneAllowed(r rune) bool {
	if unicode.IsLetter(r) || unicode.IsDigit(r) {
		return true
	}
	switch r {
	case ' ', '-', '_', '.', '\'':
		return true
	default:
		return false
	}
}
