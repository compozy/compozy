package knowledge

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
)

func TestBindingBuilder(t *testing.T) {
	t.Run("ShouldBuildBindingWithDefaults", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		binding, err := NewBinding("kb-alpha").Build(ctx)
		require.NoError(t, err)
		require.NotNil(t, binding)
		assert.Equal(t, "kb-alpha", binding.ID)
		assert.Nil(t, binding.TopK)
		assert.Nil(t, binding.MinScore)
		assert.Nil(t, binding.MaxTokens)
	})

	t.Run("ShouldTrimIdentifier", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		binding, err := NewBinding("  kb-docs  ").Build(ctx)
		require.NoError(t, err)
		assert.Equal(t, "kb-docs", binding.ID)
	})

	t.Run("ShouldFailWhenIdentifierEmpty", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		_, err := NewBinding("   ").Build(ctx)
		require.Error(t, err)
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
		assert.ErrorContains(t, buildErr, "id is required")
	})

	t.Run("ShouldOverrideTopK", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		binding, err := NewBinding("kb-overrides").WithTopK(7).Build(ctx)
		require.NoError(t, err)
		require.NotNil(t, binding.TopK)
		assert.Equal(t, 7, *binding.TopK)
	})

	t.Run("ShouldFailWhenTopKInvalid", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		builder := NewBinding("kb-overrides").WithTopK(0)
		_, err := builder.Build(ctx)
		require.Error(t, err)
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
		assert.ErrorContains(t, buildErr, "top_k override must be greater than zero")
		assert.Nil(t, builder.config.TopK)
	})

	t.Run("ShouldOverrideMinScore", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		binding, err := NewBinding("kb-overrides").WithMinScore(0.65).Build(ctx)
		require.NoError(t, err)
		require.NotNil(t, binding.MinScore)
		assert.InDelta(t, 0.65, *binding.MinScore, 0.0001)
	})

	t.Run("ShouldFailWhenMinScoreOutOfRange", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		_, err := NewBinding("kb-overrides").WithMinScore(1.2).Build(ctx)
		require.Error(t, err)
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
		assert.ErrorContains(t, buildErr, "min_score override must be between 0.0 and 1.0 inclusive")
	})

	t.Run("ShouldOverrideMaxTokens", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		binding, err := NewBinding("kb-overrides").WithMaxTokens(900).Build(ctx)
		require.NoError(t, err)
		require.NotNil(t, binding.MaxTokens)
		assert.Equal(t, 900, *binding.MaxTokens)
	})

	t.Run("ShouldFailWhenMaxTokensInvalid", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		_, err := NewBinding("kb-overrides").WithMaxTokens(-10).Build(ctx)
		require.Error(t, err)
		var buildErr *sdkerrors.BuildError
		require.ErrorAs(t, err, &buildErr)
		assert.ErrorContains(t, buildErr, "max_tokens override must be greater than zero")
	})

	t.Run("ShouldCombineOverrides", func(t *testing.T) {
		ctx := logger.ContextWithLogger(t.Context(), logger.NewForTests())
		binding, err := NewBinding("kb-overrides").WithTopK(5).WithMinScore(0.4).WithMaxTokens(600).Build(ctx)
		require.NoError(t, err)
		require.NotNil(t, binding.TopK)
		require.NotNil(t, binding.MinScore)
		require.NotNil(t, binding.MaxTokens)
		assert.Equal(t, 5, *binding.TopK)
		assert.InDelta(t, 0.4, *binding.MinScore, 0.0001)
		assert.Equal(t, 600, *binding.MaxTokens)
	})

	t.Run("ShouldRequireContext", func(t *testing.T) {
		var missingCtx context.Context
		_, err := NewBinding("kb-alpha").Build(missingCtx)
		require.Error(t, err)
		assert.EqualError(t, err, "context is required")
	})
}
