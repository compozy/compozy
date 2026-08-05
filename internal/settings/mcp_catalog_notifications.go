package settings

import (
	"context"
	"strings"
	"time"

	diagnosticcontract "github.com/compozy/compozy/internal/diagnosticcontract"
	"github.com/compozy/compozy/internal/diagnostics"
	"github.com/compozy/compozy/internal/marketplace"
)

const mcpInstallNotificationTimeout = 5 * time.Second

func (s *service) notifyMCPCatalogInstalled(
	ctx context.Context,
	entryID string,
) *diagnosticcontract.DiagnosticItem {
	if s == nil || s.marketplaceInstallEvents == nil {
		return nil
	}
	notifyCtx, cancel := mcpInstallNotificationContext(ctx)
	defer cancel()
	if err := s.marketplaceInstallEvents.NotifyInstall(notifyCtx, marketplace.InstallOutcome{
		Kind:       marketplace.KindMCP,
		EntryID:    strings.TrimSpace(entryID),
		Outcome:    marketplace.InstallOutcomeSucceeded,
		PolicyGate: marketplace.InstallPolicyGatePassed,
	}); err != nil {
		item := diagnostics.NewItem(diagnostics.ItemSpec{
			ID:       "mcp.install.event_persist_failed",
			Code:     diagnosticcontract.CodeMCPInstallEventPersistFailed,
			Category: diagnosticcontract.CategoryMCP,
			Title:    "MCP install event was not persisted",
			Message: "The MCP server is installed, but its marketplace install event could not be persisted: " +
				err.Error(),
			Severity:      diagnosticcontract.SeverityWarn,
			DataFreshness: diagnosticcontract.FreshnessLive,
		})
		return &item
	}
	return nil
}

func mcpInstallNotificationContext(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(ctx), mcpInstallNotificationTimeout)
}
