package core

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/compozy/compozy/internal/api/contract"
	taskpkg "github.com/compozy/compozy/internal/task"
	"github.com/gin-gonic/gin"
)

const (
	extensionActionDev            = "dev"
	extensionActionReload         = "reload"
	extensionActionLogs           = "logs"
	extensionLogSSEEventName      = "extension_log"
	extensionLogResetSSEEventName = "extension_log_reset"
	extensionLogPollInterval      = 100 * time.Millisecond
)

// ExtensionDevService exposes workspace-bound development lifecycle operations.
type ExtensionDevService interface {
	Dev(
		context.Context,
		contract.DevLinkExtensionRequest,
		taskpkg.ActorContext,
	) (contract.ExtensionPayload, error)
	ReloadDev(
		context.Context,
		string,
		contract.ReloadExtensionRequest,
		taskpkg.ActorContext,
	) (contract.ExtensionPayload, error)
	ExtensionLogs(
		context.Context,
		string,
		int64,
		string,
		taskpkg.ActorContext,
	) (contract.ExtensionLogsResponse, error)
}

// ExtensionScopedReadService exposes caller-filtered read projections.
type ExtensionScopedReadService interface {
	ListScoped(context.Context, taskpkg.ActorContext) ([]contract.ExtensionPayload, error)
	StatusScoped(context.Context, string, taskpkg.ActorContext) (contract.ExtensionPayload, error)
}

// ExtensionScopedRemoveService removes a workspace overlay without mutating the published row.
type ExtensionScopedRemoveService interface {
	RemoveScoped(
		context.Context,
		string,
		taskpkg.ActorContext,
	) (contract.ManagedExtensionRemovePayload, error)
}

// DevExtension links and activates one immutable generation in the trusted workspace.
func (h *BaseHandlers) DevExtension(c *gin.Context) {
	service, ok := h.extensionDevService(c)
	if !ok {
		return
	}
	var req contract.DevLinkExtensionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondExtensionError(c, http.StatusBadRequest, err)
		return
	}
	req.OriginPath = strings.TrimSpace(req.OriginPath)
	req.GenerationHash = strings.TrimSpace(req.GenerationHash)
	if req.OriginPath == "" || req.GenerationHash == "" {
		h.respondExtensionError(c, http.StatusBadRequest, errors.New("origin_path and generation_hash are required"))
		return
	}
	actor, ok := h.extensionScopedActorContext(c, extensionActionDev, true)
	if !ok {
		return
	}
	item, err := service.Dev(c.Request.Context(), req, actor)
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusCreated, contract.ExtensionResponse{Extension: item})
}

// ReloadDevExtension swaps one workspace instance to a verified generation.
func (h *BaseHandlers) ReloadDevExtension(c *gin.Context) {
	_, name, ok := h.namedExtensionService(c)
	if !ok {
		return
	}
	service, ok := h.extensionDevService(c)
	if !ok {
		return
	}
	var req contract.ReloadExtensionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.respondExtensionError(c, http.StatusBadRequest, err)
		return
	}
	req.GenerationHash = strings.TrimSpace(req.GenerationHash)
	if req.GenerationHash == "" {
		h.respondExtensionError(c, http.StatusBadRequest, errors.New("generation_hash is required"))
		return
	}
	actor, ok := h.extensionScopedActorContext(c, extensionActionReload, true)
	if !ok {
		return
	}
	item, err := service.ReloadDev(c.Request.Context(), name, req, actor)
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.ExtensionResponse{Extension: item})
}

// ExtensionLogs returns or follows the redacted ring for the trusted instance.
func (h *BaseHandlers) ExtensionLogs(c *gin.Context) {
	_, name, ok := h.namedExtensionService(c)
	if !ok {
		return
	}
	service, ok := h.extensionDevService(c)
	if !ok {
		return
	}
	after, err := parseExtensionLogSequence(c.Query("after"))
	if err != nil {
		h.respondExtensionError(c, http.StatusBadRequest, err)
		return
	}
	streamEpoch := strings.TrimSpace(c.Query("stream_epoch"))
	if after > 0 && streamEpoch == "" {
		h.respondExtensionError(
			c,
			http.StatusBadRequest,
			errors.New("stream_epoch is required when after is greater than zero"),
		)
		return
	}
	follow, err := ParseOptionalBool(c.Query("follow"))
	if err != nil {
		h.respondExtensionError(c, http.StatusBadRequest, err)
		return
	}
	actor, ok := h.extensionScopedActorContext(c, extensionActionLogs, false)
	if !ok {
		return
	}
	snapshot, err := service.ExtensionLogs(c.Request.Context(), name, after, streamEpoch, actor)
	if err != nil {
		h.respondExtensionError(c, ExtensionStatusCode(err), err)
		return
	}
	if !follow {
		c.JSON(http.StatusOK, snapshot)
		return
	}
	h.streamExtensionLogs(c, service, name, actor, after, streamEpoch, snapshot)
}

func (h *BaseHandlers) streamExtensionLogs(
	c *gin.Context,
	service ExtensionDevService,
	name string,
	actor taskpkg.ActorContext,
	after int64,
	requestedEpoch string,
	initial contract.ExtensionLogsResponse,
) {
	writer, err := PrepareSSE(c)
	if err != nil {
		h.respondExtensionError(c, http.StatusInternalServerError, err)
		return
	}
	// An empty ring emits no events; the ready comment pushes body bytes through
	// buffering proxies so EventSource clients leave the connecting state.
	if err := WriteSSEComment(writer, "extension log stream ready"); err != nil {
		h.logSSEWriteFailure(extensionLogSSEEventName, err)
		return
	}
	streamEpoch := initial.StreamEpoch
	var cursor int64
	if requestedEpoch != "" && requestedEpoch != streamEpoch {
		if err := emitExtensionLogReset(writer, initial); err != nil {
			h.logSSEWriteFailure(extensionLogResetSSEEventName, err)
			return
		}
		cursor = extensionLogSnapshotCursor(initial)
	} else {
		cursor = emitExtensionLogs(writer, after, initial.Logs)
	}
	ticker := time.NewTicker(extensionLogPollInterval)
	defer ticker.Stop()
	streamDone := h.StreamDoneChannel()
	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-streamDone:
			return
		case <-ticker.C:
			snapshot, pollErr := service.ExtensionLogs(
				c.Request.Context(), name, cursor, streamEpoch, actor,
			)
			if pollErr != nil {
				h.writeSSEBestEffort(writer, SSEMessage{Name: handlersErrorKey, Data: ErrorPayloadForError(pollErr)})
				return
			}
			if snapshot.StreamEpoch != streamEpoch {
				if err := emitExtensionLogReset(writer, snapshot); err != nil {
					h.logSSEWriteFailure(extensionLogResetSSEEventName, err)
					return
				}
				streamEpoch = snapshot.StreamEpoch
				cursor = extensionLogSnapshotCursor(snapshot)
				continue
			}
			cursor = emitExtensionLogs(writer, cursor, snapshot.Logs)
		}
	}
}

func emitExtensionLogReset(writer FlushWriter, snapshot contract.ExtensionLogsResponse) error {
	return WriteSSE(writer, SSEMessage{Name: extensionLogResetSSEEventName, Data: snapshot})
}

func extensionLogSnapshotCursor(snapshot contract.ExtensionLogsResponse) int64 {
	if len(snapshot.Logs) == 0 {
		return 0
	}
	return snapshot.Logs[len(snapshot.Logs)-1].Sequence
}

func emitExtensionLogs(writer FlushWriter, cursor int64, entries []contract.ExtensionLogPayload) int64 {
	for _, entry := range entries {
		if entry.Sequence <= cursor {
			continue
		}
		if err := WriteSSE(writer, SSEMessage{
			ID:   strconv.FormatInt(entry.Sequence, 10),
			Name: extensionLogSSEEventName,
			Data: entry,
		}); err != nil {
			return cursor
		}
		cursor = entry.Sequence
	}
	return cursor
}

func parseExtensionLogSequence(raw string) (int64, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	value, err := strconv.ParseInt(trimmed, 10, 64)
	if err != nil || value < 0 {
		return 0, errors.New("after must be a non-negative integer")
	}
	return value, nil
}

func (h *BaseHandlers) extensionDevService(c *gin.Context) (ExtensionDevService, bool) {
	service, ok := h.extensionService(c)
	if !ok {
		return nil, false
	}
	dev, ok := service.(ExtensionDevService)
	if !ok {
		h.respondExtensionError(
			c,
			http.StatusServiceUnavailable,
			errors.New("api: extension dev service is not configured"),
		)
		return nil, false
	}
	return dev, true
}

func (h *BaseHandlers) extensionScopedActorContext(
	c *gin.Context,
	action string,
	requireWorkspace bool,
) (taskpkg.ActorContext, bool) {
	action = "extensions." + strings.TrimSpace(action)
	if h.TaskActorContextResolver != nil {
		actor, err := h.TaskActorContextResolver(c, action)
		if err != nil {
			h.respondExtensionError(c, StatusForTaskError(err), err)
			return taskpkg.ActorContext{}, false
		}
		if requireWorkspace && strings.TrimSpace(actor.Scope.WorkspaceID) == "" {
			h.respondExtensionError(c, http.StatusBadRequest, errors.New("a resolved workspace is required"))
			return taskpkg.ActorContext{}, false
		}
		return actor, true
	}
	credentials := agentCallerCredentialsFromRequest(c)
	if hasAgentCallerIdentityCredentials(credentials) {
		caller, err := h.resolveAgentCallerForWorkspace(c.Request.Context(), credentials, action, "")
		if err != nil {
			h.respondExtensionError(c, StatusForTaskError(err), err)
			return taskpkg.ActorContext{}, false
		}
		return caller.Actor, true
	}
	workspaceID, err := h.resolveExpectedWorkspaceID(c.Request.Context(), c.Query("workspace"))
	if err != nil {
		h.respondExtensionError(c, http.StatusBadRequest, err)
		return taskpkg.ActorContext{}, false
	}
	if requireWorkspace && workspaceID == "" {
		h.respondExtensionError(c, http.StatusBadRequest, errors.New("workspace query is required"))
		return taskpkg.ActorContext{}, false
	}
	actor, err := taskpkg.DeriveHumanActorContextForWorkspace(
		defaultTaskActorRef,
		workspaceID,
		taskOriginKindForTransport(h.transportName()),
		action,
	)
	if err != nil {
		h.respondExtensionError(c, StatusForTaskError(err), err)
		return taskpkg.ActorContext{}, false
	}
	return actor, true
}
