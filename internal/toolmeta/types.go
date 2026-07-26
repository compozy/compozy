// Package toolmeta renders safe, provider-neutral tool progress presentation metadata.
package toolmeta

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	redactpkg "github.com/compozy/agh/internal/redact"
)

const maxDescriptorPresentationRunes = 80

// Entry is the curated presentation metadata for one native tool family.
type Entry struct {
	Verb         string
	Connector    string
	DropsPreview bool
	Emoji        string
	Preview      string
}

// DescriptorMetadata is the narrow presentation-only view implemented by tool registries.
type DescriptorMetadata struct {
	ID           string
	DisplayTitle string
	FriendlyVerb string
	Preview      string
}

// ValidateDescriptorMetadata rejects executable or multiline presentation hints.
func ValidateDescriptorMetadata(friendlyVerb string, preview string) error {
	if err := validatePresentationText("friendly_verb", friendlyVerb); err != nil {
		return err
	}
	if strings.TrimSpace(preview) == "" {
		return nil
	}
	if normalized := normalizePreviewHint(preview); normalized == "" {
		return errors.New("toolmeta: preview must be a supported declarative hint")
	}
	return nil
}

func validatePresentationText(field string, value string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if oneLine(trimmed) != trimmed {
		return fmt.Errorf("toolmeta: %s must be one line", field)
	}
	if utf8.RuneCountInString(trimmed) > maxDescriptorPresentationRunes {
		return fmt.Errorf("toolmeta: %s exceeds %d runes", field, maxDescriptorPresentationRunes)
	}
	if redactpkg.String(trimmed) != trimmed {
		return fmt.Errorf("toolmeta: %s contains secret-shaped material", field)
	}
	for _, char := range trimmed {
		if char < 0x20 || char == 0x7f {
			return fmt.Errorf("toolmeta: %s contains control characters", field)
		}
	}
	return nil
}

// DescriptorLookup resolves optional extension or MCP presentation metadata within one workspace.
type DescriptorLookup interface {
	LookupToolMetadata(workspaceID string, toolID string) (DescriptorMetadata, bool)
}

// RenderOptions controls safe preview rendering.
type RenderOptions struct {
	Descriptor   DescriptorMetadata
	PreviewLimit int
	Verbose      bool
}

// Presentation is the daemon-rendered progress chrome sent to bridge adapters.
type Presentation struct {
	ToolID    string
	Label     string
	Preview   string
	Emoji     string
	Connector string
	Fallback  bool
}

// ProgressFields returns provider-neutral fields for the bridge SDK renderer.
func (p Presentation) ProgressFields() (string, string) {
	label := p.Label
	if p.Preview == "" {
		return label, ""
	}
	if p.Fallback {
		return label + ":", strconv.Quote(p.Preview)
	}
	return strings.TrimSpace(label + p.Connector), p.Preview
}

// String renders one human-readable progress line.
func (p Presentation) String() string {
	label, preview := p.ProgressFields()
	line := strings.TrimSpace(p.Emoji + " " + label)
	if preview != "" {
		line += " " + preview
	}
	return line
}

func decodeInput(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var input map[string]any
	if err := json.Unmarshal(raw, &input); err != nil {
		return nil
	}
	return input
}
