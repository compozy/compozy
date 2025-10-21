package strategies

import (
	"context"

	"github.com/compozy/compozy/engine/memory/core"
)

const (
	// DefaultTokensPerChar represents the estimated tokens per character (1 token per 4 chars)
	DefaultTokensPerChar = 0.25
	// SimpleEstimationEncoding is the encoding name for simple estimation
	SimpleEstimationEncoding = "simple-estimation"
	// GPTAverageTokenLength represents the average character length per token in GPT models
	GPTAverageTokenLength = 4.0
	// GPTEstimationEncoding is the encoding name for GPT-style estimation
	GPTEstimationEncoding = "gpt-estimation"
)

// SimpleTokenCounterAdapter adapts a simple estimation algorithm to core.TokenCounter interface
type SimpleTokenCounterAdapter struct {
	tokensPerChar float64
}

// NewSimpleTokenCounterAdapter creates a new simple token counter adapter
func NewSimpleTokenCounterAdapter() core.TokenCounter {
	return &SimpleTokenCounterAdapter{
		tokensPerChar: DefaultTokensPerChar,
	}
}

// CountTokens estimates token count for text content
func (tc *SimpleTokenCounterAdapter) CountTokens(_ context.Context, text string) (int, error) {
	if text == "" {
		return 0, nil
	}
	tokenCount := int(float64(len(text)) * tc.tokensPerChar)
	if tokenCount < 1 {
		return 1, nil
	}
	return tokenCount, nil
}

// GetEncoding returns the name of the encoding being used
func (tc *SimpleTokenCounterAdapter) GetEncoding() string {
	return SimpleEstimationEncoding
}

// GPTTokenCounterAdapter provides GPT-style token estimation
type GPTTokenCounterAdapter struct {
	averageTokenLength float64
}

// NewGPTTokenCounterAdapter creates a new GPT-style token counter adapter
func NewGPTTokenCounterAdapter() core.TokenCounter {
	return &GPTTokenCounterAdapter{
		averageTokenLength: GPTAverageTokenLength,
	}
}

// CountTokens estimates token count using GPT-style calculation
func (tc *GPTTokenCounterAdapter) CountTokens(_ context.Context, text string) (int, error) {
	if text == "" {
		return 0, nil
	}
	tokenCount := int(float64(len(text)) / tc.averageTokenLength)
	return max(1, tokenCount), nil
}

// GetEncoding returns the name of the encoding being used
func (tc *GPTTokenCounterAdapter) GetEncoding() string {
	return GPTEstimationEncoding
}
