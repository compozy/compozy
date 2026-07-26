package cli

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/compozy/agh/internal/api/contract"
)

func (c *unixSocketClient) ListProviders(ctx context.Context) (contract.ProviderListResponse, error) {
	var response contract.ProviderListResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/providers", nil, nil, &response); err != nil {
		return contract.ProviderListResponse{}, err
	}
	return response, nil
}

func (c *unixSocketClient) ProbeProviderAuth(
	ctx context.Context,
	providerID string,
) (contract.ProviderAuthProbeResponse, error) {
	trimmed := strings.TrimSpace(providerID)
	if trimmed == "" {
		return contract.ProviderAuthProbeResponse{}, fmt.Errorf("provider auth provider_id is required")
	}
	path := "/api/providers/" + url.PathEscape(trimmed) + "/auth/probe"
	var response contract.ProviderAuthProbeResponse
	if err := c.doJSON(ctx, http.MethodPost, path, nil, nil, &response); err != nil {
		return contract.ProviderAuthProbeResponse{}, err
	}
	return response, nil
}
