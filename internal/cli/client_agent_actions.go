package cli

import (
	"context"
	"encoding/json"

	"fmt"

	"net/http"
	"net/url"

	"strings"

	"github.com/compozy/agh/internal/agentidentity"
	"github.com/compozy/agh/internal/api/contract"
)

func (c *unixSocketClient) AgentMe(
	ctx context.Context,
	credentials agentidentity.Credentials,
) (AgentMeRecord, error) {
	var response contract.AgentMeResponse
	if err := c.doAgentJSON(ctx, http.MethodGet, "/api/agent/me", nil, nil, credentials, &response); err != nil {
		return AgentMeRecord{}, err
	}
	return response.Me, nil
}

func (c *unixSocketClient) AgentContext(
	ctx context.Context,
	credentials agentidentity.Credentials,
) (AgentContextRecord, error) {
	var response contract.AgentContextResponse
	if err := c.doAgentJSON(ctx, http.MethodGet, "/api/agent/context", nil, nil, credentials, &response); err != nil {
		return AgentContextRecord{}, err
	}
	return response.Context, nil
}

func (c *unixSocketClient) AgentSpawn(
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

func (c *unixSocketClient) AgentChannels(
	ctx context.Context,
	credentials agentidentity.Credentials,
) ([]AgentChannelRecord, error) {
	var response contract.AgentChannelsResponse
	if err := c.doAgentJSON(ctx, http.MethodGet, "/api/agent/channels", nil, nil, credentials, &response); err != nil {
		return nil, err
	}
	return response.Channels, nil
}

func (c *unixSocketClient) AgentChannelRecv(
	ctx context.Context,
	channel string,
	query AgentChannelRecvQuery,
	credentials agentidentity.Credentials,
) ([]AgentChannelMessageRecord, error) {
	var response contract.AgentChannelMessagesResponse
	path := "/api/agent/channels/" + url.PathEscape(strings.TrimSpace(channel)) + "/recv"
	if err := c.doAgentJSON(
		ctx,
		http.MethodGet,
		path,
		agentChannelRecvValues(query),
		nil,
		credentials,
		&response,
	); err != nil {
		return nil, err
	}
	return response.Messages, nil
}

func (c *unixSocketClient) AgentChannelSend(
	ctx context.Context,
	channel string,
	request AgentChannelSendRequest,
	credentials agentidentity.Credentials,
) (AgentChannelMessageRecord, error) {
	var response contract.AgentChannelMessageResponse
	path := "/api/agent/channels/" + url.PathEscape(strings.TrimSpace(channel)) + "/send"
	if err := c.doAgentJSON(
		ctx,
		http.MethodPost,
		path,
		nil,
		request,
		credentials,
		&response,
	); err != nil {
		return AgentChannelMessageRecord{}, err
	}
	return response.Message, nil
}

func (c *unixSocketClient) AgentChannelReply(
	ctx context.Context,
	request AgentChannelReplyRequest,
	credentials agentidentity.Credentials,
) (AgentChannelMessageRecord, error) {
	var response contract.AgentChannelMessageResponse
	if err := c.doAgentJSON(
		ctx,
		http.MethodPost,
		"/api/agent/channels/reply",
		nil,
		request,
		credentials,
		&response,
	); err != nil {
		return AgentChannelMessageRecord{}, err
	}
	return response.Message, nil
}

func (c *unixSocketClient) AgentTaskClaimNext(
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
	if err := json.NewDecoder(response.Body).Decode(&decoded); err != nil {
		return AgentTaskNextRecord{}, fmt.Errorf("cli: decode %s %s response: %w", http.MethodPost, path, err)
	}
	claim := decoded.Claim
	return AgentTaskNextRecord{Claimed: true, Claim: &claim}, nil
}

func (c *unixSocketClient) AgentTaskHeartbeat(
	ctx context.Context,
	runID string,
	request AgentTaskHeartbeatRequest,
	credentials agentidentity.Credentials,
) (AgentTaskLeaseRecord, error) {
	return c.agentTaskLeaseAction(ctx, strings.TrimSpace(runID), "heartbeat", request, credentials)
}

func (c *unixSocketClient) AgentTaskComplete(
	ctx context.Context,
	runID string,
	request AgentTaskCompleteRequest,
	credentials agentidentity.Credentials,
) (AgentTaskLeaseRecord, error) {
	return c.agentTaskLeaseAction(ctx, strings.TrimSpace(runID), "complete", request, credentials)
}

func (c *unixSocketClient) AgentTaskFail(
	ctx context.Context,
	runID string,
	request AgentTaskFailRequest,
	credentials agentidentity.Credentials,
) (AgentTaskLeaseRecord, error) {
	return c.agentTaskLeaseAction(ctx, strings.TrimSpace(runID), "fail", request, credentials)
}

func (c *unixSocketClient) AgentTaskRelease(
	ctx context.Context,
	runID string,
	request AgentTaskReleaseRequest,
	credentials agentidentity.Credentials,
) (AgentTaskLeaseRecord, error) {
	return c.agentTaskLeaseAction(ctx, strings.TrimSpace(runID), "release", request, credentials)
}

func (c *unixSocketClient) agentTaskLeaseAction(
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

func (c *unixSocketClient) extensionAction(ctx context.Context, name string, action string) (ExtensionRecord, error) {
	var response struct {
		Extension ExtensionRecord `json:"extension"`
	}
	path := "/api/extensions/" + url.PathEscape(name) + "/" + action
	if err := c.doJSON(ctx, http.MethodPost, path, nil, nil, &response); err != nil {
		return ExtensionRecord{}, err
	}
	return response.Extension, nil
}

func (c *unixSocketClient) bridgeAction(ctx context.Context, id string, action string) (BridgeRecord, error) {
	var response struct {
		Bridge BridgeRecord `json:"bridge"`
	}
	path := "/api/bridges/" + url.PathEscape(id) + "/" + action
	if err := c.doJSON(ctx, http.MethodPost, path, nil, nil, &response); err != nil {
		return BridgeRecord{}, err
	}
	return response.Bridge, nil
}

func (c *unixSocketClient) skillAction(
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

func (c *unixSocketClient) taskRunAction(
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

func (c *unixSocketClient) forceTaskRunAction(
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

func (c *unixSocketClient) bulkForceTaskRunAction(
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

func (c *unixSocketClient) taskExecutionAction(
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
