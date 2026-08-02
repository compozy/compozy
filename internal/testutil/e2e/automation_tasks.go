package e2e

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/agentidentity"
	compozycontract "github.com/compozy/compozy/internal/api/contract"
	coreapi "github.com/compozy/compozy/internal/api/core"
	automationpkg "github.com/compozy/compozy/internal/automation"
)

// AutomationFixtureSeed describes one batch of automation definitions seeded
// through the live daemon operator surface.
type AutomationFixtureSeed struct {
	Jobs     []compozycontract.CreateJobRequest
	Triggers []compozycontract.CreateTriggerRequest
}

// AutomationFixtureState reports the created automation resources for one seed batch.
type AutomationFixtureState struct {
	Jobs     []compozycontract.JobPayload
	Triggers []compozycontract.TriggerPayload
}

// SeedAutomationFixtures creates the requested automation jobs and triggers
// through the public daemon UDS surface without mutating caller-supplied
// definitions.
func (h *RuntimeHarness) SeedAutomationFixtures(
	ctx context.Context,
	seed AutomationFixtureSeed,
) (AutomationFixtureState, error) {
	state := AutomationFixtureState{
		Jobs:     make([]compozycontract.JobPayload, 0, len(seed.Jobs)),
		Triggers: make([]compozycontract.TriggerPayload, 0, len(seed.Triggers)),
	}

	for _, request := range seed.Jobs {
		created, err := h.CreateAutomationJob(ctx, request)
		if err != nil {
			return AutomationFixtureState{}, err
		}
		state.Jobs = append(state.Jobs, created)
	}

	for _, request := range seed.Triggers {
		created, err := h.CreateAutomationTrigger(ctx, request)
		if err != nil {
			return AutomationFixtureState{}, err
		}
		state.Triggers = append(state.Triggers, created)
	}

	return state, nil
}

// CreateAutomationJob creates one automation job through the public UDS surface.
func (h *RuntimeHarness) CreateAutomationJob(
	ctx context.Context,
	request compozycontract.CreateJobRequest,
) (compozycontract.JobPayload, error) {
	var response compozycontract.JobResponse
	if err := h.UDSJSON(ctx, http.MethodPost, "/api/automation/jobs", request, &response); err != nil {
		return compozycontract.JobPayload{}, err
	}
	return response.Job, nil
}

// CreateAutomationTrigger creates one automation trigger through the public UDS surface.
func (h *RuntimeHarness) CreateAutomationTrigger(
	ctx context.Context,
	request compozycontract.CreateTriggerRequest,
) (compozycontract.TriggerPayload, error) {
	var response compozycontract.TriggerResponse
	if err := h.UDSJSON(ctx, http.MethodPost, "/api/automation/triggers", request, &response); err != nil {
		return compozycontract.TriggerPayload{}, err
	}
	return response.Trigger, nil
}

// TriggerAutomationJob forces one manual automation run through the public UDS surface.
func (h *RuntimeHarness) TriggerAutomationJob(
	ctx context.Context,
	jobID string,
) (compozycontract.RunPayload, error) {
	var response compozycontract.RunResponse
	path := "/api/automation/jobs/" + url.PathEscape(strings.TrimSpace(jobID)) + "/trigger"
	if err := h.UDSJSON(ctx, http.MethodPost, path, nil, &response); err != nil {
		return compozycontract.RunPayload{}, err
	}
	return response.Run, nil
}

// ListAutomationRuns fetches automation run history through the public UDS surface.
func (h *RuntimeHarness) ListAutomationRuns(
	ctx context.Context,
	query url.Values,
) ([]compozycontract.RunPayload, error) {
	var response compozycontract.RunsResponse
	if err := h.UDSJSON(ctx, http.MethodGet, "/api/automation/runs"+encodeQuery(query), nil, &response); err != nil {
		return nil, err
	}
	return response.Runs, nil
}

// GetAutomationRun fetches one automation run through the public UDS surface.
func (h *RuntimeHarness) GetAutomationRun(
	ctx context.Context,
	runID string,
) (compozycontract.RunPayload, error) {
	var response compozycontract.RunResponse
	path := "/api/automation/runs/" + url.PathEscape(strings.TrimSpace(runID))
	if err := h.UDSJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return compozycontract.RunPayload{}, err
	}
	return response.Run, nil
}

// ListTasks fetches lean task catalog rows through the public UDS surface.
func (h *RuntimeHarness) ListTasks(
	ctx context.Context,
	query url.Values,
) ([]compozycontract.TaskCatalogItemPayload, error) {
	var response compozycontract.TasksResponse
	if err := h.UDSJSON(ctx, http.MethodGet, "/api/tasks"+encodeQuery(query), nil, &response); err != nil {
		return nil, err
	}
	return response.Tasks, nil
}

// GetTask fetches one expanded task view through the public UDS surface.
func (h *RuntimeHarness) GetTask(
	ctx context.Context,
	taskID string,
) (compozycontract.TaskDetailPayload, error) {
	var response compozycontract.TaskDetailResponse
	path := "/api/tasks/" + url.PathEscape(strings.TrimSpace(taskID))
	if err := h.UDSJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return compozycontract.TaskDetailPayload{}, err
	}
	return response.Task, nil
}

// ListTaskRuns fetches one task's run history through the public UDS surface.
func (h *RuntimeHarness) ListTaskRuns(
	ctx context.Context,
	taskID string,
	query url.Values,
) ([]compozycontract.TaskRunPayload, error) {
	var response compozycontract.TaskRunsResponse
	path := "/api/tasks/" + url.PathEscape(strings.TrimSpace(taskID)) + "/runs" + encodeQuery(query)
	if err := h.UDSJSON(ctx, http.MethodGet, path, nil, &response); err != nil {
		return nil, err
	}
	return response.Runs, nil
}

// ClaimExactTaskRunForSession claims one exact queued run through the canonical agent lease surface.
func (h *RuntimeHarness) ClaimExactTaskRunForSession(
	ctx context.Context,
	runID string,
	session compozycontract.SessionPayload,
) (compozycontract.TaskRunPayload, error) {
	requestBody, err := json.Marshal(compozycontract.AgentTaskClaimNextRequest{
		RunID:       strings.TrimSpace(runID),
		WorkspaceID: strings.TrimSpace(session.WorkspaceID),
	})
	if err != nil {
		return compozycontract.TaskRunPayload{}, fmt.Errorf("marshal exact task-run claim: %w", err)
	}
	request, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		h.UDSURL("/api/agent/tasks/claim-next"),
		strings.NewReader(string(requestBody)),
	)
	if err != nil {
		return compozycontract.TaskRunPayload{}, fmt.Errorf("create exact task-run claim request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set(agentidentity.HeaderSessionID, strings.TrimSpace(session.ID))
	request.Header.Set(agentidentity.HeaderAgent, strings.TrimSpace(session.AgentName))

	response, err := h.UDSClient.Do(request)
	if err != nil {
		return compozycontract.TaskRunPayload{}, fmt.Errorf("execute exact task-run claim request: %w", err)
	}
	payload, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil {
		return compozycontract.TaskRunPayload{}, fmt.Errorf("read exact task-run claim response: %w", readErr)
	}
	if closeErr != nil {
		return compozycontract.TaskRunPayload{}, fmt.Errorf("close exact task-run claim response: %w", closeErr)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return compozycontract.TaskRunPayload{}, fmt.Errorf(
			"exact task-run claim status %d: %s",
			response.StatusCode,
			strings.TrimSpace(string(payload)),
		)
	}
	var claimed compozycontract.AgentTaskClaimResponse
	if err := json.Unmarshal(payload, &claimed); err != nil {
		return compozycontract.TaskRunPayload{}, fmt.Errorf("decode exact task-run claim response: %w", err)
	}
	return claimed.Claim.Run, nil
}

// StartTaskRun starts one claimed task run through the public UDS surface.
func (h *RuntimeHarness) StartTaskRun(
	ctx context.Context,
	runID string,
	request compozycontract.StartTaskRunRequest,
) (compozycontract.TaskRunPayload, error) {
	return h.updateTaskRun(ctx, runID, "/start", request)
}

// CompleteTaskRun completes one running task run through the public UDS surface.
func (h *RuntimeHarness) CompleteTaskRun(
	ctx context.Context,
	runID string,
	request compozycontract.CompleteTaskRunRequest,
) (compozycontract.TaskRunPayload, error) {
	return h.updateTaskRun(ctx, runID, "/complete", request)
}

// CompleteClaimedTaskRunForSession completes a claimed run through the canonical
// agent lease surface without exposing the raw claim token to the caller.
func (h *RuntimeHarness) CompleteClaimedTaskRunForSession(
	ctx context.Context,
	runID string,
	session compozycontract.SessionPayload,
	request compozycontract.AgentTaskCompleteRequest,
) (compozycontract.TaskRunLeaseSummaryPayload, error) {
	path := "/api/agent/tasks/" + url.PathEscape(strings.TrimSpace(runID)) + "/complete"
	response, err := doRequestWithHeaders(
		ctx,
		h.UDSClient,
		h.UDSURL(path),
		http.MethodPost,
		request,
		map[string]string{
			agentidentity.HeaderSessionID: session.ID,
			agentidentity.HeaderAgent:     session.AgentName,
		},
	)
	if err != nil {
		return compozycontract.TaskRunLeaseSummaryPayload{}, err
	}
	payload, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil {
		return compozycontract.TaskRunLeaseSummaryPayload{}, fmt.Errorf(
			"read claimed task-run completion response: %w",
			readErr,
		)
	}
	if closeErr != nil {
		return compozycontract.TaskRunLeaseSummaryPayload{}, fmt.Errorf(
			"close claimed task-run completion response: %w",
			closeErr,
		)
	}
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return compozycontract.TaskRunLeaseSummaryPayload{}, fmt.Errorf(
			"POST %s status %d: %s",
			h.UDSURL(path),
			response.StatusCode,
			strings.TrimSpace(string(payload)),
		)
	}
	var completed compozycontract.AgentTaskLeaseResponse
	if err := json.Unmarshal(payload, &completed); err != nil {
		return compozycontract.TaskRunLeaseSummaryPayload{}, fmt.Errorf(
			"decode claimed task-run completion response: %w",
			err,
		)
	}
	return completed.Lease, nil
}

// DeliverGlobalWebhook submits one signed global webhook through the public HTTP ingress.
func (h *RuntimeHarness) DeliverGlobalWebhook(
	ctx context.Context,
	endpoint string,
	secret string,
	payload []byte,
	deliveryID string,
	timestamp time.Time,
) (compozycontract.WebhookDeliveryPayload, error) {
	path := "/api/webhooks/global/" + url.PathEscape(strings.TrimSpace(endpoint))
	return h.deliverWebhook(ctx, path, secret, payload, deliveryID, timestamp)
}

// DeliverWorkspaceWebhook submits one signed workspace webhook through the public HTTP ingress.
func (h *RuntimeHarness) DeliverWorkspaceWebhook(
	ctx context.Context,
	workspaceID string,
	endpoint string,
	secret string,
	payload []byte,
	deliveryID string,
	timestamp time.Time,
) (compozycontract.WebhookDeliveryPayload, error) {
	workspaceRef := url.PathEscape(strings.TrimSpace(workspaceID))
	endpointRef := url.PathEscape(strings.TrimSpace(endpoint))
	path := "/api/webhooks/workspaces/" + workspaceRef + "/" + endpointRef
	return h.deliverWebhook(ctx, path, secret, payload, deliveryID, timestamp)
}

func (h *RuntimeHarness) updateTaskRun(
	ctx context.Context,
	runID string,
	actionPath string,
	request any,
) (compozycontract.TaskRunPayload, error) {
	var response compozycontract.TaskRunResponse
	path := "/api/task-runs/" + url.PathEscape(strings.TrimSpace(runID)) + actionPath
	if err := h.UDSJSON(ctx, http.MethodPost, path, request, &response); err != nil {
		return compozycontract.TaskRunPayload{}, err
	}
	return response.Run, nil
}

func (h *RuntimeHarness) deliverWebhook(
	ctx context.Context,
	path string,
	secret string,
	payload []byte,
	deliveryID string,
	timestamp time.Time,
) (_ compozycontract.WebhookDeliveryPayload, err error) {
	trimmedSecret := strings.TrimSpace(secret)
	if trimmedSecret == "" {
		return compozycontract.WebhookDeliveryPayload{}, errors.New("webhook secret is required")
	}
	if len(payload) == 0 {
		return compozycontract.WebhookDeliveryPayload{}, errors.New("webhook payload is required")
	}
	if strings.TrimSpace(deliveryID) == "" {
		return compozycontract.WebhookDeliveryPayload{}, errors.New("webhook delivery id is required")
	}
	if timestamp.IsZero() {
		return compozycontract.WebhookDeliveryPayload{}, errors.New("webhook timestamp is required")
	}

	signature, err := automationpkg.SignWebhookPayload(trimmedSecret, timestamp, payload)
	if err != nil {
		return compozycontract.WebhookDeliveryPayload{}, fmt.Errorf("sign webhook payload: %w", err)
	}

	targetURL := h.HTTPURL(path)
	response, err := doRequestWithHeaders(
		ctx,
		h.HTTPClient,
		targetURL,
		http.MethodPost,
		payload,
		map[string]string{
			coreapi.WebhookTimestampHeader:  timestamp.Format(time.RFC3339),
			coreapi.WebhookSignatureHeader:  signature,
			coreapi.WebhookDeliveryIDHeader: strings.TrimSpace(deliveryID),
		},
	)
	if err != nil {
		return compozycontract.WebhookDeliveryPayload{}, err
	}
	defer mergeHTTPResponseCloseError(&err, response, http.MethodPost, targetURL)

	body, err := io.ReadAll(response.Body)
	if err != nil {
		return compozycontract.WebhookDeliveryPayload{}, fmt.Errorf("read webhook response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return compozycontract.WebhookDeliveryPayload{}, fmt.Errorf(
			"POST %s status %d: %s",
			targetURL,
			response.StatusCode,
			strings.TrimSpace(string(body)),
		)
	}

	var delivery compozycontract.WebhookDeliveryResponse
	if err := json.Unmarshal(body, &delivery); err != nil {
		return compozycontract.WebhookDeliveryPayload{}, fmt.Errorf(
			"decode webhook response: %w; body=%s",
			err,
			strings.TrimSpace(string(body)),
		)
	}
	return delivery.Result, nil
}

func doRequestWithHeaders(
	ctx context.Context,
	client *http.Client,
	targetURL string,
	method string,
	body any,
	headers map[string]string,
) (*http.Response, error) {
	reader, err := requestBody(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, reader)
	if err != nil {
		return nil, fmt.Errorf("new request %s %s: %w", method, targetURL, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for key, value := range headers {
		trimmedKey := strings.TrimSpace(key)
		trimmedValue := strings.TrimSpace(value)
		if trimmedKey == "" || trimmedValue == "" {
			continue
		}
		req.Header.Set(trimmedKey, trimmedValue)
	}

	response, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("perform %s %s: %w", method, targetURL, err)
	}
	return response, nil
}
