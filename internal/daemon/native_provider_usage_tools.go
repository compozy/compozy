package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/api/core"
	"github.com/compozy/compozy/internal/providerusage"
	"github.com/compozy/compozy/internal/store"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

func (n *daemonNativeTools) providerUsageToolBindings() map[toolspkg.ToolID]nativeToolBinding {
	available := n.dependencyAvailability(func() bool { return n.deps.ProviderUsage != nil })
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDProviderUsageList:    {call: n.providerUsageList, availability: available},
		toolspkg.ToolIDProviderUsageRefresh: {call: n.providerUsageRefresh, availability: available},
	}
}

func (n *daemonNativeTools) providerUsageList(
	ctx context.Context, scope toolspkg.Scope, req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input struct {
		ProviderID string `json:"provider_id"`
	}
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	envelope, err := n.deps.ProviderUsage.Envelope(ctx, providerUsageProfileID(scope))
	if err == nil {
		envelope, err = core.ProviderUsageView(envelope, input.ProviderID, &n.deps.Config)
	}
	return providerUsageToolResult(req.ToolID, envelope, err)
}

func (n *daemonNativeTools) providerUsageRefresh(
	ctx context.Context, scope toolspkg.Scope, req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input struct {
		ProviderIDs   []string `json:"provider_ids"`
		MaxAgeSeconds int64    `json:"max_age_seconds"`
	}
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	refresh, err := core.ProviderUsageRefreshInput(contract.ProviderUsageRefreshRequest{
		Profile: providerUsageProfileID(scope), ProviderIDs: input.ProviderIDs,
		MaxAgeSeconds: input.MaxAgeSeconds, Intent: string(providerusage.IntentBackground),
	})
	if err != nil {
		return providerUsageToolResult(req.ToolID, providerusage.Envelope{}, err)
	}
	envelope, err := n.deps.ProviderUsage.Refresh(ctx, refresh)
	if err == nil {
		envelope, err = core.ProviderUsageView(envelope, "", &n.deps.Config)
	}
	return providerUsageToolResult(req.ToolID, envelope, err)
}

func providerUsageProfileID(scope toolspkg.Scope) string {
	if scope.ProfileID == "" {
		return store.DefaultProfileID
	}
	return scope.ProfileID
}

func providerUsageToolResult(
	id toolspkg.ToolID, envelope providerusage.Envelope, err error,
) (toolspkg.ToolResult, error) {
	if err != nil {
		usage := providerusage.AsUsageError(err, "provider")
		return toolspkg.ToolResult{}, toolspkg.NewToolError(toolspkg.ErrorCode(usage.Code), id, usage.Message, err)
	}
	payload := core.ProviderUsageEnvelopePayload(envelope)
	return structuredResult(payload, fmt.Sprintf("%d provider usage targets", len(payload.Targets)))
}
