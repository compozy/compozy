package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
)

type LoopRunListQuery struct {
	LoopName      string
	Status        string
	Origin        string
	OriginSession string
	Limit         int
}

type GoalTurnListQuery struct {
	NodeID    string
	ItemIndex *int
	AfterSeq  int64
	Limit     int
}

type LoopListQuery struct {
	Search   string
	Kind     string
	Category string
	Status   string
	Sort     string
	Cursor   string
	Limit    int
}

func (c *unixSocketClient) ListLoops(
	ctx context.Context,
	workspaceID string,
	query LoopListQuery,
) (contract.LoopsResponse, error) {
	var response contract.LoopsResponse
	path := loopWorkspacePath(workspaceID) + "/loops"
	if err := c.doJSON(ctx, http.MethodGet, path, loopListValues(query), nil, &response); err != nil {
		return contract.LoopsResponse{}, err
	}
	return response, nil
}

func loopListValues(query LoopListQuery) url.Values {
	values := url.Values{}
	setLoopListQueryValue(values, "q", query.Search)
	setLoopListQueryValue(values, "kind", query.Kind)
	setLoopListQueryValue(values, "category", query.Category)
	setLoopListQueryValue(values, "status", query.Status)
	setLoopListQueryValue(values, "sort", query.Sort)
	setLoopListQueryValue(values, "cursor", query.Cursor)
	if query.Limit != 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}

func setLoopListQueryValue(values url.Values, key string, value string) {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		values.Set(key, trimmed)
	}
}

func (c *unixSocketClient) CreateLoop(
	ctx context.Context,
	workspaceID string,
	request contract.CreateLoopRequest,
	credentials agentidentity.Credentials,
) (contract.LoopResponse, error) {
	var response contract.LoopResponse
	path := loopWorkspacePath(workspaceID) + "/loops"
	if err := c.doAgentJSON(ctx, http.MethodPost, path, nil, request, credentials, &response); err != nil {
		return contract.LoopResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) GetLoop(
	ctx context.Context,
	workspaceID string,
	name string,
) (contract.LoopResponse, error) {
	var response contract.LoopResponse
	path := loopDefinitionPath(workspaceID, name)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return contract.LoopResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) PatchLoop(
	ctx context.Context,
	workspaceID string,
	name string,
	request contract.PatchLoopRequest,
	credentials agentidentity.Credentials,
) (contract.LoopResponse, error) {
	var response contract.LoopResponse
	if err := c.doAgentJSON(
		ctx,
		http.MethodPatch,
		loopDefinitionPath(workspaceID, name),
		nil,
		request,
		credentials,
		&response,
	); err != nil {
		return contract.LoopResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ValidateLoop(
	ctx context.Context,
	workspaceID string,
	name string,
	request contract.ValidateLoopRequest,
) (payload contract.LoopValidationResponse, err error) {
	path := loopDefinitionPath(workspaceID, name) + "/validate"
	response, err := c.doRequest(ctx, http.MethodPost, path, nil, request)
	if err != nil {
		return contract.LoopValidationResponse{}, err
	}
	defer func() {
		if closeErr := response.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("cli: close %s response: %w", path, closeErr)
		}
	}()

	if response.StatusCode == http.StatusUnprocessableEntity {
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			return contract.LoopValidationResponse{}, fmt.Errorf("cli: decode %s response: %w", path, err)
		}
		return payload, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return contract.LoopValidationResponse{}, readAPIError(response)
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return contract.LoopValidationResponse{}, fmt.Errorf("cli: decode %s response: %w", path, err)
	}
	return payload, nil
}

func (c *unixSocketClient) DeleteLoop(
	ctx context.Context,
	workspaceID string,
	name string,
	credentials agentidentity.Credentials,
) error {
	return c.doAgentJSON(ctx, http.MethodDelete, loopDefinitionPath(workspaceID, name), nil, nil, credentials, nil)
}

func (c *unixSocketClient) RunLoop(
	ctx context.Context,
	workspaceID string,
	name string,
	request contract.RunLoopRequest,
	dry bool,
	credentials agentidentity.Credentials,
) (contract.RunLoopResponse, error) {
	var response contract.RunLoopResponse
	query := url.Values{}
	if dry {
		query.Set("dry", "true")
	}
	path := loopDefinitionPath(workspaceID, name) + "/run"
	if err := c.doAgentJSON(ctx, http.MethodPost, path, query, request, credentials, &response); err != nil {
		return contract.RunLoopResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) GetLoopConfig(
	ctx context.Context,
	workspaceID string,
	name string,
) (contract.LoopConfigResponse, error) {
	var response contract.LoopConfigResponse
	path := loopDefinitionPath(workspaceID, name) + "/config"
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return contract.LoopConfigResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) PutLoopConfig(
	ctx context.Context,
	workspaceID string,
	name string,
	request contract.PutLoopConfigRequest,
	credentials agentidentity.Credentials,
) (contract.LoopConfigResponse, error) {
	var response contract.LoopConfigResponse
	path := loopDefinitionPath(workspaceID, name) + "/config"
	if err := c.doAgentJSON(ctx, http.MethodPut, path, nil, request, credentials, &response); err != nil {
		return contract.LoopConfigResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ListLoopRuns(
	ctx context.Context,
	workspaceID string,
	query LoopRunListQuery,
) (contract.LoopRunsResponse, error) {
	var response contract.LoopRunsResponse
	path := loopWorkspacePath(workspaceID) + "/loop-runs"
	if err := c.doJSON(ctx, http.MethodGet, path, loopRunValues(query), nil, &response); err != nil {
		return contract.LoopRunsResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ListGoalTurns(
	ctx context.Context,
	workspaceID string,
	runID string,
	query GoalTurnListQuery,
) (contract.GoalTurnPage, error) {
	var response contract.GoalTurnPage
	path := loopRunPath(workspaceID, runID) + "/turns"
	if err := c.doJSON(ctx, http.MethodGet, path, goalTurnValues(query), nil, &response); err != nil {
		return contract.GoalTurnPage{}, err
	}
	return response, nil
}

func (c *unixSocketClient) GetLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
) (contract.LoopRunResponse, error) {
	var response contract.LoopRunResponse
	path := loopRunPath(workspaceID, runID)
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return contract.LoopRunResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) StopLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	credentials agentidentity.Credentials,
) error {
	return c.loopRunAction(ctx, workspaceID, runID, "stop", nil, credentials)
}

func (c *unixSocketClient) PauseLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	credentials agentidentity.Credentials,
) error {
	return c.loopRunAction(ctx, workspaceID, runID, "pause", nil, credentials)
}

func (c *unixSocketClient) ResumeLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	credentials agentidentity.Credentials,
) error {
	return c.loopRunAction(ctx, workspaceID, runID, "resume", nil, credentials)
}

func (c *unixSocketClient) ApproveLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	request contract.ApproveLoopRunRequest,
	credentials agentidentity.Credentials,
) error {
	return c.loopRunAction(ctx, workspaceID, runID, "approve", request, credentials)
}

func (c *unixSocketClient) loopRunAction(
	ctx context.Context,
	workspaceID string,
	runID string,
	action string,
	request any,
	credentials agentidentity.Credentials,
) error {
	path := loopRunPath(workspaceID, runID) + "/" + strings.TrimSpace(action)
	return c.doAgentJSON(ctx, http.MethodPost, path, nil, request, credentials, nil)
}

func loopWorkspacePath(workspaceID string) string {
	return "/api/workspaces/" + url.PathEscape(strings.TrimSpace(workspaceID))
}

func loopDefinitionPath(workspaceID string, name string) string {
	return loopWorkspacePath(workspaceID) + "/loops/" + url.PathEscape(strings.TrimSpace(name))
}

func loopRunPath(workspaceID string, runID string) string {
	return loopWorkspacePath(workspaceID) + "/loop-runs/" + url.PathEscape(strings.TrimSpace(runID))
}

func loopRunValues(query LoopRunListQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.LoopName); trimmed != "" {
		values.Set("loop", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Status); trimmed != "" {
		values.Set("status", trimmed)
	}
	if trimmed := strings.TrimSpace(query.Origin); trimmed != "" {
		values.Set("origin", trimmed)
	}
	if trimmed := strings.TrimSpace(query.OriginSession); trimmed != "" {
		values.Set("origin_session", trimmed)
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}

func goalTurnValues(query GoalTurnListQuery) url.Values {
	values := url.Values{}
	if trimmed := strings.TrimSpace(query.NodeID); trimmed != "" {
		values.Set("node", trimmed)
	}
	if query.ItemIndex != nil {
		values.Set("item", strconv.Itoa(*query.ItemIndex))
	}
	if query.AfterSeq > 0 {
		values.Set("after_seq", strconv.FormatInt(query.AfterSeq, 10))
	}
	if query.Limit > 0 {
		values.Set("limit", strconv.Itoa(query.Limit))
	}
	return values
}
