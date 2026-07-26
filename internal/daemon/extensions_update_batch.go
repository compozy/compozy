package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/agh/internal/api/contract"
	extensionpkg "github.com/compozy/agh/internal/extension"
	taskpkg "github.com/compozy/agh/internal/task"
)

func (s *daemonExtensionService) finalizeMarketplaceUpdateBatch(
	ctx context.Context,
	actor taskpkg.ActorContext,
	items []extensionpkg.MarketplaceUpdateResult,
	updateErr error,
) ([]contract.ManagedExtensionUpdatePayload, error) {
	payloads := make([]contract.ManagedExtensionUpdatePayload, 0, len(items))
	resultErr := updateErr
	for _, value := range items {
		item := extensionUpdatePayload(value)
		payloads = append(payloads, item)
		if item.Status != extensionpkg.MarketplaceUpdateStatusUpdated {
			continue
		}
		if err := s.recordExtensionUpdateEvent(ctx, actor, item); err != nil {
			resultErr = errors.Join(
				resultErr,
				fmt.Errorf("daemon: record committed extension update %q: %w", item.Name, err),
			)
		}
	}
	return payloads, resultErr
}
