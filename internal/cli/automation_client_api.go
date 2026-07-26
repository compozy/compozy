package cli

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
)

type automationClientAPI interface {
	ListAutomationSuggestions(
		ctx context.Context,
		workspaceID string,
		status automationpkg.SuggestionStatus,
	) (AutomationSuggestionListRecord, error)
	AcceptAutomationSuggestion(
		ctx context.Context,
		workspaceID string,
		suggestionID string,
	) (AutomationSuggestionAcceptanceRecord, error)
	DismissAutomationSuggestion(
		ctx context.Context,
		workspaceID string,
		suggestionID string,
	) (AutomationSuggestionRecord, error)
	ListAutomationJobs(ctx context.Context, query AutomationJobQuery) (AutomationJobListRecord, error)
	CreateAutomationJob(ctx context.Context, request AutomationJobCreateRequest) (JobRecord, error)
	GetAutomationJob(ctx context.Context, id string) (JobRecord, error)
	UpdateAutomationJob(ctx context.Context, id string, request AutomationJobUpdateRequest) (JobRecord, error)
	DeleteAutomationJob(ctx context.Context, id string) error
	TriggerAutomationJob(ctx context.Context, id string) (RunRecord, error)
	AutomationJobRuns(ctx context.Context, id string, query AutomationRunQuery) ([]RunRecord, error)
	ListAutomationTriggers(ctx context.Context, query AutomationTriggerQuery) (AutomationTriggerListRecord, error)
	CreateAutomationTrigger(ctx context.Context, request AutomationTriggerCreateRequest) (TriggerRecord, error)
	GetAutomationTrigger(ctx context.Context, id string) (TriggerRecord, error)
	UpdateAutomationTrigger(
		ctx context.Context,
		id string,
		request AutomationTriggerUpdateRequest,
	) (TriggerRecord, error)
	DeleteAutomationTrigger(ctx context.Context, id string) error
	AutomationTriggerRuns(ctx context.Context, id string, query AutomationRunQuery) ([]RunRecord, error)
	ListAutomationRuns(ctx context.Context, query AutomationRunQuery) ([]RunRecord, error)
	GetAutomationRun(ctx context.Context, id string) (RunRecord, error)
}

// AutomationSuggestionListRecord wraps one exact-workspace suggestion list.
type AutomationSuggestionListRecord = contract.AutomationSuggestionsResponse

// AutomationSuggestionRecord wraps one resolved suggestion.
type AutomationSuggestionRecord = contract.AutomationSuggestionResponse

// AutomationSuggestionAcceptanceRecord wraps one accepted suggestion and its Job.
type AutomationSuggestionAcceptanceRecord = contract.AutomationSuggestionAcceptanceResponse

// SuggestionRecord is one shared automation suggestion payload.
type SuggestionRecord = contract.AutomationSuggestionPayload

func (c *unixSocketClient) ListAutomationSuggestions(
	ctx context.Context,
	workspaceID string,
	status automationpkg.SuggestionStatus,
) (AutomationSuggestionListRecord, error) {
	var response AutomationSuggestionListRecord
	path := automationSuggestionsPath(workspaceID)
	query := url.Values{}
	if status != "" {
		query.Set("status", string(status))
	}
	if err := c.doJSON(ctx, http.MethodGet, path, query, nil, &response); err != nil {
		return AutomationSuggestionListRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) AcceptAutomationSuggestion(
	ctx context.Context,
	workspaceID string,
	suggestionID string,
) (AutomationSuggestionAcceptanceRecord, error) {
	var response AutomationSuggestionAcceptanceRecord
	path := automationSuggestionActionPath(workspaceID, suggestionID, "accept")
	if err := c.doJSON(ctx, http.MethodPost, path, nil, nil, &response); err != nil {
		return AutomationSuggestionAcceptanceRecord{}, err
	}
	return response, nil
}

func (c *unixSocketClient) DismissAutomationSuggestion(
	ctx context.Context,
	workspaceID string,
	suggestionID string,
) (AutomationSuggestionRecord, error) {
	var response AutomationSuggestionRecord
	path := automationSuggestionActionPath(workspaceID, suggestionID, "dismiss")
	if err := c.doJSON(ctx, http.MethodPost, path, nil, nil, &response); err != nil {
		return AutomationSuggestionRecord{}, err
	}
	return response, nil
}

func automationSuggestionsPath(workspaceID string) string {
	return "/api/workspaces/" + url.PathEscape(strings.TrimSpace(workspaceID)) + "/automation/suggestions"
}

func automationSuggestionActionPath(workspaceID string, suggestionID string, action string) string {
	return automationSuggestionsPath(workspaceID) + "/" +
		url.PathEscape(strings.TrimSpace(suggestionID)) + "/" + action
}
