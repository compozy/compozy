package tokens

import (
	"context"
	"fmt"
	"sync"

	memcore "github.com/compozy/compozy/engine/memory/core"
	"github.com/pkoukk/tiktoken-go"
)

const (
	defaultEncoding = "cl100k_base" // Default encoding if model-specific one fails
	// Common model names to encoding mapping can be added here if needed,
	// though tiktoken-go handles many common ones automatically.
)

// TiktokenCounter implements the TokenCounter interface using the tiktoken-go library.
type TiktokenCounter struct {
	encodingName string
	tke          *tiktoken.Tiktoken
	mu           sync.RWMutex // Protects tke if re-initialization is ever needed
}

// NewTiktokenCounter creates a new counter for the given model or encoding.
// If modelOrEncoding is a known model name, it tries to get the encoding for that model.
// Otherwise, it treats modelOrEncoding as an encoding name.
// Falls back to defaultEncoding if the specified one is not found.
func NewTiktokenCounter(modelOrEncoding string) (*TiktokenCounter, error) {
	if modelOrEncoding == "" {
		modelOrEncoding = defaultEncoding
	}

	var encodingName string
	tke, err := tiktoken.GetEncoding(modelOrEncoding)
	if err != nil {
		// Try as a model name
		tke, err = tiktoken.EncodingForModel(modelOrEncoding)
		if err != nil {
			// Fallback to default if specific model/encoding fails
			// Warning: Failed to get encoding, falling back to default
			tke, err = tiktoken.GetEncoding(defaultEncoding)
			if err != nil {
				// This would be a critical issue if the default encoding itself fails
				return nil, fmt.Errorf("failed to get default encoding '%s': %w", defaultEncoding, err)
			}
			encodingName = defaultEncoding
		} else {
			// Successfully got encoding for model - need to get the actual encoding name
			// tiktoken-go doesn't expose the encoding name directly, so we'll determine it
			// For now, we'll use a known mapping
			encodingName = getEncodingNameForModel(modelOrEncoding)
		}
	} else {
		encodingName = modelOrEncoding
	}

	return &TiktokenCounter{
		encodingName: encodingName, // Store the name of the encoding actually used
		tke:          tke,
	}, nil
}

// CountTokens counts the number of tokens in the given text using the configured encoding.
func (tc *TiktokenCounter) CountTokens(_ context.Context, text string) (int, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	if tc.tke == nil {
		return 0, fmt.Errorf("tiktoken encoder is not initialized for encoding %s", tc.encodingName)
	}

	tokens := tc.tke.Encode(text, nil, nil) // Pass nil for allowedSpecial and disallowedSpecial
	return len(tokens), nil
}

// GetEncoding returns the name of the encoding being used by this counter.
func (tc *TiktokenCounter) GetEncoding() string {
	tc.mu.RLock()
	defer tc.mu.RUnlock()
	return tc.encodingName
}

// EncodeTokens encodes text into tokens and returns the token IDs.
func (tc *TiktokenCounter) EncodeTokens(_ context.Context, text string) ([]int, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	if tc.tke == nil {
		return nil, fmt.Errorf("tiktoken encoder is not initialized for encoding %s", tc.encodingName)
	}

	tokens := tc.tke.Encode(text, nil, nil)
	return tokens, nil
}

// DecodeTokens decodes token IDs back into text.
func (tc *TiktokenCounter) DecodeTokens(_ context.Context, tokens []int) (string, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	if tc.tke == nil {
		return "", fmt.Errorf("tiktoken encoder is not initialized for encoding %s", tc.encodingName)
	}

	text := tc.tke.Decode(tokens)
	return text, nil
}

// modelToEncodingMap maps common model names to their encoding names
var modelToEncodingMap = map[string]string{
	// GPT-4 and variants
	"gpt-4":               "cl100k_base",
	"gpt-4-0314":          "cl100k_base",
	"gpt-4-0613":          "cl100k_base",
	"gpt-4-32k":           "cl100k_base",
	"gpt-4-32k-0314":      "cl100k_base",
	"gpt-4-32k-0613":      "cl100k_base",
	"gpt-4-turbo":         "cl100k_base",
	"gpt-4-turbo-preview": "cl100k_base",

	// GPT-3.5-turbo
	"gpt-3.5-turbo":          "cl100k_base",
	"gpt-3.5-turbo-0301":     "cl100k_base",
	"gpt-3.5-turbo-0613":     "cl100k_base",
	"gpt-3.5-turbo-16k":      "cl100k_base",
	"gpt-3.5-turbo-16k-0613": "cl100k_base",

	// Older models
	"text-davinci-003": "p50k_base",
	"text-davinci-002": "p50k_base",
	"text-davinci-001": "p50k_base",
	"text-curie-001":   "p50k_base",
	"text-babbage-001": "p50k_base",
	"text-ada-001":     "p50k_base",
	"davinci":          "p50k_base",
	"curie":            "p50k_base",
	"babbage":          "p50k_base",
	"ada":              "p50k_base",

	// Code models
	"code-davinci-002": "p50k_base",
	"code-davinci-001": "p50k_base",
	"code-cushman-002": "p50k_base",
	"code-cushman-001": "p50k_base",
}

// getEncodingNameForModel returns the encoding name for a given model.
// Uses tiktoken-go's EncodingForModel to leverage its built-in model mappings,
// falling back to default encoding for unknown models.
func getEncodingNameForModel(model string) string {
	// First check our explicit mapping
	if encoding, ok := modelToEncodingMap[model]; ok {
		return encoding
	}
	// For unknown models, fall back to the most common modern encoding
	// cl100k_base is used by GPT-4, GPT-3.5-turbo, and most recent models
	return defaultEncoding
}

// DefaultTokenCounter creates a default token counter, useful for tests or fallbacks.
func DefaultTokenCounter() (memcore.TokenCounter, error) {
	return NewTiktokenCounter(defaultEncoding)
}
