package config

import (
	"fmt"
	"maps"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	CmdPaletteFallbackAgent = "agent"
	cmdPaletteAliasMaxRunes = 32
)

// CmdPaletteConfig controls command-palette behavior and workspace vocabulary.
type CmdPaletteConfig struct {
	FallbackTargets []string          `toml:"fallback_targets"`
	Personalization bool              `toml:"personalization"`
	Aliases         map[string]string `toml:"aliases,omitempty"`
}

// DefaultCmdPaletteConfig returns the shipped command-palette behavior.
func DefaultCmdPaletteConfig() CmdPaletteConfig {
	return CmdPaletteConfig{
		FallbackTargets: []string{CmdPaletteFallbackAgent},
		Personalization: true,
		Aliases:         map[string]string{},
	}
}

// Validate rejects unsupported fallback targets and malformed aliases.
func (c CmdPaletteConfig) Validate() error {
	if len(c.FallbackTargets) == 0 {
		return ValidationError{
			Path:    "cmd_palette.fallback_targets",
			Message: "must contain at least one of agent",
		}
	}
	for index, target := range c.FallbackTargets {
		if strings.TrimSpace(target) != CmdPaletteFallbackAgent {
			return ValidationError{
				Path:    fmt.Sprintf("cmd_palette.fallback_targets[%d]", index),
				Message: "must be one of agent",
			}
		}
	}
	owners := make(map[string]string, len(c.Aliases))
	commandIDs := make([]string, 0, len(c.Aliases))
	for commandID := range c.Aliases {
		commandIDs = append(commandIDs, commandID)
	}
	sort.Strings(commandIDs)
	for _, commandID := range commandIDs {
		alias := c.Aliases[commandID]
		path := fmt.Sprintf("cmd_palette.aliases[%q]", commandID)
		if err := ValidateCmdPaletteAlias(alias); err != nil {
			return ValidationError{Path: path, Message: err.Error()}
		}
		if owner, exists := owners[alias]; exists {
			return ValidationError{
				Path:    path,
				Message: fmt.Sprintf("alias %q is already owned by %q", alias, owner),
			}
		}
		owners[alias] = commandID
	}
	return nil
}

// ValidateCmdPaletteAlias enforces the public alias grammar.
func ValidateCmdPaletteAlias(alias string) error {
	if !utf8.ValidString(alias) {
		return fmt.Errorf("must be valid UTF-8 and contain 1-%d characters with no whitespace", cmdPaletteAliasMaxRunes)
	}
	count := 0
	for _, character := range alias {
		count++
		if unicode.IsSpace(character) {
			return fmt.Errorf("must contain 1-%d characters with no whitespace", cmdPaletteAliasMaxRunes)
		}
	}
	if count < 1 || count > cmdPaletteAliasMaxRunes {
		return fmt.Errorf("must contain 1-%d characters with no whitespace", cmdPaletteAliasMaxRunes)
	}
	return nil
}

func cloneCmdPaletteConfig(source CmdPaletteConfig) CmdPaletteConfig {
	return CmdPaletteConfig{
		FallbackTargets: append([]string(nil), source.FallbackTargets...),
		Personalization: source.Personalization,
		Aliases:         cloneStringMap(source.Aliases),
	}
}

func cloneStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	cloned := make(map[string]string, len(source))
	maps.Copy(cloned, source)
	return cloned
}
