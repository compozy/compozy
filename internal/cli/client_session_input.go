package cli

import (
	"context"
	"net/http"
	"net/url"
)

// ListSessionInputs returns pending input in the daemon's dispatch order.
func (c *unixSocketClient) ListSessionInputs(
	ctx context.Context,
	sessionID string,
) (SessionInputListRecord, error) {
	var response SessionInputListRecord
	path, err := c.sessionScopedPath(ctx, sessionID, "/prompt/queue")
	if err != nil {
		return SessionInputListRecord{}, err
	}
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return SessionInputListRecord{}, err
	}
	return response, nil
}

// ReplaceSessionInput atomically updates one queued input.
func (c *unixSocketClient) ReplaceSessionInput(
	ctx context.Context,
	sessionID string,
	inputID string,
	request ReplaceSessionInputRequest,
) (SessionInputRecord, error) {
	entryID, err := requireNetworkPathValue("queue_entry_id", inputID)
	if err != nil {
		return SessionInputRecord{}, err
	}
	var response struct {
		Input SessionInputRecord `json:"input"`
	}
	path, err := c.sessionScopedPath(ctx, sessionID, "/prompt/queue/"+url.PathEscape(entryID))
	if err != nil {
		return SessionInputRecord{}, err
	}
	if err := c.doJSON(ctx, http.MethodPut, path, nil, request, &response); err != nil {
		return SessionInputRecord{}, err
	}
	return response.Input, nil
}

// PromoteSessionInput atomically converts queued input into fenced steering input.
func (c *unixSocketClient) PromoteSessionInput(
	ctx context.Context,
	sessionID string,
	inputID string,
	request PromoteSessionInputRequest,
) (SessionPromptRecord, error) {
	entryID, err := requireNetworkPathValue("queue_entry_id", inputID)
	if err != nil {
		return SessionPromptRecord{}, err
	}
	path, err := c.sessionScopedPath(ctx, sessionID, "/prompt/queue/"+url.PathEscape(entryID)+"/steer")
	if err != nil {
		return SessionPromptRecord{}, err
	}
	return c.doSessionPrompt(ctx, http.MethodPost, path, nil, request)
}

// CancelSessionInput removes one pending queued input.
func (c *unixSocketClient) CancelSessionInput(
	ctx context.Context,
	sessionID string,
	inputID string,
) (SessionPromptRecord, error) {
	entryID, err := requireNetworkPathValue("queue_entry_id", inputID)
	if err != nil {
		return SessionPromptRecord{}, err
	}
	path, err := c.sessionScopedPath(ctx, sessionID, "/prompt/queue/"+url.PathEscape(entryID))
	if err != nil {
		return SessionPromptRecord{}, err
	}
	return c.doSessionPrompt(ctx, http.MethodDelete, path, nil, nil)
}
