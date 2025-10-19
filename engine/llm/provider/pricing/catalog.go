package pricing

import (
	"strings"

	"github.com/compozy/compozy/engine/core"
	llmadapter "github.com/compozy/compozy/engine/llm/adapter"
)

const tokensPerThousand = 1000.0

// Price captures per-1K token pricing for prompt and completion usage.
type Price struct {
	PromptPer1K     float64
	CompletionPer1K float64
}

var providerCatalog = map[string]map[string]Price{
	string(core.ProviderOpenAI): {
		"gpt-4o": {
			PromptPer1K:     0.005, // $5 per million input tokens
			CompletionPer1K: 0.015, // $15 per million output tokens
		},
		"gpt-4o-mini": {
			PromptPer1K:     0.00015, // $0.15 per million input tokens
			CompletionPer1K: 0.0006,  // $0.60 per million output tokens
		},
	},
	string(core.ProviderAnthropic): {
		"claude-3-5-sonnet": {
			PromptPer1K:     0.003, // $3 per million input tokens
			CompletionPer1K: 0.015,
		},
		"claude-3-5-haiku": {
			PromptPer1K:     0.0008, // $0.8 per million input tokens
			CompletionPer1K: 0.004,
		},
	},
	string(core.ProviderGoogle): {
		"gemini-1-5-pro": {
			PromptPer1K:     0.0035, // $3.50 per million input tokens
			CompletionPer1K: 0.0105, // $10.50 per million output tokens
		},
		"gemini-1-5-flash": {
			PromptPer1K:     0.00035, // $0.35 per million input tokens
			CompletionPer1K: 0.00053,
		},
	},
}

// Lookup returns pricing information for the given provider/model pair when available.
func Lookup(provider core.ProviderName, model string) (Price, bool) {
	catalog, ok := providerCatalog[strings.ToLower(string(provider))]
	if !ok {
		return Price{}, false
	}
	normalized := normalizeModel(model)
	price, ok := catalog[normalized]
	return price, ok
}

// EstimateCostUSD returns the estimated USD cost for the usage profile when pricing is known.
func EstimateCostUSD(provider core.ProviderName, model string, usage *llmadapter.Usage) (float64, bool) {
	if usage == nil {
		return 0, false
	}
	price, ok := Lookup(provider, model)
	if !ok {
		return 0, false
	}
	promptCost := (float64(max(usage.PromptTokens, 0)) / tokensPerThousand) * price.PromptPer1K
	completionCost := (float64(max(usage.CompletionTokens, 0)) / tokensPerThousand) * price.CompletionPer1K
	total := promptCost + completionCost
	if total <= 0 {
		return 0, false
	}
	return total, true
}

func normalizeModel(model string) string {
	model = strings.TrimSpace(strings.ToLower(model))
	model = strings.ReplaceAll(model, ":", "-")
	model = strings.ReplaceAll(model, ".", "-")
	return model
}
