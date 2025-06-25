package strategies

import (
	"context"

	"github.com/compozy/compozy/engine/memory/core"
)

// SimpleTokenCounterAdapter adapts a simple estimation algorithm to core.TokenCounter interface
type SimpleTokenCounterAdapter struct {
	tokensPerChar float64
}

// NewSimpleTokenCounterAdapter creates a new simple token counter adapter
func NewSimpleTokenCounterAdapter() core.TokenCounter {
	return &SimpleTokenCounterAdapter{
		tokensPerChar: 0.25, // Approximately 4 characters per token
	}
}

// CountTokens estimates token count for text content
func (tc *SimpleTokenCounterAdapter) CountTokens(_ context.Context, text string) (int, error) {
	if text == "" {
		return 0, nil
	}
	// Simple character-based estimation with minimum of 1 token
	tokenCount := int(float64(len(text)) * tc.tokensPerChar)
	if tokenCount < 1 {
		return 1, nil
	}
	return tokenCount, nil
}

// GetEncoding returns the name of the encoding being used
func (tc *SimpleTokenCounterAdapter) GetEncoding() string {
	return "simple-estimation"
}

// GPTTokenCounterAdapter provides GPT-style token estimation
type GPTTokenCounterAdapter struct {
	averageTokenLength float64
}

// NewGPTTokenCounterAdapter creates a new GPT-style token counter adapter
func NewGPTTokenCounterAdapter() core.TokenCounter {
	return &GPTTokenCounterAdapter{
		averageTokenLength: 4.0, // GPT tokens average ~4 characters
	}
}

// CountTokens estimates token count using GPT-style calculation
func (tc *GPTTokenCounterAdapter) CountTokens(_ context.Context, text string) (int, error) {
	if text == "" {
		return 0, nil
	}
	// More accurate estimation considering word boundaries and punctuation
	tokenCount := int(float64(len(text)) / tc.averageTokenLength)

	// Adjust for common patterns
	if len(text) < 10 {
		// Very short text tends to be less efficient
		tokenCount = maxInt(1, tokenCount)
	} else if len(text) > 1000 {
		// Long text tends to be more efficient
		tokenCount = int(float64(tokenCount) * 0.9)
	}

	return maxInt(1, tokenCount), nil
}

// GetEncoding returns the name of the encoding being used
func (tc *GPTTokenCounterAdapter) GetEncoding() string {
	return "gpt-estimation"
}

// maxInt returns the larger of two integers
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
