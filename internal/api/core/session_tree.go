package core

import (
	"context"
	"errors"
	"net/http"
	"slices"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/sessionprocess"
	"github.com/gin-gonic/gin"
)

const withChildrenQuery = "with_children"

var errSessionTreeManagerRequired = errors.New("api: session tree manager is required")

// ListSessionDescendants lists the sessions with_children would include. The
// catalog is paginated and filtered, so clients cannot derive this from it.
func (h *BaseHandlers) ListSessionDescendants(c *gin.Context) {
	tree, ok := h.Sessions.(SessionTreeManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errSessionTreeManagerRequired)
		return
	}
	scope, sessionID, existing, ok := h.routeSessionInWorkspace(c)
	if !ok {
		return
	}
	profileScope, err := h.resolveProfileReadScope(c)
	if err != nil {
		h.respondProfileReadScopeError(c, err)
		return
	}
	if !h.requireSessionInProfile(c, existing, profileScope) {
		return
	}
	descendants, err := tree.Descendants(c.Request.Context(), scope.SessionWorkspaceID(), sessionID)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	payloads := make([]contract.SessionPayload, 0, len(descendants))
	for _, info := range descendants {
		payloads = append(payloads, SessionPayloadFromInfo(info))
	}
	h.decorateProcessSummaries(payloads)
	c.JSON(http.StatusOK, contract.SessionsResponse{Sessions: payloads})
}

func (h *BaseHandlers) archiveSessionTree(
	c *gin.Context,
	workspaceID string,
	root *session.Info,
	policy contract.SessionProcessPolicy,
) {
	tree, ok := h.Sessions.(SessionTreeManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errSessionTreeManagerRequired)
		return
	}
	stop, err := parseBoolQuery(c, "stop")
	if err != nil {
		h.respondError(c, http.StatusBadRequest, err)
		return
	}
	archived, err := tree.ArchiveWithChildren(
		c.Request.Context(), workspaceID, root.ID, session.ArchiveTreeOptions{Stop: stop},
	)
	if err != nil {
		h.continueTreeProcessCleanup(c, tree, root, policy, err)
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	if !h.finishTreeProcessPolicy(c, slices.Concat([]*session.Info{root}, archived.Descendants), policy) {
		return
	}
	h.respondArchivedSession(c, archived.Root)
}

// continueTreeProcessCleanup reads the tree after the failure, so a child that
// joined while the stop ran is enrolled too.
func (h *BaseHandlers) continueTreeProcessCleanup(
	c *gin.Context,
	tree SessionTreeManager,
	root *session.Info,
	policy contract.SessionProcessPolicy,
	err error,
) {
	h.continueProcessCleanup(root.ID, policy, err)
	if policy != contract.SessionProcessesTerminate {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(c.Request.Context()), sessionprocess.CaptureTimeout)
	defer cancel()
	descendants, readErr := tree.Descendants(ctx, root.WorkspaceID, root.ID)
	if readErr != nil && h.Logger != nil {
		h.Logger.Warn("Read session tree for process cleanup", "session_id", root.ID, "error", readErr)
	}
	for _, member := range descendants {
		h.continueProcessCleanup(member.ID, policy, err)
	}
}

// deleteSessionTree settles the process policy for every session it deleted,
// also after a partial failure, since those sessions cannot be retried.
func (h *BaseHandlers) deleteSessionTree(
	c *gin.Context,
	root *session.Info,
	policy contract.SessionProcessPolicy,
) {
	tree, ok := h.Sessions.(SessionTreeManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errSessionTreeManagerRequired)
		return
	}
	deleted, deleteErr := tree.DeleteWithChildren(c.Request.Context(), root.WorkspaceID, root.ID)
	if errors.Is(deleteErr, session.ErrDeleteStopAccepted) {
		h.continueTreeProcessCleanup(c, tree, root, policy, deleteErr)
	}
	settled := make([]*session.Info, 0, len(deleted))
	for _, id := range deleted {
		settled = append(settled, &session.Info{ID: id, ProfileID: root.ProfileID, WorkspaceID: root.WorkspaceID})
	}
	if !h.finishTreeProcessPolicy(c, settled, policy) {
		return
	}
	if deleteErr != nil {
		h.respondError(c, StatusForSessionError(deleteErr), deleteErr)
		return
	}
	c.Status(http.StatusNoContent)
}
