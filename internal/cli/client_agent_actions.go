package cli

import (
	"context"

	"net/http"
	"net/url"

	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
)

func (c *daemonClient) AgentMe(
	ctx context.Context,
	credentials agentidentity.Credentials,
) (AgentMeRecord, error) {
	var response contract.AgentMeResponse
	if err := c.doAgentJSON(ctx, http.MethodGet, "/api/agent/me", nil, nil, credentials, &response); err != nil {
		return AgentMeRecord{}, err
	}
	return response.Me, nil
}

func (c *daemonClient) AgentContext(
	ctx context.Context,
	credentials agentidentity.Credentials,
) (AgentContextRecord, error) {
	var response contract.AgentContextResponse
	if err := c.doAgentJSON(ctx, http.MethodGet, "/api/agent/context", nil, nil, credentials, &response); err != nil {
		return AgentContextRecord{}, err
	}
	return response.Context, nil
}

func (c *daemonClient) AgentSpawn(
	ctx context.Context,
	request AgentSpawnRequest,
	credentials agentidentity.Credentials,
) (AgentSpawnRecord, error) {
	var response contract.AgentSpawnResponse
	if err := c.doAgentJSON(
		ctx,
		http.MethodPost,
		"/api/agent/spawn",
		nil,
		request,
		credentials,
		&response,
	); err != nil {
		return AgentSpawnRecord{}, err
	}
	return response.Spawn, nil
}

func (c *daemonClient) AgentTaskClaimNext(
	ctx context.Context,
	request AgentTaskClaimNextRequest,
	credentials agentidentity.Credentials,
) (_ AgentTaskNextRecord, err error) {
	const path = "/api/agent/tasks/claim-next"
	response, err := c.doRequestWithCredentials(ctx, http.MethodPost, path, nil, request, "", credentials)
	if err != nil {
		return AgentTaskNextRecord{}, err
	}
	defer mergeResponseBodyCloseError(&err, response, http.MethodPost, path)

	if response.StatusCode == http.StatusNoContent {
		if err := drainResponseBody(http.MethodPost, path, response.Body); err != nil {
			return AgentTaskNextRecord{}, err
		}
		return AgentTaskNextRecord{Claimed: false}, nil
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return AgentTaskNextRecord{}, readAPIError(response)
	}

	var decoded contract.AgentTaskClaimResponse
	if err := decodeJSONResponseBody(http.MethodPost, path, response.Body, &decoded); err != nil {
		return AgentTaskNextRecord{}, err
	}
	claim := decoded.Claim
	return AgentTaskNextRecord{Claimed: true, Claim: &claim}, nil
}

func (c *daemonClient) AgentTaskHeartbeat(
	ctx context.Context,
	runID string,
	request AgentTaskHeartbeatRequest,
	credentials agentidentity.Credentials,
) (AgentTaskLeaseRecord, error) {
	return c.agentTaskLeaseAction(ctx, strings.TrimSpace(runID), "heartbeat", request, credentials)
}

func (c *daemonClient) AgentTaskComplete(
	ctx context.Context,
	runID string,
	request AgentTaskCompleteRequest,
	credentials agentidentity.Credentials,
) (AgentTaskLeaseRecord, error) {
	return c.agentTaskLeaseAction(ctx, strings.TrimSpace(runID), "complete", request, credentials)
}

func (c *daemonClient) AgentTaskFail(
	ctx context.Context,
	runID string,
	request AgentTaskFailRequest,
	credentials agentidentity.Credentials,
) (AgentTaskLeaseRecord, error) {
	return c.agentTaskLeaseAction(ctx, strings.TrimSpace(runID), "fail", request, credentials)
}

func (c *daemonClient) AgentTaskRelease(
	ctx context.Context,
	runID string,
	request AgentTaskReleaseRequest,
	credentials agentidentity.Credentials,
) (AgentTaskLeaseRecord, error) {
	return c.agentTaskLeaseAction(ctx, strings.TrimSpace(runID), "release", request, credentials)
}

func (c *daemonClient) agentTaskLeaseAction(
	ctx context.Context,
	runID string,
	action string,
	request any,
	credentials agentidentity.Credentials,
) (AgentTaskLeaseRecord, error) {
	var response contract.AgentTaskLeaseResponse
	path := "/api/agent/tasks/" + url.PathEscape(runID) + "/" + strings.TrimSpace(action)
	if err := c.doAgentJSON(ctx, http.MethodPost, path, nil, request, credentials, &response); err != nil {
		return AgentTaskLeaseRecord{}, err
	}
	return response.Lease, nil
}

func (c *daemonClient) extensionAction(ctx context.Context, name string, action string) (ExtensionRecord, error) {
	var response struct {
		Extension ExtensionRecord `json:"extension"`
	}
	path := "/api/extensions/" + url.PathEscape(name) + "/" + action
	if err := c.doJSON(ctx, http.MethodPost, path, nil, nil, &response); err != nil {
		return ExtensionRecord{}, err
	}
	return response.Extension, nil
}

func (c *daemonClient) skillAction(
	ctx context.Context,
	name string,
	action string,
	query SkillQuery,
) (SkillActionRecord, error) {
	var response SkillActionRecord
	path := "/api/skills/" + url.PathEscape(name) + "/" + strings.TrimSpace(action)
	if err := c.doJSON(ctx, http.MethodPost, path, skillValues(query), nil, &response); err != nil {
		return SkillActionRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) taskRunAction(
	ctx context.Context,
	id string,
	action string,
	requestBody any,
) (TaskRunRecord, error) {
	var response contract.TaskRunResponse
	path := "/api/task-runs/" + url.PathEscape(id) + "/" + action
	if err := c.doJSON(ctx, http.MethodPost, path, nil, requestBody, &response); err != nil {
		return TaskRunRecord{}, err
	}
	return response.Run, nil
}

func (c *daemonClient) forceTaskRunAction(
	ctx context.Context,
	id string,
	action string,
	requestBody any,
) (TaskRunRecord, error) {
	var response contract.TaskRunResponse
	path := "/api/runs/" + url.PathEscape(id) + "/" + strings.TrimSpace(action)
	if err := c.doJSON(ctx, http.MethodPost, path, nil, requestBody, &response); err != nil {
		return TaskRunRecord{}, err
	}
	return response.Run, nil
}

func (c *daemonClient) bulkForceTaskRunAction(
	ctx context.Context,
	action string,
	requestBody BulkForceTaskRunRequest,
) (BulkForceTaskRunRecord, error) {
	var response contract.BulkForceTaskRunResponse
	path := "/api/runs/bulk/" + strings.TrimSpace(action)
	if err := c.doJSON(ctx, http.MethodPost, path, nil, requestBody, &response); err != nil {
		return BulkForceTaskRunRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) taskExecutionAction(
	ctx context.Context,
	id string,
	action string,
	requestBody TaskExecutionRequest,
) (TaskExecutionRecord, error) {
	var response contract.TaskExecutionResponse
	path := "/api/tasks/" + url.PathEscape(id) + "/" + strings.TrimSpace(action)
	if err := c.doJSON(ctx, http.MethodPost, path, nil, requestBody, &response); err != nil {
		return TaskExecutionRecord{}, err
	}
	return response, nil
}
