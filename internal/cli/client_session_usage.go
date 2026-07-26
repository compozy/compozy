package cli

import (
	"context"
	"net/http"

	"github.com/compozy/agh/internal/api/contract"
)

// SessionUsageRecord is the shared aggregated session usage payload.
type SessionUsageRecord = contract.SessionUsagePayload

func (c *unixSocketClient) GetSessionUsage(ctx context.Context, id string) (SessionUsageRecord, error) {
	var response contract.SessionUsageResponse
	path, err := c.sessionScopedPath(ctx, id, "/usage")
	if err != nil {
		return SessionUsageRecord{}, err
	}
	if err := c.doJSON(ctx, http.MethodGet, path, nil, nil, &response); err != nil {
		return SessionUsageRecord{}, err
	}
	return response.Usage, nil
}
