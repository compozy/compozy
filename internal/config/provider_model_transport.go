package config

import "strings"

const (
	claudeSonnetACPModelValue = "sonnet"
	claudeOpusACPModelValue   = "opus[1m]"
	claudeHaikuACPModelValue  = "haiku"
)

// ACPModelTransportValue returns the provider-owned config-option value for a
// canonical AGH model ID. Unknown and custom IDs remain exact so the live ACP
// option remains the final authority.
func ACPModelTransportValue(provider string, canonicalModel string) string {
	model := strings.TrimSpace(canonicalModel)
	if strings.TrimSpace(provider) != providerClaudeKey {
		return model
	}
	switch model {
	case modelClaudeSonnet5ID:
		return claudeSonnetACPModelValue
	case modelClaudeOpus48ID:
		return claudeOpusACPModelValue
	case modelClaudeHaiku45CurrentID:
		return claudeHaikuACPModelValue
	default:
		return model
	}
}
