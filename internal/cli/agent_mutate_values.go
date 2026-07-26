package cli

import (
	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

func normalizeAgentFlagValues(values []string) []string {
	normalized := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			normalized = append(normalized, value)
		}
	}
	return normalized
}

func cloneStrings(values []string) []string {
	return append([]string(nil), values...)
}

func cloneAgentSkills(skills *contract.CreateAgentSkillsConfig) *contract.CreateAgentSkillsConfig {
	if skills == nil {
		return nil
	}
	return &contract.CreateAgentSkillsConfig{Disabled: cloneStrings(skills.Disabled)}
}
