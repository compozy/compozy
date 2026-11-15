package cli

import (
	"context"
	"net/http"

	"github.com/compozy/compozy/internal/api/contract"
)

const sessionHistoryCleanupPath = "/api/sessions/history-cleanup"

type sessionHistoryCleanupClient interface {
	PreviewSessionHistoryCleanup(
		context.Context,
		contract.SessionHistoryCleanupRequest,
	) (contract.SessionHistoryCleanupPreviewResponse, error)
	StartSessionHistoryCleanup(
		context.Context,
		contract.SessionHistoryCleanupRequest,
	) (contract.SessionHistoryCleanupStartResponse, error)
	GetSessionHistoryCleanup(context.Context) (contract.SessionHistoryCleanupRunResponse, error)
	StopSessionHistoryCleanup(context.Context) (contract.SessionHistoryCleanupRunResponse, error)
}

var _ sessionHistoryCleanupClient = (*daemonClient)(nil)

// PreviewSessionHistoryCleanup plans an archived-history cleanup without deleting anything.
func (c *daemonClient) PreviewSessionHistoryCleanup(
	ctx context.Context,
	request contract.SessionHistoryCleanupRequest,
) (contract.SessionHistoryCleanupPreviewResponse, error) {
	request.DryRun = true
	var response contract.SessionHistoryCleanupPreviewResponse
	err := c.doJSON(ctx, http.MethodPost, sessionHistoryCleanupPath, nil, request, &response)
	return response, err
}

// StartSessionHistoryCleanup starts deleting archived history in the daemon's background.
func (c *daemonClient) StartSessionHistoryCleanup(
	ctx context.Context,
	request contract.SessionHistoryCleanupRequest,
) (contract.SessionHistoryCleanupStartResponse, error) {
	request.DryRun = false
	var response contract.SessionHistoryCleanupStartResponse
	err := c.doJSON(ctx, http.MethodPost, sessionHistoryCleanupPath, nil, request, &response)
	return response, err
}

// GetSessionHistoryCleanup reads the profile's running or most recent cleanup.
func (c *daemonClient) GetSessionHistoryCleanup(
	ctx context.Context,
) (contract.SessionHistoryCleanupRunResponse, error) {
	var response contract.SessionHistoryCleanupRunResponse
	err := c.doJSON(ctx, http.MethodGet, sessionHistoryCleanupPath, nil, nil, &response)
	return response, err
}

// StopSessionHistoryCleanup stops the profile's running cleanup after its deletes in flight.
func (c *daemonClient) StopSessionHistoryCleanup(
	ctx context.Context,
) (contract.SessionHistoryCleanupRunResponse, error) {
	var response contract.SessionHistoryCleanupRunResponse
	err := c.doJSON(ctx, http.MethodPost, sessionHistoryCleanupPath+"/stop", nil, nil, &response)
	return response, err
}
