package acpmock

import (
	"encoding/json"
	"strings"
)

const (
	loopOutputContractPrefix = "Output contract:\n" +
		"End your final message with exactly one JSON object that satisfies this output_schema " +
		"(no other JSON object may follow it): "
	outputContractPromptMarker   = "\n\n" + loopOutputContractPrefix
	schemaRetryPromptMarker      = "\n\nYour previous response did not satisfy output_schema: "
	schemaRetryInstructionMarker = "\nReturn exactly one JSON object that satisfies this output_schema: "
	userRequestPromptMarker      = "User request:"
)

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
	for _, marker := range []string{userRequestPromptMarker} {
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
	next := stripTrailingRunAgentPromptContract(prompt)
	next = stripLeadingPromptBlock(next, compozySituationContextOpen, compozySituationContextClose)
	next = stripLeadingSkillsCatalogBlock(next, currentAvailableSkillsOpen, currentAvailableSkillsClose)
	next = stripLeadingSkillsCatalogBlock(next, availableSkillsOpen, availableSkillsClose)
	next = stripLeadingSelfClosingPromptBlock(next, currentAvailableSkillsSelfClosing)
	next = stripLeadingSelfClosingPromptBlock(next, availableSkillsSelfClosing)
	next = stripLeadingDurableMemoryBlock(next)
	next = stripLeadingPromptBlock(next, workspaceKnowledgeOpen, workspaceKnowledgeClose)
	next = stripLeadingUserMessageBlock(next)
	next = stripLeadingInboundBridgePrompt(next)
	next = stripTrailingLoopOutputContract(next)
	return strings.TrimSpace(next)
}

func stripTrailingLoopOutputContract(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	marker := "\n\n" + loopOutputContractPrefix
	markerIndex := strings.LastIndex(trimmed, marker)
	if markerIndex < 0 {
		return trimmed
	}
	schema := strings.TrimSpace(trimmed[markerIndex+len(marker):])
	if !strings.HasPrefix(schema, "{") || !json.Valid([]byte(schema)) {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:markerIndex])
}

func stripTrailingRunAgentPromptContract(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if index := strings.LastIndex(trimmed, schemaRetryPromptMarker); index >= 0 {
		retry := trimmed[index+len(schemaRetryPromptMarker):]
		instructionIndex := strings.LastIndex(retry, schemaRetryInstructionMarker)
		if instructionIndex >= 0 {
			schema := strings.TrimSpace(retry[instructionIndex+len(schemaRetryInstructionMarker):])
			if json.Valid([]byte(schema)) {
				trimmed = strings.TrimSpace(trimmed[:index])
			}
		}
	}
	if index := strings.LastIndex(trimmed, outputContractPromptMarker); index >= 0 {
		schema := strings.TrimSpace(trimmed[index+len(outputContractPromptMarker):])
		if json.Valid([]byte(schema)) {
			trimmed = strings.TrimSpace(trimmed[:index])
		}
	}
	return trimmed
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
