package orchestrator

import (
	"context"
	"testing"

	"github.com/compozy/compozy/engine/agent"
	"github.com/stretchr/testify/assert"
)

func TestValidateRequest_NegativeCases(t *testing.T) {
	ctx := context.Background()
	t.Run("Should error when agent is nil", func(t *testing.T) {
		err := validateRequest(ctx, Request{Agent: nil, Action: &agent.ActionConfig{ID: "a", Prompt: "p"}})
		assert.Error(t, err)
	})
	t.Run("Should error when action is nil", func(t *testing.T) {
		err := validateRequest(ctx, Request{Agent: &agent.Config{Instructions: "i"}, Action: nil})
		assert.Error(t, err)
	})
	t.Run("Should error when agent instructions empty", func(t *testing.T) {
		err := validateRequest(
			ctx,
			Request{Agent: &agent.Config{Instructions: "   "}, Action: &agent.ActionConfig{ID: "a", Prompt: "p"}},
		)
		assert.Error(t, err)
	})
	t.Run("Should error when action prompt empty", func(t *testing.T) {
		err := validateRequest(
			ctx,
			Request{Agent: &agent.Config{Instructions: "ok"}, Action: &agent.ActionConfig{ID: "a", Prompt: "  "}},
		)
		assert.Error(t, err)
	})
}
