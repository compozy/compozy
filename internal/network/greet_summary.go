package network

import (
	"encoding/json"
	"fmt"
	"strings"
)

const maxNetworkPreviewRunes = 160

// ResolveGreetSummary returns a deterministic operator-facing summary for one
// greet advertisement.
func ResolveGreetSummary(card PeerCard, summary string) string {
	if trimmed := strings.TrimSpace(summary); trimmed != "" {
		return truncateNetworkPreview(trimmed)
	}

	name := strings.TrimSpace(card.PeerID)
	if card.DisplayName != nil {
		if trimmed := strings.TrimSpace(*card.DisplayName); trimmed != "" {
			name = trimmed
		}
	}
	if name == "" {
		name = "Peer"
	}

	capabilityLabel, extraCount := greetCapabilityLabel(card)
	if capabilityLabel == "" {
		return truncateNetworkPreview(name + " is present")
	}

	summaryText := fmt.Sprintf("%s ready for %s", name, capabilityLabel)
	if extraCount > 0 {
		summaryText = fmt.Sprintf("%s +%d more", summaryText, extraCount)
	}
	return truncateNetworkPreview(summaryText)
}

func greetCapabilityLabel(card PeerCard) (string, int) {
	briefs := decodeCapabilityBriefs(card.Ext[capabilityBriefExtKey])
	if len(briefs) > 0 {
		label := strings.TrimSpace(briefs[0].Summary)
		if label == "" {
			label = strings.TrimSpace(briefs[0].ID)
		}
		if label != "" {
			return label, len(briefs) - 1
		}
	}

	for idx, capability := range card.Capabilities {
		if trimmed := strings.TrimSpace(capability); trimmed != "" {
			extra := 0
			for _, candidate := range card.Capabilities[idx+1:] {
				if strings.TrimSpace(candidate) != "" {
					extra++
				}
			}
			return trimmed, extra
		}
	}
	return "", 0
}

func decodeCapabilityBriefs(raw json.RawMessage) []capabilityBrief {
	if len(raw) == 0 || !json.Valid(raw) {
		return nil
	}

	var brief []capabilityBrief
	if err := json.Unmarshal(raw, &brief); err != nil {
		return nil
	}

	filtered := make([]capabilityBrief, 0, len(brief))
	for _, item := range brief {
		id := strings.TrimSpace(item.ID)
		summary := strings.TrimSpace(item.Summary)
		if id == "" && summary == "" {
			continue
		}
		filtered = append(filtered, capabilityBrief{ID: id, Summary: summary})
	}
	return filtered
}

func truncateNetworkPreview(value string) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxNetworkPreviewRunes {
		return string(runes)
	}
	return strings.TrimSpace(string(runes[:maxNetworkPreviewRunes]))
}
