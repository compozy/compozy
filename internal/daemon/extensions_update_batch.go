package daemon

import (
	"context"
	"errors"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	eventspkg "github.com/compozy/compozy/internal/events"
	extensionpkg "github.com/compozy/compozy/internal/extension"
	taskpkg "github.com/compozy/compozy/internal/task"
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
		eventType := ""
		switch item.Status {
		case extensionpkg.MarketplaceUpdateStatusUpdated:
			eventType = eventspkg.ExtensionUpdateCompleted
		case extensionpkg.MarketplaceUpdateStatusFailed:
			eventType = eventspkg.ExtensionUpdateFailed
		default:
			continue
		}
		if err := s.recordCanonicalExtensionLifecycleEvent(ctx, actor, extensionpkg.LifecycleEvent{
			Type: eventType, ExtensionName: item.Name, SourceKind: item.Registry,
		}); err != nil {
			resultErr = errors.Join(
				resultErr,
				fmt.Errorf("daemon: record extension update %q: %w", item.Name, err),
			)
		}
	}
	return payloads, resultErr
}
