package cli

import (
	"context"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
)

func (c *daemonClient) MutateSessionGoal(
	ctx context.Context,
	id string,
	request contract.SessionGoalCommandRequest,
) (contract.GoalCommandResult, error) {
	var response contract.GoalCommandResult
	path, err := c.sessionScopedPath(ctx, id, "/goal")
	if err != nil {
		return contract.GoalCommandResult{}, err
	}
	if err := c.doJSON(ctx, http.MethodPost, path, nil, request, &response); err != nil {
		return contract.GoalCommandResult{}, err
	}
	return response, nil
}
