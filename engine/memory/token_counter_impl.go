package memory

import (
	"context"
	"fmt"
	"sync"

	"github.com/pkoukk/tiktoken-go"
	// Assuming a logger is available, e.g., from "github.com/compozy/compozy/pkg/logger"
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

// getEncodingNameForModel returns the encoding name for a given model.
// This is a workaround since tiktoken-go doesn't expose the encoding name directly.
func getEncodingNameForModel(model string) string {
	// Known model to encoding mappings
	switch model {
	case "gpt-4", "gpt-4-32k", "gpt-4-turbo", "gpt-3.5-turbo":
		return "cl100k_base"
	case "gpt2", "text-davinci-002", "text-davinci-003":
		return "p50k_base"
	case "davinci", "curie", "babbage", "ada":
		return "r50k_base"
	default:
		// Default to cl100k_base for unknown models
		return defaultEncoding
	}
}

// Utility function to create a default token counter, useful for tests or fallbacks.
func DefaultTokenCounter() (TokenCounter, error) {
	return NewTiktokenCounter(defaultEncoding)
}
