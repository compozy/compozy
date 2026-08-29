package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"net/http"

	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/compozy/compozy/internal/worktree"

	"github.com/gin-gonic/gin"
)

const (
	handlersErrorKey     = "error"
	handlersOperationKey = "operation"
)

const defaultPollInterval = 100 * time.Millisecond

const (
	defaultSessionRecapLimit       = 20
	maxSessionRecapLimit           = 100
	pendingTranscriptMarkerLimit   = maxSessionRecapLimit * 5
	recapConsistencyPersistedReads = "persisted_reads"
)

var errCreateAgentRequestInvalid = errors.New("api: invalid create agent request")

// SetHTTPPort overrides the reported HTTP port for daemon status responses.
func (h *BaseHandlers) SetHTTPPort(port int) {
	if h == nil || port <= 0 {
		return
	}
	h.httpPort.Store(int64(port))
}

// CreateSession creates a new runtime session.
func (h *BaseHandlers) CreateSession(c *gin.Context) {
	var req contract.CreateSessionRequest
	if err := decodeStrictJSONBody(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode create session request: %w", h.transportName(), err),
		)
		return
	}
	if err := h.validateCreateSessionRequest(req); err != nil {
		h.respondError(c, statusForCreateSessionValidationError(err), err)
		return
	}
	parentSessionID, err := h.resolveCreateSessionParent(c, req)
	if err != nil {
		h.respondError(c, StatusForAgentIdentityError(err), err)
		return
	}
	if h.SessionAcceptance == nil {
		h.respondError(
			c,
			http.StatusServiceUnavailable,
			errors.New("api: asynchronous session acceptance is unavailable"),
		)
		return
	}
	mutationScope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	worktreeTarget, err := h.resolveCreateSessionWorktree(c, mutationScope.ProfileID, req)
	if err != nil {
		h.respondError(c, StatusForWorktreeError(err), err)
		return
	}

	opts := session.CreateOpts{
		ProfileID:            mutationScope.ProfileID,
		AgentName:            req.AgentName,
		Name:                 req.Name,
		Workspace:            req.Workspace,
		WorkspacePath:        req.WorkspacePath,
		Worktree:             worktreeTarget.ID,
		NetworkParticipation: req.NetworkParticipation,
		Type:                 session.SessionTypeUser,
	}
	if parentSessionID != "" {
		parent, statusErr := h.Sessions.Status(c.Request.Context(), parentSessionID)
		if statusErr != nil {
			h.respondError(c, StatusForSessionError(statusErr), statusErr)
			return
		}
		if !h.requireSessionInProfile(c, parent, mutationScope) {
			return
		}
		opts.Lineage = &store.SessionLineage{ParentSessionID: parentSessionID}
	}
	info, err := h.SessionAcceptance.CreateAccepted(c.Request.Context(), session.CreateAcceptedOpts{Session: opts})
	if err != nil {
		status := StatusForSessionError(err)
		err = errors.Join(err, h.rollbackMaterializedSessionWorktree(c.Request.Context(), worktreeTarget))
		h.respondError(c, status, err)
		return
	}

	payload, err := h.sessionPayloadWithOptionalHealth(c.Request.Context(), info, false)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	c.JSON(http.StatusCreated, contract.SessionResponse{Session: payload})
}

func (h *BaseHandlers) resolveCreateSessionWorktree(
	c *gin.Context,
	profileID string,
	req contract.CreateSessionRequest,
) (materializedSessionWorktree, error) {
	if ref := strings.TrimSpace(req.Worktree); ref != "" {
		return materializedSessionWorktree{ID: ref}, nil
	}
	if req.NewWorktree == nil {
		return materializedSessionWorktree{}, nil
	}
	if h.Worktrees == nil {
		return materializedSessionWorktree{}, errors.New("api: worktree creation is unavailable")
	}
	workspaceID, err := h.lookupWorkspaceID(c.Request.Context(), req.Workspace)
	if err != nil {
		return materializedSessionWorktree{}, err
	}
	item, err := h.Worktrees.CreateReady(c.Request.Context(), workspaceID, worktree.CreateOptions{
		ProfileID: profileID,
		Name:      strings.TrimSpace(req.NewWorktree.Name),
		Origin:    worktree.OriginManual,
	})
	if err != nil {
		return materializedSessionWorktree{}, err
	}
	return materializedSessionWorktree{ID: item.ID, WorkspaceID: workspaceID, Created: true}, nil
}

// resolveCreateSessionParent picks the provenance parent for one create request:
// an explicit parent wins; otherwise a validated agent caller links its own session.
func (h *BaseHandlers) resolveCreateSessionParent(
	c *gin.Context,
	req contract.CreateSessionRequest,
) (string, error) {
	if explicit := strings.TrimSpace(req.ParentSessionID); explicit != "" {
		return explicit, nil
	}
	credentials := agentCallerCredentialsFromRequest(c)
	if !hasAgentCallerIdentityCredentials(credentials) {
		return "", nil
	}
	caller, err := h.resolveAgentCaller(c.Request.Context(), credentials, agentActionSessionCreate)
	if err != nil {
		return "", fmt.Errorf("api: resolve create session caller identity: %w", err)
	}
	return caller.Session.ID, nil
}

// GetSession returns one session snapshot.
func (h *BaseHandlers) GetSession(c *gin.Context) {
	_, _, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	readScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if !readScope.Matches(info.ProfileID) {
		h.respondError(c, http.StatusNotFound, errors.New("session not found"))
		return
	}
	includeHealth, err := parseBoolQuery(c, "include_health")
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	payload, err := h.sessionPayloadWithOptionalHealth(c.Request.Context(), info, includeHealth)
	if err != nil {
		h.respondError(c, StatusForHeartbeatError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.SessionResponse{Session: payload})
}

// DeleteSession removes one session from the runtime catalog and persisted history.
func (h *BaseHandlers) DeleteSession(c *gin.Context) {
	_, sessionID, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	scope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if !h.requireSessionInProfile(c, info, scope) {
		return
	}
	if err := h.Sessions.Delete(c.Request.Context(), sessionID); err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}

	c.Status(http.StatusNoContent)
}

// StopSession stops a running session without deleting persisted history.
func (h *BaseHandlers) StopSession(c *gin.Context) {
	_, sessionID, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	scope, err := h.resolveProfileMutationScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if !h.requireSessionInProfile(c, info, scope) {
		return
	}
	var request contract.StopSessionRequest
	if c.Request.ContentLength != 0 {
		if err := decodeStrictJSONBody(c, &request); err != nil {
			h.respondError(c, http.StatusUnprocessableEntity, fmt.Errorf("decode session stop request: %w", err))
			return
		}
	}
	if request.Subtree {
		if h.Calls == nil {
			h.respondError(c, http.StatusServiceUnavailable, errors.New("api: calls service is not configured"))
			return
		}
		operationCtx, cancel, operationErr := h.callsOperationContext(c.Request.Context())
		if operationErr != nil {
			h.respondCallsError(c, operationErr)
			return
		}
		defer cancel()
		report, drainErr := h.Calls.DrainSubtree(operationCtx, sessionID, h.callsOperatorActor(), request.Reason)
		if drainErr != nil {
			h.respondCallsError(c, drainErr)
			return
		}
		if err := h.Sessions.StopWithCause(
			operationCtx,
			sessionID,
			session.CauseUserRequested,
			request.Reason,
		); err != nil &&
			!errors.Is(err, session.ErrSessionNotFound) {
			h.respondError(c, StatusForSessionError(err), err)
			return
		}
		c.JSON(http.StatusOK, contract.StopSessionSubtreeResponse{
			StoppedChildren: len(report.Stopped), ClosedCalls: len(report.CanceledCalls),
			PreservedResults: report.PreservedResults,
		})
		return
	}
	if err := h.Sessions.Stop(c.Request.Context(), sessionID); err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}

	c.Status(http.StatusNoContent)
}

// ResumeSession attaches a caller to an eligible live session.
func (h *BaseHandlers) ResumeSession(c *gin.Context) {
	h.AttachSession(c)
}

// AttachSession acquires a short-lived attach lease without starting a new runtime authority.
func (h *BaseHandlers) AttachSession(c *gin.Context) {
	attacher, ok := h.Sessions.(SessionAttachManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session attach manager is required"))
		return
	}
	_, sessionID, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	var req contract.AttachSessionRequest
	if err := decodeOptionalJSONBody(c, &req); err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	maxTTL := time.Duration(contract.SessionAttachMaxTTLSeconds) * time.Second
	if req.TTLSeconds > contract.SessionAttachMaxTTLSeconds {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("attach ttl must be <= %s", maxTTL))
		return
	}
	ttl := time.Duration(contract.SessionAttachDefaultTTLSeconds) * time.Second
	if req.TTLSeconds > 0 {
		ttl = time.Duration(req.TTLSeconds) * time.Second
	}
	attachedTo := strings.TrimSpace(req.AttachedTo)
	if attachedTo == "" {
		attachedTo = fmt.Sprintf("%s:%d", h.transportName(), h.PID())
	}
	attach, err := attacher.AttachSession(c.Request.Context(), store.SessionAttachRequest{
		SessionID:  sessionID,
		AttachedTo: attachedTo,
		Now:        h.Now(),
		TTL:        ttl,
	})
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}

	info, err := h.Sessions.Status(c.Request.Context(), sessionID)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	payload := sessionPayloadFromInfoAt(info, attach.AttachedAt, false)
	c.JSON(http.StatusOK, contract.SessionAttachResponse{
		Session: payload,
		Attach: contract.SessionAttachPayload{
			SessionID:       attach.SessionID,
			AttachedTo:      attach.AttachedTo,
			AttachExpiresAt: attach.AttachExpiresAt,
			AttachedAt:      attach.AttachedAt,
		},
	})
}

func decodeOptionalJSONBody(c *gin.Context, dest any) error {
	if c.Request.Body == nil {
		return nil
	}
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return fmt.Errorf("read request body: %w", err)
	}
	if strings.TrimSpace(string(body)) == "" {
		return nil
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("decode request body: %w", err)
	}
	return nil
}

// RepairSession inspects and optionally repairs an interrupted persisted session transcript.
func (h *BaseHandlers) RepairSession(c *gin.Context) {
	_, sessionID, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	dryRun, err := repairBoolQuery(c, "dry_run", "dry-run")
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	force, err := repairBoolQuery(c, "force")
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}

	result, err := h.Sessions.RepairSession(c.Request.Context(), session.RepairOpts{
		SessionID: sessionID,
		DryRun:    dryRun,
		Force:     force,
	})
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}

	c.JSON(http.StatusOK, contract.SessionRepairResponse{Repair: SessionRepairPayloadFromResult(result)})
}
