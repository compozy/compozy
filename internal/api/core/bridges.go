package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	bridgepkg "github.com/compozy/agh/internal/bridges"

	"github.com/compozy/agh/internal/notifications"

	"github.com/gin-gonic/gin"
)

const (
	bridgesErrorKey = "error"
)

var errBridgeServiceUnavailable = errors.New("bridge service is not configured")

type taskNotificationCursorReader interface {
	GetCursor(ctx context.Context, key notifications.CursorKey) (notifications.Cursor, error)
}

type bridgeScopeQuery struct {
	scope       string
	workspaceID string
}

func (h *BaseHandlers) parseBridgeScopeQuery(ctx context.Context, c *gin.Context) (bridgeScopeQuery, error) {
	scope := strings.ToLower(strings.TrimSpace(c.Query("scope")))
	switch scope {
	case "", "all", string(bridgepkg.ScopeGlobal), string(bridgepkg.ScopeWorkspace):
	default:
		return bridgeScopeQuery{}, fmt.Errorf("%s: unsupported bridge list scope %q", h.transportName(), scope)
	}

	workspaceID, err := h.bridgeListWorkspaceID(ctx, c)
	if err != nil {
		return bridgeScopeQuery{}, err
	}
	if scope == string(bridgepkg.ScopeGlobal) && workspaceID != "" {
		return bridgeScopeQuery{}, fmt.Errorf(
			"%s: global bridge list scope cannot include workspace id",
			h.transportName(),
		)
	}
	if scope == string(bridgepkg.ScopeWorkspace) && workspaceID == "" {
		return bridgeScopeQuery{}, fmt.Errorf(
			"%s: workspace bridge list scope requires workspace id",
			h.transportName(),
		)
	}
	return bridgeScopeQuery{scope: scope, workspaceID: workspaceID}, nil
}

func (h *BaseHandlers) bridgeListWorkspaceID(ctx context.Context, c *gin.Context) (string, error) {
	if workspaceID := strings.TrimSpace(c.Query("workspace_id")); workspaceID != "" {
		return workspaceID, nil
	}
	if workspaceRef := strings.TrimSpace(c.Query("workspace")); workspaceRef != "" {
		id, err := h.lookupWorkspaceID(ctx, workspaceRef)
		if err != nil {
			return "", err
		}
		return id, nil
	}
	return "", nil
}

func bridgeInstanceMatchesListQuery(instance bridgepkg.BridgeInstance, query bridgeScopeQuery) bool {
	scope := instance.Scope.Normalize()
	workspaceID := strings.TrimSpace(instance.WorkspaceID)
	switch query.scope {
	case string(bridgepkg.ScopeGlobal):
		return scope == bridgepkg.ScopeGlobal
	case string(bridgepkg.ScopeWorkspace):
		return scope == bridgepkg.ScopeWorkspace && workspaceID == query.workspaceID
	default:
		if query.workspaceID == "" {
			return true
		}
		return scope == bridgepkg.ScopeGlobal || (scope == bridgepkg.ScopeWorkspace && workspaceID == query.workspaceID)
	}
}

// ListBridgeProviders returns installed bridge-capable providers.
func (h *BaseHandlers) ListBridgeProviders(c *gin.Context) {
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return
	}

	providers, err := bridges.ListProviders(c.Request.Context())
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}

	payloads := make([]contract.BridgeProviderPayload, 0, len(providers))
	for _, provider := range providers {
		payloads = append(payloads, BridgeProviderPayloadFromBridgeProvider(provider))
	}
	c.JSON(http.StatusOK, contract.BridgeProvidersResponse{Providers: payloads})
}

// CreateBridge persists a new bridge instance.
func (h *BaseHandlers) CreateBridge(c *gin.Context) {
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return
	}

	var req contract.CreateBridgeRequest
	if err := decodeStrictBridgeJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode create bridge request: %w", h.transportName(), err),
		)
		return
	}

	createReq, err := req.ToCreateInstanceRequest()
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}

	instance, err := bridges.CreateInstance(c.Request.Context(), createReq)
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	h.respondBridge(c, http.StatusCreated, *instance)
}

func decodeStrictBridgeJSON(c *gin.Context, dest any) error {
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return err
	}
	var extra json.RawMessage
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("request body must contain a single JSON value")
}

// GetBridge returns one persisted bridge instance.
func (h *BaseHandlers) GetBridge(c *gin.Context) {
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return
	}

	instance, err := bridges.GetInstance(c.Request.Context(), strings.TrimSpace(c.Param("id")))
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	h.respondBridge(c, http.StatusOK, *instance)
}

// UpdateBridge patches the mutable configuration fields of one bridge instance.
func (h *BaseHandlers) UpdateBridge(c *gin.Context) {
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return
	}

	var req contract.UpdateBridgeRequest
	if err := decodeStrictBridgeJSON(c, &req); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode update bridge request: %w", h.transportName(), err),
		)
		return
	}

	updateReq, err := req.ToUpdateInstanceRequest(c.Param("id"))
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}

	instance, err := bridges.UpdateInstance(c.Request.Context(), updateReq)
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	h.respondBridge(c, http.StatusOK, *instance)
}

// EnableBridge moves one bridge instance into the starting lifecycle state.
func (h *BaseHandlers) EnableBridge(c *gin.Context) {
	h.transitionBridge(c, (*BaseHandlers).enableBridge)
}

// DisableBridge moves one bridge instance into the disabled lifecycle state.
func (h *BaseHandlers) DisableBridge(c *gin.Context) {
	h.transitionBridge(c, (*BaseHandlers).disableBridge)
}

// RestartBridge restarts one bridge instance while preserving route ownership.
func (h *BaseHandlers) RestartBridge(c *gin.Context) {
	h.transitionBridge(c, (*BaseHandlers).restartBridge)
}

// ListBridgeRoutes returns the persisted routes owned by one bridge instance.
func (h *BaseHandlers) ListBridgeRoutes(c *gin.Context) {
	bridges, ok := h.bridgeService()
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errBridgeServiceUnavailable)
		return
	}

	routes, err := bridges.ListRoutes(c.Request.Context(), strings.TrimSpace(c.Param("id")))
	if err != nil {
		h.respondError(c, StatusForBridgeError(err), err)
		return
	}
	c.JSON(http.StatusOK, contract.BridgeRoutesResponse{Routes: routes})
}
