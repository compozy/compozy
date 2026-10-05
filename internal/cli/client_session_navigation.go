package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/transcript"
)

// SearchSessionTranscript searches the session projection across retained history.
func (c *daemonClient) SearchSessionTranscript(
	ctx context.Context,
	id string,
	query transcript.SearchQuery,
) (contract.SessionTranscriptSearchResponse, error) {
	var result contract.SessionTranscriptSearchResponse
	path, err := c.sessionReadPath(ctx, id, "/transcript/search")
	if err != nil {
		return result, err
	}
	query, err = query.Normalize()
	if err != nil {
		return result, err
	}
	values := url.Values{"q": {query.Query}, "limit": {strconv.Itoa(query.Limit)}}
	err = c.doJSON(ctx, http.MethodGet, path, values, nil, &result)
	return result, err
}

// GetSessionOutline returns the full retained operator-message trail.
func (c *daemonClient) GetSessionOutline(
	ctx context.Context,
	id string,
) (contract.SessionTranscriptOutlineResponse, error) {
	var result contract.SessionTranscriptOutlineResponse
	path, err := c.sessionReadPath(ctx, id, "/transcript/outline")
	if err != nil {
		return result, err
	}
	err = c.doJSON(ctx, http.MethodGet, path, nil, nil, &result)
	return result, err
}

// sessionReadPath preserves Global ownership for reads without widening project mutations.
func (c *daemonClient) sessionReadPath(ctx context.Context, id, suffix string) (string, error) {
	sessionID, err := requirePathValue("session_id", id)
	if err != nil {
		return "", err
	}
	record, err := c.GetSession(ctx, sessionID)
	if err != nil {
		return "", fmt.Errorf("cli: resolve session %q history owner: %w", sessionID, err)
	}
	base := "/api"
	if workspaceID := strings.TrimSpace(record.WorkspaceID); workspaceID != "" {
		base += "/workspaces/" + url.PathEscape(workspaceID)
	}
	return base + "/sessions/" + url.PathEscape(sessionID) + suffix, nil
}
