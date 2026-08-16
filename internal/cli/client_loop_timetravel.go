package cli

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
)

func (c *daemonClient) DiffLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	generation int64,
	againstGeneration int64,
	againstRunID string,
) (contract.LoopDiffResponse, error) {
	var response contract.LoopDiffResponse
	values := url.Values{}
	if generation > 0 {
		values.Set("generation", strconv.FormatInt(generation, 10))
	}
	if againstGeneration > 0 {
		values.Set("against_generation", strconv.FormatInt(againstGeneration, 10))
	}
	if strings.TrimSpace(againstRunID) != "" {
		values.Set("against_run", strings.TrimSpace(againstRunID))
	}
	if err := c.doJSON(ctx, http.MethodGet, loopRunPath(workspaceID, runID)+"/diff", values, nil, &response); err != nil {
		return contract.LoopDiffResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) RerunLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	request contract.RerunLoopRequest,
	credentials agentidentity.Credentials,
) (contract.RerunLoopResponse, error) {
	var response contract.RerunLoopResponse
	if err := c.doAgentJSON(ctx, http.MethodPost, loopRunPath(workspaceID, runID)+"/rerun",
		nil, request, credentials, &response); err != nil {
		return contract.RerunLoopResponse{}, err
	}
	return response, nil
}

func (c *daemonClient) ForkLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	request contract.ForkLoopRequest,
	credentials agentidentity.Credentials,
) (contract.ForkLoopResponse, error) {
	var response contract.ForkLoopResponse
	if err := c.doAgentJSON(ctx, http.MethodPost, loopRunPath(workspaceID, runID)+"/fork",
		nil, request, credentials, &response); err != nil {
		return contract.ForkLoopResponse{}, err
	}
	return response, nil
}
