package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	enginecore "github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/engine/task/tasks/core"
	"github.com/compozy/compozy/engine/task/tasks/shared"
	"github.com/compozy/compozy/pkg/tplengine"
)

func TestTransitionNormalizer_Normalize(t *testing.T) {
	t.Run("Should handle nil transition", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		normalizer := core.NewTransitionNormalizer[*enginecore.SuccessTransition](templateEngine)
		ctx := &shared.NormalizationContext{}
		// Act
		err := normalizer.Normalize(nil, ctx)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should normalize success transition with templates", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		normalizer := core.NewTransitionNormalizer[*enginecore.SuccessTransition](templateEngine)
		next := "{{ .next_task }}"
		transition := &enginecore.SuccessTransition{
			Next: &next,
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"next_task": "processed_task",
			},
		}
		// Act
		err := normalizer.Normalize(transition, ctx)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, transition.Next)
		assert.Equal(t, "processed_task", *transition.Next)
	})

	t.Run("Should normalize error transition with templates", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		normalizer := core.NewTransitionNormalizer[*enginecore.ErrorTransition](templateEngine)
		next := "{{ .error_handler }}"
		transition := &enginecore.ErrorTransition{
			Next: &next,
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"error_handler": "error_task",
			},
		}
		// Act
		err := normalizer.Normalize(transition, ctx)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, transition.Next)
		assert.Equal(t, "error_task", *transition.Next)
	})

	t.Run("Should set current input from transition when ctx input is nil", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		normalizer := core.NewTransitionNormalizer[*enginecore.SuccessTransition](templateEngine)
		input := enginecore.NewInput(map[string]any{"key": "value"})
		transition := &enginecore.SuccessTransition{
			With: &input,
		}
		ctx := &shared.NormalizationContext{
			CurrentInput: nil,
		}
		// Act
		err := normalizer.Normalize(transition, ctx)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, ctx.CurrentInput)
		assert.Equal(t, &input, ctx.CurrentInput)
	})

	t.Run("Should not override existing current input", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		normalizer := core.NewTransitionNormalizer[*enginecore.SuccessTransition](templateEngine)
		existingInput := enginecore.NewInput(map[string]any{"existing": "input"})
		transitionInput := enginecore.NewInput(map[string]any{"transition": "input"})
		transition := &enginecore.SuccessTransition{
			With: &transitionInput,
		}
		ctx := &shared.NormalizationContext{
			CurrentInput: &existingInput,
		}
		// Act
		err := normalizer.Normalize(transition, ctx)
		// Assert
		assert.NoError(t, err)
		assert.Equal(t, &existingInput, ctx.CurrentInput)
	})

	t.Run("Should handle transition with 'with' input template", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		normalizer := core.NewTransitionNormalizer[*enginecore.SuccessTransition](templateEngine)
		transition := &enginecore.SuccessTransition{
			With: &enginecore.Input{
				"processed_field": "{{ .data }}_processed",
			},
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"data": "test_data",
			},
		}
		// Act
		err := normalizer.Normalize(transition, ctx)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, transition.With)
		assert.Equal(t, "test_data_processed", (*transition.With)["processed_field"])
	})
}

func TestSuccessTransitionNormalizer_Normalize(t *testing.T) {
	t.Run("Should return nil for nil transition", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		normalizer := core.NewSuccessTransitionNormalizer(templateEngine)
		ctx := &shared.NormalizationContext{}
		// Act
		err := normalizer.Normalize(nil, ctx)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should normalize success transition", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		normalizer := core.NewSuccessTransitionNormalizer(templateEngine)
		next := "{{ .workflow_id }}_success"
		transition := &enginecore.SuccessTransition{
			Next: &next,
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"workflow_id": "test_workflow",
			},
		}
		// Act
		err := normalizer.Normalize(transition, ctx)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, transition.Next)
		assert.Equal(t, "test_workflow_success", *transition.Next)
	})

	t.Run("Should set current input from transition", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		normalizer := core.NewSuccessTransitionNormalizer(templateEngine)
		input := enginecore.NewInput(map[string]any{"success": "data"})
		transition := &enginecore.SuccessTransition{
			With: &input,
		}
		ctx := &shared.NormalizationContext{
			CurrentInput: nil,
		}
		// Act
		err := normalizer.Normalize(transition, ctx)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, ctx.CurrentInput)
		assert.Equal(t, &input, ctx.CurrentInput)
	})
}

func TestErrorTransitionNormalizer_Normalize(t *testing.T) {
	t.Run("Should return nil for nil transition", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		normalizer := core.NewErrorTransitionNormalizer(templateEngine)
		ctx := &shared.NormalizationContext{}
		// Act
		err := normalizer.Normalize(nil, ctx)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should normalize error transition", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		normalizer := core.NewErrorTransitionNormalizer(templateEngine)
		next := "{{ .workflow_id }}_error_handler"
		transition := &enginecore.ErrorTransition{
			Next: &next,
		}
		ctx := &shared.NormalizationContext{
			Variables: map[string]any{
				"workflow_id": "test_workflow",
			},
		}
		// Act
		err := normalizer.Normalize(transition, ctx)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, transition.Next)
		assert.Equal(t, "test_workflow_error_handler", *transition.Next)
	})

	t.Run("Should set current input from transition", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		normalizer := core.NewErrorTransitionNormalizer(templateEngine)
		input := enginecore.NewInput(map[string]any{"error": "context"})
		transition := &enginecore.ErrorTransition{
			With: &input,
		}
		ctx := &shared.NormalizationContext{
			CurrentInput: nil,
		}
		// Act
		err := normalizer.Normalize(transition, ctx)
		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, ctx.CurrentInput)
		assert.Equal(t, &input, ctx.CurrentInput)
	})
}

func TestIsNil(t *testing.T) {
	t.Run("Should return true for nil interface", func(t *testing.T) {
		// This tests the isNil function indirectly through the generic normalizer
		templateEngine := &tplengine.TemplateEngine{}
		normalizer := core.NewTransitionNormalizer[*enginecore.SuccessTransition](templateEngine)
		ctx := &shared.NormalizationContext{}
		// Act
		err := normalizer.Normalize(nil, ctx)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should handle typed nil pointer for SuccessTransition", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		normalizer := core.NewTransitionNormalizer[*enginecore.SuccessTransition](templateEngine)
		var transition *enginecore.SuccessTransition
		ctx := &shared.NormalizationContext{}
		// Act
		err := normalizer.Normalize(transition, ctx)
		// Assert
		assert.NoError(t, err)
	})

	t.Run("Should handle typed nil pointer for ErrorTransition", func(t *testing.T) {
		// Arrange
		templateEngine := &tplengine.TemplateEngine{}
		normalizer := core.NewTransitionNormalizer[*enginecore.ErrorTransition](templateEngine)
		var transition *enginecore.ErrorTransition
		ctx := &shared.NormalizationContext{}
		// Act
		err := normalizer.Normalize(transition, ctx)
		// Assert
		assert.NoError(t, err)
	})
}
