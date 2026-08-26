package cli

import (
	"context"
	"net/http"
	"net/url"

	"github.com/compozy/compozy/internal/api/contract"
)

// SessionWaitRequest is the stable bounded wait request.
type SessionWaitRequest = contract.SessionWaitRequest

// SessionWaitRecord is the stable bounded wait outcome.
type SessionWaitRecord = contract.SessionWaitResponse

// SessionPromptCancelRecord is the stable idempotent cancellation outcome.
type SessionPromptCancelRecord = contract.SessionPromptCancelResponse

func (c *daemonClient) WaitSession(
	ctx context.Context,
	workspaceID string,
	sessionID string,
	request SessionWaitRequest,
) (SessionWaitRecord, error) {
	workspaceRef, err := requireNetworkPathValue("workspace_id", workspaceID)
	if err != nil {
		return SessionWaitRecord{}, err
	}
	sessionRef, err := requireNetworkPathValue("session_id", sessionID)
	if err != nil {
		return SessionWaitRecord{}, err
	}
	path := "/api/workspaces/" + url.PathEscape(workspaceRef) + "/sessions/" +
		url.PathEscape(sessionRef) + "/wait"
	var response SessionWaitRecord
	if err := c.doBoundedWaitJSON(
		ctx,
		http.MethodPost,
		path,
		nil,
		request,
		boundedWaitDuration(request.TimeoutMS),
		&response,
	); err != nil {
		return SessionWaitRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) CancelSessionPrompt(
	ctx context.Context,
	sessionID string,
) (SessionPromptCancelRecord, error) {
	path, err := c.sessionScopedPath(ctx, sessionID, "/prompt/cancel")
	if err != nil {
		return SessionPromptCancelRecord{}, err
	}
	var response SessionPromptCancelRecord
	if err := c.doJSON(ctx, http.MethodPost, path, nil, nil, &response); err != nil {
		return SessionPromptCancelRecord{}, err
	}
	return response, nil
}
