package core

import (
	"context"
	"unicode/utf8"
)

// TokenEstimationStrategy defines the strategy for estimating tokens when actual counting fails
type TokenEstimationStrategy string

const (
	// EnglishEstimation uses the standard 1 token ≈ 4 characters for English text
	EnglishEstimation TokenEstimationStrategy = "english"
	// UnicodeEstimation uses character count based estimation for Unicode-heavy text
	UnicodeEstimation TokenEstimationStrategy = "unicode"
	// ChineseEstimation uses optimized estimation for Chinese/Japanese/Korean text
	ChineseEstimation TokenEstimationStrategy = "chinese"
	// ConservativeEstimation uses a conservative approach assuming higher token density
	ConservativeEstimation TokenEstimationStrategy = "conservative"
)

// TokenEstimator provides fallback token estimation when actual counting fails
type TokenEstimator interface {
	EstimateTokens(ctx context.Context, text string) int
}

// DefaultTokenEstimator implements TokenEstimator with configurable strategies
type DefaultTokenEstimator struct {
	strategy TokenEstimationStrategy
}

// NewTokenEstimator creates a new token estimator with the given strategy
func NewTokenEstimator(strategy TokenEstimationStrategy) *DefaultTokenEstimator {
	if strategy == "" {
		strategy = EnglishEstimation
	}
	return &DefaultTokenEstimator{strategy: strategy}
}

// EstimateTokens estimates token count based on the configured strategy
func (e *DefaultTokenEstimator) EstimateTokens(_ context.Context, text string) int {
	if text == "" {
		return 0
	}
	switch e.strategy {
	case UnicodeEstimation:
		// For Unicode-heavy text, use rune count as a better approximation
		// Based on empirical observation that Unicode characters often map 2:1 to tokens
		return utf8.RuneCountInString(text) / 2
	case ChineseEstimation:
		// For CJK text, characters are often 1:1 or 2:1 with tokens
		// Using 1.5:1 ratio based on analysis of common CJK tokenization patterns
		runeCount := utf8.RuneCountInString(text)
		return (runeCount * 2) / 3
	case ConservativeEstimation:
		// Conservative estimation assumes higher token density
		// Uses 3:1 char-to-token ratio to overestimate for safety in memory management
		return len(text) / 3
	case EnglishEstimation:
		// Standard English estimation: ~4 characters per token
		// Based on GPT tokenizer analysis of typical English text
		return len(text) / 4
	default:
		// Default to English estimation
		e.strategy = EnglishEstimation
		return len(text) / 4
	}
}

// TokenCounterWithFallback wraps a TokenCounter with fallback estimation
type TokenCounterWithFallback struct {
	counter   TokenCounter
	estimator TokenEstimator
}

// NewTokenCounterWithFallback creates a new token counter with fallback estimation
func NewTokenCounterWithFallback(counter TokenCounter, estimator TokenEstimator) *TokenCounterWithFallback {
	if estimator == nil {
		estimator = NewTokenEstimator(EnglishEstimation)
	}
	return &TokenCounterWithFallback{
		counter:   counter,
		estimator: estimator,
	}
}

// CountTokens counts tokens using the primary counter, falling back to estimation on error
func (t *TokenCounterWithFallback) CountTokens(ctx context.Context, text string) (int, error) {
	if t.counter != nil {
		count, err := t.counter.CountTokens(ctx, text)
		if err == nil {
			return count, nil
		}
		// Fall back to estimation on error
	}
	return t.estimator.EstimateTokens(ctx, text), nil
}

// GetEncoding returns the encoding name from the primary counter
func (t *TokenCounterWithFallback) GetEncoding() string {
	if t.counter != nil {
		return t.counter.GetEncoding()
	}
	return "estimation"
}
