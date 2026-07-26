package core

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
	"github.com/compozy/agh/internal/resources"
	"github.com/compozy/agh/internal/windowmanager"
	"github.com/gin-gonic/gin"
)

// ListWindowManagerLayoutProfiles lists global and route-workspace layout profiles.
func (h *BaseHandlers) ListWindowManagerLayoutProfiles(c *gin.Context) {
	workspaceID := windowManagerWorkspace(c)
	service, ok := h.windowManagerLayoutProfileService(c, workspaceID)
	if !ok {
		return
	}

	scopes := []resources.ResourceScope{
		{Kind: resources.ResourceScopeKindGlobal},
		{Kind: resources.ResourceScopeKindWorkspace, ID: string(workspaceID)},
	}
	records := make([]resources.RawRecord, 0)
	for index := range scopes {
		visible, err := service.List(c.Request.Context(), resources.ResourceFilter{
			Kind:  windowmanager.WindowLayoutResourceKind,
			Scope: &scopes[index],
		})
		if err != nil {
			h.respondError(c, StatusForResourceError(err), err)
			return
		}
		records = append(records, visible...)
	}
	sort.Slice(records, func(left, right int) bool {
		if records[left].ID != records[right].ID {
			return records[left].ID < records[right].ID
		}
		if records[left].Scope.Kind != records[right].Scope.Kind {
			return records[left].Scope.Kind < records[right].Scope.Kind
		}
		return records[left].Scope.ID < records[right].Scope.ID
	})
	c.JSON(http.StatusOK, contract.ResourcesResponse{Records: ResourceRecordPayloadsFromRaw(records)})
}

// PutWindowManagerLayoutProfile creates or replaces one route-workspace-visible profile.
func (h *BaseHandlers) PutWindowManagerLayoutProfile(c *gin.Context) {
	workspaceID := windowManagerWorkspace(c)
	service, ok := h.windowManagerLayoutProfileService(c, workspaceID)
	if !ok {
		return
	}

	profileID, err := windowManagerLayoutProfileID(c)
	if err != nil {
		h.respondError(c, statusForResourceRequestError(err), err)
		return
	}
	var request contract.PutResourceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode layout profile put request: %w", h.transportName(), err),
		)
		return
	}
	draft, err := parseResourcePutDraft(windowmanager.WindowLayoutResourceKind, profileID, request)
	if err != nil {
		h.respondError(c, statusForResourceRequestError(err), err)
		return
	}
	if !windowManagerLayoutProfileScopeVisible(draft.Scope, workspaceID) {
		err := fmt.Errorf(
			"%w: layout profile scope %q is outside workspace %q",
			resources.ErrPermissionDenied,
			draft.Scope.ID,
			workspaceID,
		)
		h.respondError(c, StatusForResourceError(err), err)
		return
	}
	if err := ensureWindowManagerLayoutProfileIDVisible(
		c.Request.Context(), service, workspaceID, profileID,
	); err != nil {
		h.respondError(c, StatusForResourceError(err), err)
		return
	}

	record, err := service.Put(c.Request.Context(), draft)
	if err != nil {
		h.respondError(c, StatusForResourceError(err), err)
		return
	}
	status := http.StatusOK
	if draft.ExpectedVersion == 0 {
		status = http.StatusCreated
	}
	c.JSON(status, contract.ResourceResponse{Record: ResourceRecordPayloadFromRaw(record)})
}

// DeleteWindowManagerLayoutProfile deletes one route-workspace-visible profile.
func (h *BaseHandlers) DeleteWindowManagerLayoutProfile(c *gin.Context) {
	workspaceID := windowManagerWorkspace(c)
	service, ok := h.windowManagerLayoutProfileService(c, workspaceID)
	if !ok {
		return
	}

	profileID, err := windowManagerLayoutProfileID(c)
	if err != nil {
		h.respondError(c, statusForResourceRequestError(err), err)
		return
	}
	var request contract.DeleteResourceRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.respondError(
			c,
			http.StatusBadRequest,
			fmt.Errorf("%s: decode layout profile delete request: %w", h.transportName(), err),
		)
		return
	}
	if request.ExpectedVersion <= 0 {
		err := fmt.Errorf("%w: expected_version must be positive", resources.ErrValidation)
		h.respondError(c, statusForResourceRequestError(err), err)
		return
	}
	record, err := service.Get(c.Request.Context(), windowmanager.WindowLayoutResourceKind, profileID)
	if err != nil {
		h.respondError(c, StatusForResourceError(err), err)
		return
	}
	if !windowManagerLayoutProfileScopeVisible(record.Scope, workspaceID) {
		h.respondError(c, http.StatusNotFound, resources.ErrNotFound)
		return
	}
	if err := service.Delete(
		c.Request.Context(),
		windowmanager.WindowLayoutResourceKind,
		profileID,
		request.ExpectedVersion,
	); err != nil {
		h.respondError(c, StatusForResourceError(err), err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *BaseHandlers) windowManagerLayoutProfileService(
	c *gin.Context,
	workspaceID windowmanager.WorkspaceID,
) (ResourceService, bool) {
	if h.WindowManager == nil {
		h.respondWindowManagerError(c, workspaceID, windowmanager.ErrClosed)
		return nil, false
	}
	snapshot, err := h.WindowManager.Snapshot(c.Request.Context(), workspaceID)
	if err != nil {
		h.respondWindowManagerError(c, workspaceID, err)
		return nil, false
	}
	if snapshot.WorkspaceID != workspaceID {
		h.respondWindowManagerError(
			c,
			workspaceID,
			fmt.Errorf("snapshot workspace %q does not match route workspace %q", snapshot.WorkspaceID, workspaceID),
		)
		return nil, false
	}
	if h.Resources == nil {
		h.respondError(c, http.StatusServiceUnavailable, errResourceServiceUnavailable)
		return nil, false
	}
	return h.Resources, true
}

func ensureWindowManagerLayoutProfileIDVisible(
	ctx context.Context,
	service ResourceService,
	workspaceID windowmanager.WorkspaceID,
	profileID string,
) error {
	record, err := service.Get(ctx, windowmanager.WindowLayoutResourceKind, profileID)
	if errors.Is(err, resources.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !windowManagerLayoutProfileScopeVisible(record.Scope, workspaceID) {
		return resources.ErrNotFound
	}
	return nil
}

func windowManagerLayoutProfileID(c *gin.Context) (string, error) {
	profileID := strings.TrimSpace(c.Param("profile_id"))
	if profileID == "" {
		return "", fmt.Errorf("%w: profile_id is required", resources.ErrValidation)
	}
	return profileID, nil
}

func windowManagerLayoutProfileScopeVisible(
	scope resources.ResourceScope,
	workspaceID windowmanager.WorkspaceID,
) bool {
	normalized := scope.Normalize()
	return normalized.Kind == resources.ResourceScopeKindGlobal ||
		(normalized.Kind == resources.ResourceScopeKindWorkspace && normalized.ID == string(workspaceID))
}
