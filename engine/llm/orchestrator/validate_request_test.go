package orchestrator

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateRequest_NegativeCases(t *testing.T) {
	ctx := context.Background()
	t.Run("Should error when agent is nil", func(t *testing.T) {
		err := validateRequest(ctx, Request{Agent: nil, Action: &agent.ActionConfig{ID: "a", Prompt: "p"}})
		require.Error(t, err)
		var coreErr *core.Error
		require.ErrorAs(t, err, &coreErr)
		assert.Equal(t, ErrCodeInputValidation, coreErr.Code)
		assert.Equal(t, "agent", coreErr.Details["field"])
	})
	t.Run("Should error when action is nil", func(t *testing.T) {
		err := validateRequest(ctx, Request{Agent: &agent.Config{ID: "agent-1", Instructions: "i"}, Action: nil})
		require.Error(t, err)
		var coreErr *core.Error
		require.ErrorAs(t, err, &coreErr)
		assert.Equal(t, ErrCodeInputValidation, coreErr.Code)
		assert.Equal(t, "action", coreErr.Details["field"])
	})
	t.Run("Should error when agent instructions empty", func(t *testing.T) {
		err := validateRequest(
			ctx,
			Request{
				Agent:  &agent.Config{ID: "agent-1", Instructions: "   "},
				Action: &agent.ActionConfig{ID: "a", Prompt: "p"},
			},
		)
		require.Error(t, err)
		var coreErr *core.Error
		require.ErrorAs(t, err, &coreErr)
		assert.Equal(t, ErrCodeInputValidation, coreErr.Code)
		assert.Equal(t, "agent.instructions", coreErr.Details["field"])
	})
	t.Run("Should error when action prompt empty", func(t *testing.T) {
		err := validateRequest(
			ctx,
			Request{
				Agent:  &agent.Config{ID: "agent-1", Instructions: "ok"},
				Action: &agent.ActionConfig{ID: "a", Prompt: "  "},
			},
		)
		require.Error(t, err)
		var coreErr *core.Error
		require.ErrorAs(t, err, &coreErr)
		assert.Equal(t, ErrCodeInputValidation, coreErr.Code)
		assert.Equal(t, "action.prompt", coreErr.Details["field"])
	})
}
