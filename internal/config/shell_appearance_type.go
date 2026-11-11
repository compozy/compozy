package config

import (
	"fmt"
	"slices"
)

const (
	shellAppearanceTextSizePath      = "shell.appearance.text_size"
	shellAppearanceLetterSpacingPath = "shell.appearance.letter_spacing"
)

// ShellTextSize is a named step that scales every shell text size together.
// Empty (unset) and ShellTextSizeDefault both select the built-in scale.
type ShellTextSize string

const (
	ShellTextSizeSmall   ShellTextSize = "small"
	ShellTextSizeDefault ShellTextSize = "default"
	ShellTextSizeLarge   ShellTextSize = "large"
	ShellTextSizeLarger  ShellTextSize = "larger"
)

// ShellTextSizes lists the accepted text size steps, smallest first.
func ShellTextSizes() []ShellTextSize {
	return []ShellTextSize{ShellTextSizeSmall, ShellTextSizeDefault, ShellTextSizeLarge, ShellTextSizeLarger}
}

// Normalized maps the unset value to ShellTextSizeDefault.
func (s ShellTextSize) Normalized() ShellTextSize {
	if s == "" {
		return ShellTextSizeDefault
	}
	return s
}

func (s ShellTextSize) validate() error {
	if s == "" || slices.Contains(ShellTextSizes(), s) {
		return nil
	}
	return shellValidationError(
		shellAppearanceTextSizePath,
		fmt.Sprintf("must be one of %q, %q, %q or %q: %q",
			ShellTextSizeSmall, ShellTextSizeDefault, ShellTextSizeLarge, ShellTextSizeLarger, s),
	)
}

// ShellLetterSpacing is a named step for the interface text tracking.
// Empty (unset) and ShellLetterSpacingDefault both select the built-in tracking.
type ShellLetterSpacing string

const (
	ShellLetterSpacingTight   ShellLetterSpacing = "tight"
	ShellLetterSpacingDefault ShellLetterSpacing = "default"
	ShellLetterSpacingRelaxed ShellLetterSpacing = "relaxed"
	ShellLetterSpacingWide    ShellLetterSpacing = "wide"
)

// ShellLetterSpacings lists the accepted letter spacing steps, tightest first.
func ShellLetterSpacings() []ShellLetterSpacing {
	return []ShellLetterSpacing{
		ShellLetterSpacingTight, ShellLetterSpacingDefault, ShellLetterSpacingRelaxed, ShellLetterSpacingWide,
	}
}

// Normalized maps the unset value to ShellLetterSpacingDefault.
func (s ShellLetterSpacing) Normalized() ShellLetterSpacing {
	if s == "" {
		return ShellLetterSpacingDefault
	}
	return s
}

func (s ShellLetterSpacing) validate() error {
	if s == "" || slices.Contains(ShellLetterSpacings(), s) {
		return nil
	}
	return shellValidationError(
		shellAppearanceLetterSpacingPath,
		fmt.Sprintf("must be one of %q, %q, %q or %q: %q",
			ShellLetterSpacingTight, ShellLetterSpacingDefault, ShellLetterSpacingRelaxed, ShellLetterSpacingWide, s),
	)
}
