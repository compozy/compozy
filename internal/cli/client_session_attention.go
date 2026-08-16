package cli

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

func (c *daemonClient) ListSessionInteractions(
	ctx context.Context,
	id string,
	statuses []string,
) (SessionInteractionsRecord, error) {
	path, err := c.sessionScopedPath(ctx, id, "/interactions")
	if err != nil {
		return SessionInteractionsRecord{}, err
	}
	query := url.Values{}
	for _, status := range statuses {
		if normalized := strings.TrimSpace(status); normalized != "" {
			query.Add("status", normalized)
		}
	}
	var response SessionInteractionsRecord
	if err := c.doJSON(ctx, http.MethodGet, path, query, nil, &response); err != nil {
		return SessionInteractionsRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) GetSessionAttentionSummary(
	ctx context.Context,
) (SessionAttentionSummaryRecord, error) {
	var response SessionAttentionSummaryRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/sessions/attention-summary", nil, nil, &response); err != nil {
		return SessionAttentionSummaryRecord{}, err
	}
	return response, nil
}
