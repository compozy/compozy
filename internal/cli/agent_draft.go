package cli

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/spf13/cobra"
)

type agentDraftClient interface {
	DraftAgentInstructions(context.Context, contract.DraftAgentInstructionsRequest) (contract.DraftAgentInstructionsResponse, error)
}

var _ agentDraftClient = (*daemonClient)(nil)

func (c *daemonClient) DraftAgentInstructions(ctx context.Context, request contract.DraftAgentInstructionsRequest) (contract.DraftAgentInstructionsResponse, error) {
	var result contract.DraftAgentInstructionsResponse
	err := c.doJSON(ctx, http.MethodPost, "/api/agents/draft-instructions", nil, request, &result)
	return result, err
}

func newAgentDraftCommand(deps commandDeps) *cobra.Command {
	var runtime contract.AgentDraftRuntime
	cmd := &cobra.Command{Use: "draft-instructions <goal>", Short: "Draft agent instructions without saving a definition", Args: exactOneNonBlankArg()}
	cmd.Flags().StringVar(&runtime.Provider, "provider", "", "Provider for the draft")
	cmd.Flags().StringVar(&runtime.Model, "model", "", "Model for the draft")
	cmd.Flags().StringVar(&runtime.ReasoningEffort, "reasoning-effort", "", "Reasoning effort for the draft")
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		if strings.TrimSpace(runtime.Provider) == "" {
			return errors.New("cli: --provider is required")
		}
		base, err := clientFromDeps(deps)
		if err != nil {
			return err
		}
		client, ok := base.(agentDraftClient)
		if !ok {
			return errors.New("cli: instruction drafting unavailable")
		}
		result, err := client.DraftAgentInstructions(cmd.Context(), contract.DraftAgentInstructionsRequest{Goal: args[0], Runtime: runtime})
		if err != nil {
			return err
		}
		return writeInstanceOutput(cmd, result, result.Instructions+"\n")
	}
	return cmd
}
