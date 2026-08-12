package daemon

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/compozy/compozy/internal/api/contract"
	core "github.com/compozy/compozy/internal/api/core"
	taskpkg "github.com/compozy/compozy/internal/task"
	toolspkg "github.com/compozy/compozy/internal/tools"
)

type marketplaceSearchInput struct {
	Query  string `json:"query"`
	Kind   string `json:"kind"`
	Limit  int    `json:"limit"`
	Cursor string `json:"cursor"`
}

func (n *daemonNativeTools) marketplaceToolBindings(
	availability toolspkg.NativeAvailabilityFunc,
) map[toolspkg.ToolID]nativeToolBinding {
	return map[toolspkg.ToolID]nativeToolBinding{
		toolspkg.ToolIDMarketplaceSearch: {
			call:         n.marketplaceSearch,
			availability: availability,
		},
	}
}

func (n *daemonNativeTools) marketplaceSearch(
	ctx context.Context,
	scope toolspkg.Scope,
	req toolspkg.CallRequest,
) (toolspkg.ToolResult, error) {
	var input marketplaceSearchInput
	if err := decodeNativeInput(req, &input); err != nil {
		return toolspkg.ToolResult{}, err
	}
	readScope, workspaceID := marketplaceToolScope(scope)
	var actor *taskpkg.ActorContext
	if workspaceID != "" {
		resolvedActor, actorErr := nativeExtensionScopedActorContext(scope, req)
		if actorErr != nil {
			return toolspkg.ToolResult{}, marketplaceNativeError(req.ToolID, actorErr)
		}
		actor = &resolvedActor
	}
	handlers := n.marketplaceHandlers()
	if handlers == nil {
		return toolspkg.ToolResult{}, marketplaceNativeError(
			req.ToolID,
			errors.Join(core.ErrMarketplaceUnavailable, errors.New("marketplace dependencies are not configured")),
		)
	}
	if kind := strings.TrimSpace(input.Kind); kind != "" {
		response, err := handlers.MarketplaceKind(ctx, core.MarketplaceKindRequest{
			Kind: kind, Query: input.Query, Limit: input.Limit, Cursor: input.Cursor,
			Scope: readScope, WorkspaceID: workspaceID, Actor: actor,
		})
		if err != nil {
			return toolspkg.ToolResult{}, marketplaceNativeError(req.ToolID, err)
		}
		return structuredResult(response, fmt.Sprintf("%d %s marketplace entries", len(response.Items), response.Kind))
	}
	if strings.TrimSpace(input.Cursor) != "" {
		return toolspkg.ToolResult{}, marketplaceNativeError(
			req.ToolID,
			errors.Join(core.ErrMarketplaceValidation, errors.New("marketplace cursor requires kind")),
		)
	}
	response, err := handlers.MarketplaceSearch(ctx, core.MarketplaceSearchRequest{
		Query: input.Query, Limit: input.Limit, Scope: readScope, WorkspaceID: workspaceID, Actor: actor,
	})
	if err != nil {
		return toolspkg.ToolResult{}, marketplaceNativeError(req.ToolID, err)
	}
	count := 0
	for _, result := range response.Kinds {
		count += len(result.Items)
	}
	return structuredResult(response, fmt.Sprintf("%d marketplace entries", count))
}

func (n *daemonNativeTools) marketplaceHandlers() *core.BaseHandlers {
	if n == nil || n.deps == nil {
		return nil
	}
	var settings core.SettingsService
	if n.deps.Settings != nil {
		settings = n.deps.Settings()
	}
	return core.NewBaseHandlers(&core.BaseHandlerConfig{
		MarketplaceCatalog:        n.deps.MarketplaceCatalog,
		SkillMarketplace:          n.deps.MarketplaceSkills,
		InstalledSkillMarketplace: n.deps.MarketplaceInstalledSkills,
		Extensions:                n.extensionCoreService(),
		Settings:                  settings,
		HomePaths:                 n.deps.HomePaths,
		Config:                    n.deps.Config,
	})
}

func marketplaceToolScope(scope toolspkg.Scope) (string, string) {
	workspaceID := strings.TrimSpace(scope.WorkspaceID)
	if workspaceID == "" {
		return contract.MarketplaceScopeGlobal, ""
	}
	return contract.MarketplaceScopeWorkspace, workspaceID
}

func marketplaceNativeError(id toolspkg.ToolID, err error) error {
	switch {
	case errors.Is(err, core.ErrMarketplaceValidation):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeInvalidInput,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolInvalidInput, err),
		)
	case errors.Is(err, core.ErrMarketplaceNotFound):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeNotFound,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolNotFound, err),
		)
	case errors.Is(err, core.ErrMarketplaceUnavailable):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeUnavailable,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolUnavailable, err),
		)
	case errors.Is(err, taskpkg.ErrPermissionDenied):
		return toolspkg.NewToolError(
			toolspkg.ErrorCodeDenied,
			id,
			err.Error(),
			fmt.Errorf("%w: %w", toolspkg.ErrToolDenied, err),
			toolspkg.ReasonSessionDenied,
		)
	default:
		return err
	}
}
