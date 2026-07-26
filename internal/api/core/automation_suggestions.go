package core

import (
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	automationpkg "github.com/compozy/agh/internal/automation"
	"github.com/gin-gonic/gin"
)

// ListAutomationSuggestions returns one workspace's consent-first Job proposals.
func (h *BaseHandlers) ListAutomationSuggestions(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}
	status := automationpkg.SuggestionStatus(strings.TrimSpace(c.Query("status")))
	if status == "" {
		status = automationpkg.SuggestionStatusPending
	}
	if err := status.Validate("status"); err != nil {
		h.respondError(c, http.StatusBadRequest, NewAutomationValidationError(err))
		return
	}
	suggestions, err := manager.ListSuggestions(c.Request.Context(), c.Param("workspace_id"), status)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.AutomationSuggestionsResponse{
		Suggestions: AutomationSuggestionPayloadsFromSuggestions(suggestions),
	})
}

// AcceptAutomationSuggestion accepts one proposal and creates its Job.
func (h *BaseHandlers) AcceptAutomationSuggestion(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}
	accepted, err := manager.AcceptSuggestion(
		c.Request.Context(),
		c.Param("workspace_id"),
		c.Param("suggestion_id"),
	)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.AutomationSuggestionAcceptanceResponse{
		Suggestion: AutomationSuggestionPayloadFromSuggestion(accepted.Suggestion),
		Job:        JobPayloadFromJob(accepted.Job, nil, nil),
	})
}

// DismissAutomationSuggestion dismisses one proposal without creating a Job.
func (h *BaseHandlers) DismissAutomationSuggestion(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}
	dismissed, err := manager.DismissSuggestion(
		c.Request.Context(),
		c.Param("workspace_id"),
		c.Param("suggestion_id"),
	)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.AutomationSuggestionResponse{
		Suggestion: AutomationSuggestionPayloadFromSuggestion(dismissed),
	})
}

// AutomationSuggestionPayloadFromSuggestion converts one domain suggestion to its shared payload.
func AutomationSuggestionPayloadFromSuggestion(
	suggestion automationpkg.Suggestion,
) contract.AutomationSuggestionPayload {
	return contract.AutomationSuggestionPayload{
		ID:          suggestion.ID,
		WorkspaceID: suggestion.WorkspaceID,
		Source:      suggestion.Source,
		DedupKey:    suggestion.DedupKey,
		Status:      suggestion.Status,
		Payload:     JobPayloadFromJob(suggestion.Payload, nil, nil),
		CreatedAt:   suggestion.CreatedAt,
		ResolvedAt:  suggestion.ResolvedAt,
	}
}

// AutomationSuggestionPayloadsFromSuggestions converts domain suggestions to shared payloads.
func AutomationSuggestionPayloadsFromSuggestions(
	suggestions []automationpkg.Suggestion,
) []contract.AutomationSuggestionPayload {
	payloads := make([]contract.AutomationSuggestionPayload, 0, len(suggestions))
	for _, suggestion := range suggestions {
		payloads = append(payloads, AutomationSuggestionPayloadFromSuggestion(suggestion))
	}
	return payloads
}
