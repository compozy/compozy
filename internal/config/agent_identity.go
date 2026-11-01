package config

import (
	"bytes"
	_ "embed"
	"encoding/json/v2"
	"errors"
	"maps"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/rivo/uniseg"
)

const (
	AgentKindOnDemand = "on_demand"
	AgentKindStaysOn  = "stays_on"
)

var (
	ErrAgentKindImmutable = errors.New(
		"An agent's kind is fixed at creation. Duplicate it to choose another kind.",
	)
	ErrAgentKindInvalid        = errors.New("Agent kind must be on_demand or stays_on.")
	ErrAgentDisplayNameInvalid = errors.New("Display names can have up to 80 characters.")
	ErrAgentDescriptionInvalid = errors.New("Descriptions can have up to 280 characters.")
	ErrAgentAppearanceInvalid  = errors.New("Appearance needs a `#rrggbb` color and one icon or emoji.")
	agentColorPattern          = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)
)

func (a AgentDef) EffectiveKind() string {
	if a.Kind == "" {
		return AgentKindOnDemand
	}
	return a.Kind
}

func (a AgentDef) Label() string {
	if label := strings.TrimSpace(a.DisplayName); label != "" {
		return label
	}
	words := strings.FieldsFunc(
		a.Name,
		func(r rune) bool { return r == '-' || r == '_' || r == '.' || unicode.IsSpace(r) },
	)
	for i, word := range words {
		rs := []rune(word)
		rs[0] = unicode.ToUpper(rs[0])
		words[i] = string(rs)
	}
	return strings.Join(words, " ")
}

// CloneAgentAppearance preserves unknown versioned appearance fields without sharing mutable data.
func CloneAgentAppearance(value map[string]any) map[string]any {
	result := maps.Clone(value)
	for key, value := range result {
		result[key] = cloneAppearanceValue(value)
	}
	return result
}

func cloneAppearanceValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return CloneAgentAppearance(v)
	case []any:
		result := make([]any, len(v))
		for i := range v {
			result[i] = cloneAppearanceValue(v[i])
		}
		return result
	default:
		return value
	}
}

func (a AgentDef) ResolveAppearance() (map[string]any, error) {
	if a.Appearance != nil {
		if err := ValidateAgentAppearance(a.Appearance); err == nil {
			return CloneAgentAppearance(a.Appearance), nil
		} else {
			return a.defaultAppearance(), err
		}
	}
	return a.defaultAppearance(), nil
}

func (a AgentDef) defaultAppearance() map[string]any {
	return map[string]any{"version": 1, "avatar": "generated", "seed": a.Name, "generation": 1, "color": "#7c5cff"}
}

func (a AgentDef) ValidateIdentity() error {
	if utf8.RuneCountInString(strings.TrimSpace(a.DisplayName)) > 80 {
		return ErrAgentDisplayNameInvalid
	}
	if utf8.RuneCountInString(a.Description) > 280 {
		return ErrAgentDescriptionInvalid
	}
	if a.EffectiveKind() != AgentKindOnDemand && a.EffectiveKind() != AgentKindStaysOn {
		return ErrAgentKindInvalid
	}
	// Appearance is decoded loosely; read projections diagnose invalid or future values.
	return nil
}

func ValidateAgentAppearance(a map[string]any) error {
	if a == nil {
		return nil
	}
	if !appearanceVersionOne(a["version"]) {
		return ErrAgentAppearanceInvalid
	}
	color := agentAppearanceString(a["color"])
	if !agentColorPattern.MatchString(color) {
		return ErrAgentAppearanceInvalid
	}
	icon := agentAppearanceString(a["icon"])
	emoji := agentAppearanceString(a["emoji"])
	if icon != "" && emoji != "" {
		return ErrAgentAppearanceInvalid
	}
	switch a["avatar"] {
	case "generated":
		seed := agentAppearanceString(a["seed"])
		if strings.TrimSpace(seed) == "" || !appearanceVersionOne(a["generation"]) || icon != "" || emoji != "" {
			return ErrAgentAppearanceInvalid
		}
	case "icon":
		if !strings.Contains("\n"+LucideIconData, "\n"+icon+"\n") || emoji != "" {
			return ErrAgentAppearanceInvalid
		}
	case "emoji":
		if !validAgentEmoji(emoji) || icon != "" {
			return ErrAgentAppearanceInvalid
		}
	default:
		return ErrAgentAppearanceInvalid
	}
	return nil
}

//go:embed lucide_icons.gen.txt
var LucideIconData string

func validAgentEmoji(value string) bool {
	if len(value) > 64 || !utf8.ValidString(value) || strings.ContainsFunc(value, unicode.IsControl) ||
		uniseg.GraphemeClusterCount(value) != 1 {
		return false
	}

	for _, r := range value {
		switch r {
		case 0x20E3, 0x00A9, 0x00AE, 0x3030, 0x303D, 0x3297, 0x3299:
			return true
		}
		for _, bounds := range [][2]rune{
			{0x1F000, 0x1FAFF}, {0x2600, 0x27BF}, {0x2190, 0x21FF},
			{0x2300, 0x23FF}, {0x2B00, 0x2BFF},
		} {
			if r >= bounds[0] && r <= bounds[1] {
				return true
			}
		}
	}

	return false
}

func EqualAgentAppearance(a, b map[string]any) bool {
	left, err := json.Marshal(a, json.Deterministic(true))
	if err != nil {
		return false
	}
	right, err := json.Marshal(b, json.Deterministic(true))
	return err == nil && bytes.Equal(left, right)
}

func appearanceVersionOne(value any) bool {
	switch v := value.(type) {
	case int:
		return v == 1
	case int64:
		return v == 1
	case uint64:
		return v == 1
	case float64:
		return v == 1
	default:
		return false
	}
}

func agentAppearanceString(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}
