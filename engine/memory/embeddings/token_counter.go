package embeddings

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/pkoukk/tiktoken-go"
	"golang.org/x/sync/singleflight"
)

type counterKey struct {
	provider string
	model    string
}

type tokenCounter interface {
	CountTokens(ctx context.Context, text string) (int, error)
}

const defaultEncoding = "cl100k_base"

var (
	tokenCounters   sync.Map
	tokenizerBuilds singleflight.Group
)

// EstimateTokens counts tokens for the provided texts using a cached tokenizer per model.
func EstimateTokens(ctx context.Context, provider string, model string, texts []string) (int, error) {
	if len(texts) == 0 {
		return 0, nil
	}
	counter, err := counterForModel(provider, model)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, text := range texts {
		count, countErr := counter.CountTokens(ctx, text)
		if countErr != nil {
			return total, fmt.Errorf("count tokens: %w", countErr)
		}
		total += count
	}
	return total, nil
}

func counterForModel(provider string, model string) (tokenCounter, error) {
	key := counterKey{
		provider: strings.TrimSpace(provider),
		model:    strings.TrimSpace(model),
	}
	if cached, ok := tokenCounters.Load(key); ok {
		if counter, valid := cached.(tokenCounter); valid {
			return counter, nil
		}
	}
	v, err, _ := tokenizerBuilds.Do(key.provider+"|"+key.model, func() (any, error) {
		return newTokenizer(key.model)
	})
	if err != nil {
		return nil, fmt.Errorf("create tokenizer for provider %s model %s: %w", provider, model, err)
	}
	counter, ok := v.(tokenCounter)
	if !ok {
		return nil, fmt.Errorf("unexpected tokenizer type %T", v)
	}
	tokenCounters.Store(key, counter)
	return counter, nil
}

// resetTokenCounters clears the tokenizer cache; intended for tests only.
func resetTokenCounters() {
	tokenCounters = sync.Map{}
}

type tiktokenCounter struct {
	encoder *tiktoken.Tiktoken
}

func newTokenizer(model string) (tokenCounter, error) {
	trimmed := strings.TrimSpace(model)
	encoder, err := resolveEncoder(trimmed)
	if err != nil {
		return nil, err
	}
	return &tiktokenCounter{encoder: encoder}, nil
}

func resolveEncoder(model string) (*tiktoken.Tiktoken, error) {
	if model != "" {
		if enc, err := tiktoken.EncodingForModel(model); err == nil {
			return enc, nil
		}
	}
	enc, err := tiktoken.GetEncoding(defaultEncoding)
	if err != nil {
		return nil, fmt.Errorf("get default encoding: %w", err)
	}
	return enc, nil
}

// CountTokens returns the number of tokens in the text using the configured encoder.
// The context parameter is unused as tokenization is CPU-only and non-cancellable.
func (c *tiktokenCounter) CountTokens(ctx context.Context, text string) (int, error) {
	_ = ctx
	if c.encoder == nil {
		return 0, fmt.Errorf("tiktoken encoder not initialized")
	}
	tokens := c.encoder.Encode(text, nil, nil)
	return len(tokens), nil
}
