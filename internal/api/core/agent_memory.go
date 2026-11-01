package core

import (
	"errors"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/agentmemory"
	"github.com/compozy/compozy/internal/api/contract"
	compozyconfig "github.com/compozy/compozy/internal/config"
	"github.com/gin-gonic/gin"
)

func (h *BaseHandlers) RegisterAgentMemoryRoutes(agents gin.IRouter) {
	for _, route := range []struct{ method, path, operation string }{
		{http.MethodGet, "/:name/memory", "read"}, {http.MethodPut, "/:name/memory", "write"},
		{http.MethodPost, agentMemoryEntriesRoute, "add"}, {http.MethodPatch, agentMemoryEntriesRoute, "replace"},
		{http.MethodDelete, agentMemoryEntriesRoute, "remove"}, {http.MethodPost, "/:name/memory/search", "search"},
		{http.MethodPost, "/:name/memory/validate", "validate"}, {http.MethodGet, "/:name/memory/history", "history"},
		{http.MethodPost, "/:name/memory/rollback", "rollback"},
	} {
		agents.Handle(route.method, route.path, func(c *gin.Context) { h.agentMemoryOperation(c, route.operation) })
	}
}

func (h *BaseHandlers) agentMemoryTarget(c *gin.Context) (agentmemory.Key, string, error) {
	if h.AgentMemory == nil {
		return agentmemory.Key{}, "", errors.New("agent_memory_unavailable")
	}
	scope, profileName, err := h.agentResourceProfile(c)
	if err != nil {
		return agentmemory.Key{}, "", err
	}
	ref := strings.TrimSpace(c.Query("workspace"))
	name := pathAgentName(c)
	callerOrigin, ref, err := h.agentMemoryCallerOrigin(c, ref, name, scope.ProfileID, profileName)
	if err != nil {
		return agentmemory.Key{}, "", err
	}
	var entries workspaceAgentEntries
	if ref == "" {
		entries, err = h.globalAgentEntries(c.Request.Context(), scope.ProfileID, profileName)
	} else {
		entries, err = h.workspaceAgentEntriesWithDiagnostics(c.Request.Context(), ref, profileName)
	}
	if err != nil {
		return agentmemory.Key{}, "", err
	}
	for _, entry := range entries.Entries {
		if entry.Def.Name != name {
			continue
		}
		if entry.Def.EffectiveKind() != compozyconfig.AgentKindStaysOn {
			return agentmemory.Key{}, "", errors.New("agent_not_stays_on")
		}
		if callerOrigin != "" && callerOrigin != entry.MemoryOrigin {
			return agentmemory.Key{}, "", errors.New("agent_instance_forbidden")
		}
		if entry.MemoryOrigin == "" {
			return agentmemory.Key{}, "", errors.New("agent_memory_identity_unavailable")
		}
		source := entries.WorkspaceName
		if source == "" {
			source = "Global"
		}
		return agentmemory.Key{ProfileID: scope.ProfileID, Origin: entry.MemoryOrigin, Agent: name}, source, nil
	}
	return agentmemory.Key{}, "", os.ErrNotExist
}

func (h *BaseHandlers) agentMemoryOperation(c *gin.Context, operation string) {
	key, source, err := h.agentMemoryTarget(c)
	if err != nil {
		h.respondAgentMemoryError(c, err)
		return
	}
	var result any
	switch operation {
	case "read":
		result, err = h.AgentMemory.Read(key)
	case "history":
		var revisions []agentmemory.Revision
		revisions, err = h.AgentMemory.History(key)
		result = agentMemoryHistoryPayload(revisions)
	case "write":
		var req contract.AgentMemoryWriteRequest
		if err = decodeAuthoredJSONBody(c, &req, false); err == nil {
			result, err = h.AgentMemory.Write(key, req.Content, req.ExpectedDigest)
		}
	case "add":
		var req contract.AgentMemoryAddRequest
		if err = decodeAuthoredJSONBody(c, &req, false); err == nil {
			result, err = h.AgentMemory.Add(key, time.Now(), source, req.Text, req.Until)
		}
	case "replace":
		var req contract.AgentMemoryReplaceRequest
		if err = decodeAuthoredJSONBody(c, &req, false); err == nil {
			result, err = h.AgentMemory.Replace(key, req.Match, req.Text)
		}
	case "remove":
		var req contract.AgentMemoryRemoveRequest
		if err = decodeAuthoredJSONBody(c, &req, false); err == nil {
			var doc agentmemory.Document
			var removed int
			doc, removed, err = h.AgentMemory.Remove(key, req.Match)
			result = contract.AgentMemoryRemoveResponse{Digest: doc.Digest, Removed: removed}
		}
	case "rollback":
		var req contract.AgentMemoryRollbackRequest
		if err = decodeAuthoredJSONBody(c, &req, false); err == nil {
			result, err = h.AgentMemory.Rollback(key, req.Revision)
		}
	case "search":
		var req contract.AgentMemorySearchRequest
		if err = decodeAuthoredJSONBody(c, &req, false); err == nil {
			var results []agentmemory.SearchResult
			results, err = h.searchAgentMemory(c, key, req)
			result = agentMemorySearchPayload(results)
		}
	case "validate":
		var req contract.AgentMemoryValidateRequest
		if err = decodeAuthoredJSONBody(c, &req, false); err == nil {
			result = agentMemoryValidationPayload(req.Content)
		}
	}
	if err != nil {
		h.respondAgentMemoryError(c, err)
		return
	}
	if doc, ok := result.(agentmemory.Document); ok {
		result = agentMemoryDocumentPayload(doc)
	}
	c.JSON(http.StatusOK, result)
}

func (h *BaseHandlers) respondAgentMemoryError(c *gin.Context, err error) {
	status := StatusForAgentIdentityError(err)
	if status == http.StatusInternalServerError {
		status = http.StatusUnprocessableEntity
	}
	switch {
	case errors.Is(err, agentmemory.ErrConflict):
		status = http.StatusConflict
	case errors.Is(err, os.ErrNotExist):
		status = http.StatusNotFound
	case strings.Contains(err.Error(), "forbidden"):
		status = http.StatusForbidden
	case strings.Contains(err.Error(), "unavailable"):
		status = http.StatusServiceUnavailable
	}
	code := ""
	switch {
	case errors.Is(err, agentmemory.ErrConflict):
		code = "agent_memory_conflict"
	case errors.Is(err, agentmemory.ErrBudgetExceeded):
		code = "agent_memory_budget_exceeded"
	case errors.Is(err, agentmemory.ErrEntryTooLarge):
		code = "agent_memory_entry_too_large"
	case err.Error() == "agent_not_stays_on":
		code = "agent_not_stays_on"
	case err.Error() == "agent_instance_forbidden":
		code = "agent_instance_forbidden"
	}
	if code != "" {
		err = agentMemoryRouteError{cause: err, code: code}
	}
	h.respondError(c, status, err)
}

type agentMemoryRouteError struct {
	cause error
	code  string
}

var _ diagnosticCodeCarrier = agentMemoryRouteError{}

func (e agentMemoryRouteError) Error() string          { return e.cause.Error() }
func (e agentMemoryRouteError) Unwrap() error          { return e.cause }
func (e agentMemoryRouteError) DiagnosticCode() string { return e.code }

const agentMemoryEntriesRoute = "/:name/memory/entries"

func (h *BaseHandlers) agentMemoryCallerOrigin(
	c *gin.Context,
	ref, name, profileID, profileName string,
) (string, string, error) {
	callerOrigin := ""
	credentials := agentCallerCredentialsFromRequest(c)
	if !hasAgentCallerIdentityCredentials(credentials) {
		return callerOrigin, ref, nil
	}
	caller, err := h.resolveAgentCallerForWorkspace(c.Request.Context(), credentials, "agent.memory", ref)
	if err != nil {
		return "", ref, err
	}
	if caller.Session.AgentName != name || caller.Session.ProfileID != profileID {
		return "", ref, errors.New("agent_instance_forbidden")
	}
	if h.AgentInstances == nil {
		return "", ref, errors.New("agent_instance_forbidden")
	}
	if err := h.AgentInstances.Authority.AuthorizeMemory(
		c.Request.Context(),
		caller.Session.ID,
		name,
		profileID,
	); err != nil {
		return "", ref, errors.New("agent_instance_forbidden")
	}
	var own workspaceAgentEntries
	if caller.Session.WorkspaceID == "" {
		own, err = h.globalAgentEntries(c.Request.Context(), profileID, profileName)
	} else {
		own, err = h.workspaceAgentEntriesWithDiagnostics(
			c.Request.Context(),
			caller.Session.WorkspaceID,
			profileName,
		)
	}
	if err != nil {
		return "", ref, err
	}
	for _, entry := range own.Entries {
		if entry.Def.Name == name {
			callerOrigin = entry.MemoryOrigin
		}
	}
	if callerOrigin == "" {
		return "", ref, errors.New("agent_instance_forbidden")
	}
	if ref == "" {
		ref = caller.Session.WorkspaceID
	}
	return callerOrigin, ref, nil
}
