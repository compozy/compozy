package agentaction

import (
	"context"
	"fmt"
	"strings"

	engineagent "github.com/compozy/compozy/engine/agent"
	"github.com/compozy/compozy/engine/core"
	"github.com/compozy/compozy/pkg/logger"
	sdkerrors "github.com/compozy/compozy/sdk/internal/errors"
	"github.com/compozy/compozy/sdk/internal/validate"
)

// New creates an action configuration using functional options.
//
// The action ID must be provided as it uniquely identifies the action within
// an agent's scope. All other configuration is applied through options.
//
// **Example:**
//
//	action, err := agentaction.New(ctx, "review-code",
//	    agentaction.WithPrompt("Analyze code for quality and improvements"),
//	    agentaction.WithTools([]enginetool.Config{{ID: "file-reader"}}),
//	    agentaction.WithTimeout("30s"),
//	)
func New(ctx context.Context, id string, opts ...Option) (*engineagent.ActionConfig, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context is required")
	}
	log := logger.FromContext(ctx)
	log.Debug("creating action configuration", "action", id)
	cfg := &engineagent.ActionConfig{
		ID: strings.TrimSpace(id),
	}
	for _, opt := range opts {
		opt(cfg)
	}
	collected := make([]error, 0)
	if err := validate.ID(ctx, cfg.ID); err != nil {
		collected = append(collected, fmt.Errorf("action id is invalid: %w", err))
	}
	cfg.Prompt = strings.TrimSpace(cfg.Prompt)
	if err := validate.NonEmpty(ctx, "prompt", cfg.Prompt); err != nil {
		collected = append(collected, err)
	}
	if len(collected) > 0 {
		return nil, &sdkerrors.BuildError{Errors: collected}
	}
	cloned, err := core.DeepCopy(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to clone action config: %w", err)
	}
	return cloned, nil
}
