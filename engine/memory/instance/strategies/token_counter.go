package strategies

import (
	"github.com/compozy/compozy/engine/llm"
)

// TokenCounter provides an interface for counting tokens in messages
type TokenCounter interface {
	CountTokens(message llm.Message) int
	CountTokensInContent(content string) int
}

// SimpleTokenCounter provides a basic character-based token estimation
type SimpleTokenCounter struct {
	tokensPerChar float64
}

// NewSimpleTokenCounter creates a new simple token counter
func NewSimpleTokenCounter() TokenCounter {
	return &SimpleTokenCounter{
		tokensPerChar: 0.25, // Approximately 4 characters per token
	}
}

// CountTokens estimates token count for a complete message
func (tc *SimpleTokenCounter) CountTokens(message llm.Message) int {
	// Account for role + content + JSON structure overhead
	roleTokens := tc.CountTokensInContent(string(message.Role))
	contentTokens := tc.CountTokensInContent(message.Content)
	structureOverhead := 2 // JSON structure, field names, etc.
	return roleTokens + contentTokens + structureOverhead
}

// CountTokensInContent estimates token count for text content only
func (tc *SimpleTokenCounter) CountTokensInContent(content string) int {
	if content == "" {
		return 0
	}
	// Simple character-based estimation with minimum of 1 token
	tokenCount := int(float64(len(content)) * tc.tokensPerChar)
	if tokenCount < 1 {
		return 1
	}
	return tokenCount
}

// GPTTokenCounter provides GPT-style token estimation (more accurate)
type GPTTokenCounter struct {
	averageTokenLength float64
}

// NewGPTTokenCounter creates a new GPT-style token counter
func NewGPTTokenCounter() TokenCounter {
	return &GPTTokenCounter{
		averageTokenLength: 4.0, // GPT tokens average ~4 characters
	}
}

// CountTokens estimates token count using GPT-style calculation
func (tc *GPTTokenCounter) CountTokens(message llm.Message) int {
	// More sophisticated estimation considering role and structure
	baseTokens := tc.CountTokensInContent(message.Content)
	roleOverhead := 2      // Role field overhead
	structureOverhead := 3 // JSON structure overhead
	if message.Role == llm.MessageRoleSystem {
		structureOverhead++ // System messages have slightly more overhead
	}
	return baseTokens + roleOverhead + structureOverhead
}

// CountTokensInContent estimates token count with better accuracy
func (tc *GPTTokenCounter) CountTokensInContent(content string) int {
	if content == "" {
		return 0
	}
	// More accurate estimation considering word boundaries and punctuation
	tokenCount := int(float64(len(content)) / tc.averageTokenLength)

	// Adjust for common patterns
	if len(content) < 10 {
		// Very short text tends to be less efficient
		tokenCount = maxInt(1, tokenCount)
	} else if len(content) > 1000 {
		// Long text tends to be more efficient
		tokenCount = int(float64(tokenCount) * 0.9)
	}

	return maxInt(1, tokenCount)
}

// maxInt returns the larger of two integers
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
