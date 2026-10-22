package e2e

import (
	"context"
	"net/http"
	"net/url"

	compozycontract "github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/workspaceaccess"
)

// GetUserSession fetches one User-scope session through its `@user` route.
func (h *RuntimeHarness) GetUserSession(ctx context.Context, sessionID string) (compozycontract.SessionPayload, error) {
	var response compozycontract.SessionResponse
	if err := h.userSessionJSON(ctx, sessionID, "", &response); err != nil {
		return compozycontract.SessionPayload{}, err
	}
	return response.Session, nil
}

// UserSessionTranscript fetches the transcript of one User-scope session.
func (h *RuntimeHarness) UserSessionTranscript(
	ctx context.Context,
	sessionID string,
) (compozycontract.SessionTranscriptResponse, error) {
	var response compozycontract.SessionTranscriptResponse
	if err := h.userSessionJSON(ctx, sessionID, "/transcript", &response); err != nil {
		return compozycontract.SessionTranscriptResponse{}, err
	}
	return response, nil
}

func (h *RuntimeHarness) userSessionJSON(ctx context.Context, sessionID, suffix string, response any) error {
	path, err := h.workspaceScopedAPIPath(workspaceaccess.UserSelector, "/sessions/"+url.PathEscape(sessionID)+suffix)
	if err != nil {
		return err
	}
	return h.UDSJSON(ctx, http.MethodGet, path, nil, response)
}
