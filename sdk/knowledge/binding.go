package knowledge

import (
	"context"
	"fmt"
	"math"
	"strings"

	enginecore "github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

const (
	minBindingScore = 0.0
	maxBindingScore = 1.0
)

// BindingBuilder configures knowledge bindings with optional retrieval overrides while collecting validation errors.
type BindingBuilder struct {
	config *enginecore.KnowledgeBinding
	errors []error
}

// NewBinding creates a knowledge binding builder for the provided knowledge base identifier.
func NewBinding(knowledgeBaseID string) *BindingBuilder {
	trimmedID := strings.TrimSpace(knowledgeBaseID)
	return &BindingBuilder{
		config: &enginecore.KnowledgeBinding{ID: trimmedID},
		errors: make([]error, 0),
	}
}

// WithTopK overrides the number of results retrieved from the knowledge base.
func (b *BindingBuilder) WithTopK(topK int) *BindingBuilder {
	if b == nil {
		return nil
	}
	if topK <= 0 {
		b.errors = append(b.errors, fmt.Errorf("top_k override must be greater than zero: got %d", topK))
		return b
	}
	value := topK
	b.config.TopK = &value
	return b
}

// WithMinScore overrides the minimum relevance score accepted from retrieval results.
func (b *BindingBuilder) WithMinScore(score float64) *BindingBuilder {
	if b == nil {
		return nil
	}
	if math.IsNaN(score) || math.IsInf(score, 0) {
		b.errors = append(b.errors, fmt.Errorf("min_score override must be a finite number"))
		return b
	}
	if score < minBindingScore || score > maxBindingScore {
		b.errors = append(
			b.errors,
			fmt.Errorf(
				"min_score override must be between %.1f and %.1f inclusive: got %.4f",
				minBindingScore,
				maxBindingScore,
				score,
			),
		)
		return b
	}
	value := score
	b.config.MinScore = &value
	return b
}

// WithMaxTokens overrides the maximum tokens injected into the agent prompt.
func (b *BindingBuilder) WithMaxTokens(max int) *BindingBuilder {
	if b == nil {
		return nil
	}
	if max <= 0 {
		b.errors = append(b.errors, fmt.Errorf("max_tokens override must be greater than zero: got %d", max))
		return b
	}
	value := max
	b.config.MaxTokens = &value
	return b
}

// Build validates the binding configuration and returns an immutable clone.
func (b *BindingBuilder) Build(ctx context.Context) (*enginecore.KnowledgeBinding, error) {
	if b == nil {
		return nil, fmt.Errorf("knowledge binding builder is required")
	}
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	b.config.ID = strings.TrimSpace(b.config.ID)
	log.Debug("building knowledge binding", "knowledge_base", b.config.ID)
	collected := make([]error, 0, len(b.errors)+4)
	collected = append(collected, b.errors...)
	collected = append(collected, validate.ValidateID(ctx, b.config.ID))
	collected = append(collected, b.validateTopK())
	collected = append(collected, b.validateMinScore())
	collected = append(collected, b.validateMaxTokens())
	filtered := filterErrors(collected)
	if len(filtered) > 0 {
		return nil, &sdkerrors.BuildError{Errors: filtered}
	}
	cloned := b.config.Clone()
	return &cloned, nil
}

func (b *BindingBuilder) validateTopK() error {
	if b == nil || b.config == nil || b.config.TopK == nil {
		return nil
	}
	if *b.config.TopK <= 0 {
		return fmt.Errorf("top_k override must be greater than zero: got %d", *b.config.TopK)
	}
	return nil
}

func (b *BindingBuilder) validateMinScore() error {
	if b == nil || b.config == nil || b.config.MinScore == nil {
		return nil
	}
	score := *b.config.MinScore
	if math.IsNaN(score) || math.IsInf(score, 0) {
		return fmt.Errorf("min_score override must be a finite number")
	}
	if score < minBindingScore || score > maxBindingScore {
		return fmt.Errorf(
			"min_score override must be between %.1f and %.1f inclusive: got %.4f",
			minBindingScore,
			maxBindingScore,
			score,
		)
	}
	return nil
}

func (b *BindingBuilder) validateMaxTokens() error {
	if b == nil || b.config == nil || b.config.MaxTokens == nil {
		return nil
	}
	if *b.config.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens override must be greater than zero: got %d", *b.config.MaxTokens)
	}
	return nil
}
