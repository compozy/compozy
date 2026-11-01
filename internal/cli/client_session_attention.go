package cli

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/compozy/internal/agentidentity"
	"github.com/compozy/compozy/internal/api/contract"
)

// AgentNotifyRequest is the stable agent notification request payload.
type AgentNotifyRequest = contract.AgentNotifyRequest

// AgentNotifyRecord is the daemon-provable notification outcome.
type AgentNotifyRecord = contract.AgentNotifyResponse

func (c *daemonClient) GetSessionAttentionSummary(
	ctx context.Context,
) (SessionAttentionSummaryRecord, error) {
	var response SessionAttentionSummaryRecord
	if err := c.doJSON(ctx, http.MethodGet, "/api/sessions/attention-summary", nil, nil, &response); err != nil {
		return SessionAttentionSummaryRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) ListSessionInteractions(
	ctx context.Context,
	sessionID string,
	statuses []string,
) (SessionInteractionsRecord, error) {
	return c.listSessionInteractions(ctx, sessionID, statuses, false, "/interactions")
}

func (c *daemonClient) ListSessionPendingInteractions(ctx context.Context, sessionID string, descendants bool) (SessionInteractionsRecord, error) {
	return c.listSessionInteractions(ctx, sessionID, nil, descendants, "/pending-interactions")
}

func (c *daemonClient) listSessionInteractions(ctx context.Context, sessionID string, statuses []string, descendants bool, suffix string) (SessionInteractionsRecord, error) {
	path, err := c.sessionScopedPath(ctx, sessionID, suffix)
	if err != nil {
		return SessionInteractionsRecord{}, err
	}
	values := make(url.Values)
	if descendants {
		values.Set("descendants", "true")
	}
	for _, status := range statuses {
		if trimmed := strings.TrimSpace(status); trimmed != "" {
			values.Add("status", trimmed)
		}
	}
	var response SessionInteractionsRecord
	if err := c.doJSON(ctx, http.MethodGet, path, values, nil, &response); err != nil {
		return SessionInteractionsRecord{}, err
	}
	return response, nil
}

func (c *daemonClient) AgentNotify(
	ctx context.Context,
	request AgentNotifyRequest,
	credentials agentidentity.Credentials,
) (AgentNotifyRecord, error) {
	var response AgentNotifyRecord
	if err := c.doAgentJSON(
		ctx,
		http.MethodPost,
		"/api/agent/notify",
		nil,
		request,
		credentials,
		&response,
	); err != nil {
		return AgentNotifyRecord{}, err
	}
	return response, nil
}
