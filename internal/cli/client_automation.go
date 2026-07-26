package cli

import (
	"context"

	"net/http"
	"net/url"

	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

func (c *unixSocketClient) ListAutomationJobs(
	ctx context.Context,
	query AutomationJobQuery,
) (AutomationJobListRecord, error) {
	var response contract.JobsResponse
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/automation/jobs",
		automationJobValues(query),
		nil,
		&response,
	); err != nil {
		return AutomationJobListRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) CreateAutomationJob(
	ctx context.Context,
	request AutomationJobCreateRequest,
) (JobRecord, error) {
	var response contract.JobResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/automation/jobs", nil, request, &response); err != nil {
		return JobRecord{}, err
	}
	return response.Job, nil
}

func (c *unixSocketClient) GetAutomationJob(ctx context.Context, id string) (JobRecord, error) {
	var response contract.JobResponse
	path := "/api/automation/jobs/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return JobRecord{}, err
	}
	return response.Job, nil
}

func (c *unixSocketClient) UpdateAutomationJob(
	ctx context.Context,
	id string,
	request AutomationJobUpdateRequest,
) (JobRecord, error) {
	var response contract.JobResponse
	path := "/api/automation/jobs/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodPatch, path, nil, request, &response); err != nil {
		return JobRecord{}, err
	}
	return response.Job, nil
}

func (c *unixSocketClient) DeleteAutomationJob(ctx context.Context, id string) error {
	path := "/api/automation/jobs/" + url.PathEscape(strings.TrimSpace(id))
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, nil)
}

func (c *unixSocketClient) TriggerAutomationJob(ctx context.Context, id string) (RunRecord, error) {
	var response contract.RunResponse
	path := "/api/automation/jobs/" + url.PathEscape(strings.TrimSpace(id)) + "/trigger"
	if err := c.doJSON(ctx, http.MethodPost, path, nil, nil, &response); err != nil {
		return RunRecord{}, err
	}
	return response.Run, nil
}

func (c *unixSocketClient) AutomationJobRuns(
	ctx context.Context,
	id string,
	query AutomationRunQuery,
) ([]RunRecord, error) {
	var response contract.RunsResponse
	path := "/api/automation/jobs/" + url.PathEscape(strings.TrimSpace(id)) + "/runs"
	if err := c.doJSON(ctx, http.MethodGet, path, automationRunValues(query), nil, &response); err != nil {
		return nil, err
	}
	return response.Runs, nil
}

func (c *unixSocketClient) ListAutomationTriggers(
	ctx context.Context,
	query AutomationTriggerQuery,
) (AutomationTriggerListRecord, error) {
	var response contract.TriggersResponse
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/automation/triggers",
		automationTriggerValues(query),
		nil,
		&response,
	); err != nil {
		return AutomationTriggerListRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) CreateAutomationTrigger(
	ctx context.Context,
	request AutomationTriggerCreateRequest,
) (TriggerRecord, error) {
	var response contract.TriggerResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/automation/triggers", nil, request, &response); err != nil {
		return TriggerRecord{}, err
	}
	return response.Trigger, nil
}

func (c *unixSocketClient) GetAutomationTrigger(ctx context.Context, id string) (TriggerRecord, error) {
	var response contract.TriggerResponse
	path := "/api/automation/triggers/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return TriggerRecord{}, err
	}
	return response.Trigger, nil
}

func (c *unixSocketClient) UpdateAutomationTrigger(
	ctx context.Context,
	id string,
	request AutomationTriggerUpdateRequest,
) (TriggerRecord, error) {
	var response contract.TriggerResponse
	path := "/api/automation/triggers/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodPatch, path, nil, request, &response); err != nil {
		return TriggerRecord{}, err
	}
	return response.Trigger, nil
}

func (c *unixSocketClient) DeleteAutomationTrigger(ctx context.Context, id string) error {
	path := "/api/automation/triggers/" + url.PathEscape(strings.TrimSpace(id))
	return c.doJSON(ctx, http.MethodDelete, path, nil, nil, nil)
}

func (c *unixSocketClient) AutomationTriggerRuns(
	ctx context.Context,
	id string,
	query AutomationRunQuery,
) ([]RunRecord, error) {
	var response contract.RunsResponse
	path := "/api/automation/triggers/" + url.PathEscape(strings.TrimSpace(id)) + "/runs"
	if err := c.doJSON(ctx, http.MethodGet, path, automationRunValues(query), nil, &response); err != nil {
		return nil, err
	}
	return response.Runs, nil
}

func (c *unixSocketClient) ListAutomationRuns(ctx context.Context, query AutomationRunQuery) ([]RunRecord, error) {
	var response contract.RunsResponse
	if err := c.doJSON(
		ctx,
		http.MethodGet,
		"/api/automation/runs",
		automationRunValues(query),
		nil,
		&response,
	); err != nil {
		return nil, err
	}
	return response.Runs, nil
}

func (c *unixSocketClient) GetAutomationRun(ctx context.Context, id string) (RunRecord, error) {
	var response contract.RunResponse
	path := "/api/automation/runs/" + url.PathEscape(strings.TrimSpace(id))
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return RunRecord{}, err
	}
	return response.Run, nil
}
