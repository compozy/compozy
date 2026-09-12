package acp

import (
	"encoding/json"
	"log/slog"

	"github.com/compozy/compozy/internal/redact"
)

func decodeUsageMeta(raw json.RawMessage) map[string]any {
	var meta map[string]any
	if err := json.Unmarshal(redact.ClaimTokensJSON(raw), &meta); err != nil {
		return nil
	}
	return meta
}

func validateContextNumbers(usage TokenUsage) (TokenUsage, []string) {
	var dropped []string
	for _, field := range []struct {
		name    string
		value   **int64
		minimum int64
	}{
		{"used", &usage.ContextUsed, 0},
		{"size", &usage.ContextSize, 1},
		{"input_tokens", &usage.InputTokens, 0},
		{"output_tokens", &usage.OutputTokens, 0},
		{"total_tokens", &usage.TotalTokens, 0},
		{"thought_tokens", &usage.ThoughtTokens, 0},
		{"cache_read_tokens", &usage.CacheReadTokens, 0},
		{"cache_write_tokens", &usage.CacheWriteTokens, 0},
	} {
		if *field.value != nil && **field.value < field.minimum {
			*field.value = nil
			dropped = append(dropped, field.name)
		}
	}
	return usage, dropped
}

func (p *AgentProcess) validatedUsage(usage TokenUsage) TokenUsage {
	validated, dropped := validateContextNumbers(usage)
	if len(dropped) > 0 {
		p.usageRangeWarning.Do(func() {
			p.usageLogger().
				Warn("acp: dropped out-of-range context number", "session_id", p.SessionID, "fields", dropped)
		})
	}
	return validated
}

func (p *AgentProcess) warnUsageAlias(usage *wireUsage) {
	if usage == nil || (usage.LegacyCacheReadTokens == nil && usage.LegacyCacheWriteTokens == nil) {
		return
	}
	p.usageAliasWarning.Do(func() {
		p.usageLogger().Warn(
			"acp: cacheReadTokens/cacheWriteTokens are deprecated; "+
				"send cachedReadTokens/cachedWriteTokens (alias removed in v0.6.0)",
			"session_id", p.SessionID,
		)
	})
}

func (p *AgentProcess) usageLogger() *slog.Logger {
	if p.logger != nil {
		return p.logger
	}
	return slog.Default()
}
