package core

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	automationpkg "github.com/compozy/compozy/internal/automation"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

// ListAutomationSuggestions returns one workspace's consent-first Job proposals.
func (h *BaseHandlers) ListAutomationSuggestions(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}
	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
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
	var suggestions []automationpkg.Suggestion
	if c.Param("workspace_id") == "" {
		breadth, ok := h.aggregateBreadth(c)
		if !ok {
			return
		}
		reader, ok := manager.(interface {
			ListSuggestionsAggregate(
				context.Context,
				store.ReadScope,
				automationpkg.SuggestionStatus,
			) ([]automationpkg.Suggestion, error)
		})
		if !ok {
			h.respondError(c, http.StatusServiceUnavailable, fmt.Errorf("automation suggestion aggregate unavailable"))
			return
		}
		suggestions, err = reader.ListSuggestionsAggregate(c.Request.Context(), readScope, status)
		if err == nil {
			allowed, selectionErr := h.aggregateWorkspaceSet(c, breadth)
			if selectionErr != nil {
				err = selectionErr
			} else {
				selected := suggestions[:0]
				for _, item := range suggestions {
					if allowed[item.WorkspaceID] {
						selected = append(selected, item)
					}
				}
				suggestions = selected
			}
		}
	} else {
		if _, parseErr := parseWorkspaceBreadth(
			c,
			c.Param("workspace_id"),
			breadthEncoding{Selector: workspaceIDKey},
		); parseErr != nil {
			h.respondAggregateBreadthError(c, parseErr)
			return
		}
		suggestions, err = manager.ListSuggestions(c.Request.Context(), readScope, c.Param("workspace_id"), status)
	}
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	payloads := AutomationSuggestionPayloadsFromSuggestions(suggestions)
	jobs := make([]contract.JobPayload, 0, len(payloads))
	for index := range payloads {
		jobs = append(jobs, payloads[index].Payload)
	}
	if err := h.decorateAutomationJobOwners(c.Request.Context(), jobs); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	for index := range payloads {
		payloads[index].Payload = jobs[index]
	}
	c.JSON(http.StatusOK, contract.AutomationSuggestionsResponse{Suggestions: payloads})
}

// AcceptAutomationSuggestion accepts one proposal and creates its Job.
func (h *BaseHandlers) AcceptAutomationSuggestion(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}
	mutationScope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	accepted, err := manager.AcceptSuggestion(
		c.Request.Context(),
		mutationScope.ProfileID,
		c.Param("workspace_id"),
		c.Param("suggestion_id"),
	)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	suggestion := AutomationSuggestionPayloadFromSuggestion(accepted.Suggestion)
	job := JobPayloadFromJob(accepted.Job, nil, nil)
	if err := h.decorateAutomationJobOwner(c.Request.Context(), &job); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	suggestion.Payload = job
	c.JSON(http.StatusOK, contract.AutomationSuggestionAcceptanceResponse{Suggestion: suggestion, Job: job})
}

// DismissAutomationSuggestion dismisses one proposal without creating a Job.
func (h *BaseHandlers) DismissAutomationSuggestion(c *gin.Context) {
	manager, ok := h.requireAutomationManager(c)
	if !ok {
		return
	}
	mutationScope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	dismissed, err := manager.DismissSuggestion(
		c.Request.Context(),
		mutationScope.ProfileID,
		c.Param("workspace_id"),
		c.Param("suggestion_id"),
	)
	if err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	payload := AutomationSuggestionPayloadFromSuggestion(dismissed)
	if err := h.decorateAutomationJobOwner(c.Request.Context(), &payload.Payload); err != nil {
		h.respondError(c, StatusForAutomationError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.AutomationSuggestionResponse{
		Suggestion: payload,
	})
}

// AutomationSuggestionPayloadFromSuggestion converts one domain suggestion to its shared payload.
func AutomationSuggestionPayloadFromSuggestion(
	suggestion automationpkg.Suggestion,
) contract.AutomationSuggestionPayload {
	return contract.AutomationSuggestionPayload{
		ID:          suggestion.ID,
		ProfileID:   suggestion.ProfileID,
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
