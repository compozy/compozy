package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"net/http"

	"strings"
	"time"

	"github.com/compozy/agh/internal/api/contract"

	"github.com/compozy/agh/internal/session"
	"github.com/compozy/agh/internal/store"

	"github.com/gin-gonic/gin"
)

const (
	handlersErrorKey     = "error"
	handlersOperationKey = "operation"
)

const defaultPollInterval = 100 * time.Millisecond

const (
	defaultSessionAttachTTL        = 15 * time.Minute
	maxSessionAttachTTL            = 24 * time.Hour
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

	opts := session.CreateOpts{
		AgentName:            req.AgentName,
		Provider:             strings.TrimSpace(req.Provider),
		Model:                strings.TrimSpace(req.Model),
		ReasoningEffort:      strings.TrimSpace(string(req.ReasoningEffort)),
		Name:                 req.Name,
		Workspace:            strings.TrimSpace(req.Workspace),
		WorkspacePath:        strings.TrimSpace(req.WorkspacePath),
		NetworkParticipation: req.NetworkParticipation,
		Type:                 session.SessionTypeUser,
	}
	if h.SessionAcceptance == nil {
		h.respondError(
			c,
			http.StatusServiceUnavailable,
			errors.New("api: asynchronous session acceptance is unavailable"),
		)
		return
	}
	info, err := h.SessionAcceptance.CreateAccepted(c.Request.Context(), session.CreateAcceptedOpts{
		Session:       opts,
		InitialPrompt: strings.TrimSpace(req.Prompt),
	})
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}

	c.JSON(http.StatusCreated, contract.SessionResponse{Session: SessionPayloadFromInfo(info)})
}

// GetSession returns one session snapshot.
func (h *BaseHandlers) GetSession(c *gin.Context) {
	_, _, info, ok := h.routeSessionInWorkspace(c)
	if !ok {
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
	_, sessionID, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
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
	_, sessionID, _, ok := h.routeSessionInWorkspace(c)
	if !ok {
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
	ttl := defaultSessionAttachTTL
	if req.TTLSeconds > 0 {
		ttl = time.Duration(req.TTLSeconds) * time.Second
	}
	if ttl > maxSessionAttachTTL {
		h.respondError(c, http.StatusBadRequest, fmt.Errorf("attach ttl must be <= %s", maxSessionAttachTTL))
		return
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
	payload := sessionPayloadFromInfoAt(info, attach.AttachedAt)
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
