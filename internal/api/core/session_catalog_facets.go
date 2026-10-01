package core

import (
	"errors"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/session"
	"github.com/compozy/compozy/internal/store"
	"github.com/gin-gonic/gin"
)

// ListSessionFacets returns exact population counts separately from bounded rows.
func (h *BaseHandlers) ListSessionFacets(c *gin.Context) {
	if !h.requireOperatorSurface(c, "session catalog facets") {
		return
	}
	query, _, err := h.parseSessionListQuery(c)
	if err != nil {
		if isProfileDomainError(err) {
			respondProfileError(c, err)
			return
		}
		status := http.StatusBadRequest
		if isSessionListWorkspaceError(err) {
			status = StatusForWorkspaceError(err)
		}
		h.respondError(c, status, err)
		return
	}
	manager, ok := h.Sessions.(SessionCatalogFacetManager)
	if !ok {
		h.respondError(c, http.StatusServiceUnavailable, errors.New("api: session catalog facets are required"))
		return
	}
	facets, err := manager.CatalogFacets(c.Request.Context(), query)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, session.ErrListQueryInvalid) {
			status = http.StatusBadRequest
		}
		h.respondError(c, status, err)
		return
	}
	response := contract.SessionCatalogFacetsResponse{
		Facets:      sessionFacetPayload(facets.Facets),
		ByWorkspace: make([]contract.SessionWorkspaceFacetsPayload, 0, len(facets.ByWorkspace)),
	}
	for _, workspace := range facets.ByWorkspace {
		response.ByWorkspace = append(
			response.ByWorkspace,
			contract.SessionWorkspaceFacetsPayload{
				WorkspaceID: workspace.WorkspaceID,
				Facets:      sessionFacetPayload(workspace.Facets),
			},
		)
	}
	c.JSON(http.StatusOK, response)
}

func sessionFacetPayload(facets store.SessionCatalogFacets) contract.SessionCatalogFacetsPayload {
	return contract.SessionCatalogFacetsPayload{
		All:               facets.All,
		NeedsYou:          facets.NeedsYou,
		Working:           facets.Working,
		Finished:          facets.Finished,
		Idle:              facets.Idle,
		TerminalApprovals: facets.TerminalApprovals,
	}
}
