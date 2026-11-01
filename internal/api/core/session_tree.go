package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
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
	ctx := c.Request.Context()
	descendants, err := tree.Descendants(ctx, workspaceID, root.ID)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	archived, err := tree.ArchiveWithChildren(ctx, workspaceID, root.ID, stop)
	if err != nil {
		h.continueProcessCleanup(root.ID, policy, err)
		for _, member := range descendants {
			h.continueProcessCleanup(member.ID, policy, err)
		}
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	if !h.finishProcessPolicy(c, root, policy) {
		return
	}
	for _, member := range archived.Descendants {
		if !h.finishProcessPolicy(c, member, policy) {
			return
		}
	}
	h.respondArchivedSession(c, archived.Root)
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
	ctx := c.Request.Context()
	descendants, err := tree.Descendants(ctx, root.WorkspaceID, root.ID)
	if err != nil {
		h.respondError(c, StatusForSessionError(err), err)
		return
	}
	members := make(map[string]*session.Info, len(descendants)+1)
	for _, member := range descendants {
		members[member.ID] = member
	}
	members[root.ID] = root
	deleted, deleteErr := tree.DeleteWithChildren(ctx, root.WorkspaceID, root.ID)
	if errors.Is(deleteErr, session.ErrDeleteStopAccepted) {
		for id := range members {
			h.continueProcessCleanup(id, policy, deleteErr)
		}
	}
	for _, id := range deleted {
		member, known := members[id]
		if !known {
			// A child created after the read shares the root's owner.
			member = &session.Info{ID: id, ProfileID: root.ProfileID, WorkspaceID: root.WorkspaceID}
		}
		if !h.finishProcessPolicy(c, member, policy) {
			return
		}
	}
	if deleteErr != nil {
		h.respondError(c, StatusForSessionError(deleteErr), deleteErr)
		return
	}
	c.Status(http.StatusNoContent)
}
