package daemon

import (
	"context"
	"fmt"

	"github.com/compozy/compozy/internal/api/contract"
	"github.com/compozy/compozy/internal/marketplace"
)

// The completed install owns this observation; only persisted origin qualifies it.
// Event persistence failure does not roll back an already committed installation.
func (s *daemonExtensionService) notifyMarketplaceExtensionInstalled(
	ctx context.Context, item contract.ExtensionPayload,
) error {
	if item.Origin == nil || item.Origin.SourceRef == "" || item.Origin.EntryID == "" {
		return nil
	}
	outcome := marketplace.InstallOutcome{
		EntryID: item.Origin.EntryID,
		Origin:  &marketplace.Origin{SourceRef: item.Origin.SourceRef, EntryID: item.Origin.EntryID},
		Outcome: marketplace.InstallOutcomeSucceeded, PolicyGate: marketplace.InstallPolicyGatePassed,
	}
	if item.Provenance != nil {
		outcome.ResolvedRef = item.Provenance.ResolvedRef
	}
	notifier := &daemonMarketplaceNotifier{writer: s.eventWriter, logger: s.logger, now: s.now}
	if err := notifier.NotifyInstall(ctx, outcome); err != nil {
		return fmt.Errorf(
			"daemon: extension %q is installed but its marketplace event could not be persisted: %w",
			item.Name,
			err,
		)
	}
	return nil
}
