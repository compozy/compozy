package core

import (
	"context"
	"errors"

	"github.com/compozy/compozy/internal/api/contract"
	settingspkg "github.com/compozy/compozy/internal/settings"
)

func (h *BaseHandlers) marketplaceExtensions(
	ctx context.Context,
	scope marketplaceReadScope,
) ([]contract.ExtensionPayload, error) {
	if h.Extensions == nil {
		return nil, errors.Join(
			ErrMarketplaceUnavailable,
			errors.New("extension service is not configured"),
		)
	}
	if scope.scope == settingspkg.ScopeUser {
		return h.Extensions.List(ctx)
	}
	scoped, ok := h.Extensions.(ExtensionScopedReadService)
	if !ok {
		return nil, errors.Join(
			ErrMarketplaceUnavailable,
			errors.New("scoped extension reads are not configured"),
		)
	}
	if err := validateMarketplaceReadActor(scope, scope.actor); err != nil {
		return nil, err
	}
	return scoped.ListScoped(ctx, *scope.actor)
}
