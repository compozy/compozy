package acpmock

import "strings"

func canonicalUserText(prompt string) string {
	current := strings.TrimSpace(prompt)
	for {
		current = promptAfterLastUserMarker(current)
		next := stripKnownPromptAugmentation(current)
		next = promptAfterLastUserMarker(next)
		if next == current {
			return current
		}
		current = next
	}
}

func promptAfterLastUserMarker(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	markerIndex := -1
	markerLength := 0
	for _, marker := range []string{"User request:"} {
		index := lastLineMarkerIndex(trimmed, marker)
		if index > markerIndex {
			markerIndex = index
			markerLength = len(marker)
		}
	}
	if markerIndex < 0 {
		return trimmed
	}
	return strings.TrimSpace(trimmed[markerIndex+markerLength:])
}

func lastLineMarkerIndex(text string, marker string) int {
	searchEnd := len(text)
	for searchEnd >= 0 {
		index := strings.LastIndex(text[:searchEnd], marker)
		if index < 0 {
			return -1
		}
		if index == 0 || text[index-1] == '\n' {
			return index
		}
		searchEnd = index
	}
	return -1
}

func stripKnownPromptAugmentation(prompt string) string {
	next := prompt
	next = stripLeadingPromptBlock(next, aghSituationContextOpen, aghSituationContextClose)
	next = stripLeadingSkillsCatalogBlock(next, currentAvailableSkillsOpen, currentAvailableSkillsClose)
	next = stripLeadingSkillsCatalogBlock(next, availableSkillsOpen, availableSkillsClose)
	next = stripLeadingSelfClosingPromptBlock(next, currentAvailableSkillsSelfClosing)
	next = stripLeadingSelfClosingPromptBlock(next, availableSkillsSelfClosing)
	next = stripLeadingDurableMemoryBlock(next)
	next = stripLeadingUserMessageBlock(next)
	next = stripLeadingInboundBridgePrompt(next)
	return strings.TrimSpace(next)
}

func stripLeadingPromptBlock(prompt string, open string, closeTag string) string {
	trimmed := strings.TrimSpace(prompt)
	if !strings.HasPrefix(trimmed, open) {
		return trimmed
	}
	_, after, ok := strings.Cut(trimmed, closeTag)
	if !ok {
		return trimmed
	}
	return strings.TrimSpace(after)
}

func stripLeadingSkillsCatalogBlock(prompt string, open string, closeTag string) string {
	after := stripLeadingPromptBlock(prompt, open, closeTag)
	if after == strings.TrimSpace(prompt) {
		return after
	}
	if stripped, ok := stripLeadingSkillsCatalogInstructions(after); ok {
		return stripped
	}
	if _, rest, ok := strings.Cut(after, currentSkillsCatalogFinalLine); ok {
		return strings.TrimSpace(rest)
	}
	return after
}

func stripLeadingSkillsCatalogInstructions(prompt string) (string, bool) {
	trimmed := strings.TrimSpace(prompt)
	if !strings.HasPrefix(trimmed, currentSkillsCatalogOpeningLine) {
		return trimmed, false
	}
	_, rest, ok := strings.Cut(trimmed, "\n\n")
	if !ok {
		return trimmed, false
	}
	return strings.TrimSpace(rest), true
}

func stripLeadingSelfClosingPromptBlock(prompt string, block string) string {
	trimmed := strings.TrimSpace(prompt)
	if !strings.HasPrefix(trimmed, block) {
		return trimmed
	}
	return strings.TrimSpace(strings.TrimPrefix(trimmed, block))
}

func stripLeadingDurableMemoryBlock(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if !strings.HasPrefix(trimmed, durableMemoryOpen) {
		return trimmed
	}
	_, after, ok := strings.Cut(trimmed, durableMemoryClose)
	if !ok {
		return trimmed
	}
	return stripLeadingUserMessageBlock(after)
}

func stripLeadingUserMessageBlock(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if !strings.HasPrefix(trimmed, fixtureUserMessageOpen) {
		return trimmed
	}
	body := strings.TrimPrefix(trimmed, fixtureUserMessageOpen)
	message, after, ok := strings.Cut(body, fixtureUserMessageClose)
	if !ok {
		return trimmed
	}
	tail := strings.TrimSpace(after)
	if tail == "" {
		return strings.TrimSpace(message)
	}
	return strings.TrimSpace(strings.TrimSpace(message) + "\n" + tail)
}

func stripLeadingInboundBridgePrompt(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if !strings.HasPrefix(trimmed, inboundBridgePromptPrefix) {
		return trimmed
	}
	_, after, ok := strings.Cut(trimmed, "\n\n")
	if !ok {
		return trimmed
	}
	return strings.TrimSpace(after)
}
