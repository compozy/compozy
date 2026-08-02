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

type LoopRunListQuery struct {
	LoopName      string
	Status        string
	Origin        string
	OriginSession string
	Limit         int
}

type LoopNodeListQuery struct {
	State    string
	LoopName string
	RunID    string
	Cursor   string
	Limit    int
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
	defer mergeResponseBodyCloseError(&err, response, http.MethodPost, path)

	if response.StatusCode == http.StatusUnprocessableEntity {
		if err := decodeJSONResponseBody(http.MethodPost, path, response.Body, &payload); err != nil {
			return contract.LoopValidationResponse{}, err
		}
		return payload, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return contract.LoopValidationResponse{}, readAPIError(response)
	}
	if err := decodeJSONResponseBody(http.MethodPost, path, response.Body, &payload); err != nil {
		return contract.LoopValidationResponse{}, err
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

func (c *unixSocketClient) CancelLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	credentials agentidentity.Credentials,
) (contract.LoopMutationResponse, error) {
	return c.loopRunLifecycleAction(ctx, workspaceID, runID, "cancel", nil, credentials)
}

func (c *unixSocketClient) KillLoopRun(
	ctx context.Context,
	workspaceID string,
	runID string,
	credentials agentidentity.Credentials,
) (contract.LoopMutationResponse, error) {
	return c.loopRunLifecycleAction(ctx, workspaceID, runID, "kill", nil, credentials)
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
	_, err := c.loopRunLifecycleAction(ctx, workspaceID, runID, action, request, credentials)
	return err
}

func (c *unixSocketClient) loopRunLifecycleAction(
	ctx context.Context,
	workspaceID string,
	runID string,
	action string,
	request any,
	credentials agentidentity.Credentials,
) (contract.LoopMutationResponse, error) {
	var response contract.LoopMutationResponse
	path := loopRunPath(workspaceID, runID) + "/" + strings.TrimSpace(action)
	if err := c.doAgentJSON(ctx, http.MethodPost, path, nil, request, credentials, &response); err != nil {
		return contract.LoopMutationResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ListLoopNodes(
	ctx context.Context,
	workspaceID string,
	query LoopNodeListQuery,
) (contract.LoopNodeInventoryResponse, error) {
	var response contract.LoopNodeInventoryResponse
	path := loopWorkspacePath(workspaceID) + "/loop-nodes"
	if err := c.doJSON(ctx, http.MethodGet, path, loopNodeValues(query), nil, &response); err != nil {
		return contract.LoopNodeInventoryResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) PauseLoopNode(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	request contract.LoopNodePauseRequest,
	credentials agentidentity.Credentials,
) (contract.LoopMutationResponse, error) {
	return c.loopNodeAction(ctx, workspaceID, runID, nodeID, "pause", request, credentials)
}

func (c *unixSocketClient) ResumeLoopNode(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	request contract.LoopNodeResumeRequest,
	credentials agentidentity.Credentials,
) (contract.LoopMutationResponse, error) {
	return c.loopNodeAction(ctx, workspaceID, runID, nodeID, "resume", request, credentials)
}

func (c *unixSocketClient) CancelLoopNode(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	request contract.LoopNodeMutationRequest,
	credentials agentidentity.Credentials,
) (contract.LoopMutationResponse, error) {
	return c.loopNodeAction(ctx, workspaceID, runID, nodeID, "cancel", request, credentials)
}

func (c *unixSocketClient) KillLoopNode(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	request contract.LoopNodeMutationRequest,
	credentials agentidentity.Credentials,
) (contract.LoopMutationResponse, error) {
	return c.loopNodeAction(ctx, workspaceID, runID, nodeID, "kill", request, credentials)
}

func (c *unixSocketClient) RequeueLoopNode(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	request contract.LoopNodeMutationRequest,
	credentials agentidentity.Credentials,
) (contract.LoopMutationResponse, error) {
	return c.loopNodeAction(ctx, workspaceID, runID, nodeID, "requeue", request, credentials)
}

func (c *unixSocketClient) loopNodeAction(
	ctx context.Context,
	workspaceID string,
	runID string,
	nodeID string,
	action string,
	request any,
	credentials agentidentity.Credentials,
) (contract.LoopMutationResponse, error) {
	var response contract.LoopMutationResponse
	path := loopRunPath(workspaceID, runID) + "/nodes/" + url.PathEscape(strings.TrimSpace(nodeID)) +
		"/" + strings.TrimSpace(action)
	if err := c.doAgentJSON(ctx, http.MethodPost, path, nil, request, credentials, &response); err != nil {
		return contract.LoopMutationResponse{}, err
	}
	return response, nil
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

func loopNodeValues(query LoopNodeListQuery) url.Values {
	values := url.Values{}
	setLoopListQueryValue(values, "state", query.State)
	setLoopListQueryValue(values, "loop", query.LoopName)
	setLoopListQueryValue(values, "run_id", query.RunID)
	setLoopListQueryValue(values, "cursor", query.Cursor)
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
