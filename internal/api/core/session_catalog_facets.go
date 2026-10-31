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
	if !h.requireOperatorOrGrantedAgent(c, "session catalog facets") {
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
		ByProfile:   make([]contract.SessionProfileFacetsPayload, 0, len(facets.ByProfile)),
		ByWorkspace: make([]contract.SessionWorkspaceFacetsPayload, 0, len(facets.ByWorkspace)),
		Later:       facets.Later,
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
	for _, profile := range facets.ByProfile {
		response.ByProfile = append(
			response.ByProfile,
			contract.SessionProfileFacetsPayload{
				ProfileID: profile.ProfileID,
				Facets:    sessionFacetPayload(profile.Facets),
			},
		)
	}
	c.JSON(http.StatusOK, response)
}

func sessionFacetPayload(facets store.SessionCatalogFacets) contract.SessionCatalogFacetsPayload {
	return contract.SessionCatalogFacetsPayload{
		Board: contract.SessionBoardCountsPayload{
			NeedsYou: facets.Board.NeedsYou,
			Working:  facets.Board.Working,
			Waiting:  facets.Board.Waiting,
			InReview: facets.Board.InReview,
			Done:     facets.Board.Done,
		},
		All:               facets.All,
		NeedsYou:          facets.NeedsYou,
		Working:           facets.Working,
		Finished:          facets.Finished,
		Idle:              facets.Idle,
		TerminalApprovals: facets.TerminalApprovals,
	}
}
