package settings

import (
	"context"
	"strings"

	diagnosticcontract "github.com/compozy/agh/internal/diagnosticcontract"
	"github.com/compozy/agh/internal/diagnostics"
	"github.com/compozy/agh/internal/marketplace"
)

func (s *service) notifyMCPCatalogInstalled(
	ctx context.Context,
	entryID string,
) *diagnosticcontract.DiagnosticItem {
	if s == nil || s.marketplaceInstallEvents == nil {
		return nil
	}
	if err := s.marketplaceInstallEvents.NotifyInstall(context.WithoutCancel(ctx), marketplace.InstallOutcome{
		Kind:       marketplace.KindMCP,
		EntryID:    strings.TrimSpace(entryID),
		Outcome:    marketplace.InstallOutcomeSucceeded,
		PolicyGate: marketplace.InstallPolicyGatePassed,
	}); err != nil {
		item := diagnostics.NewItem(
			"mcp.install.event_persist_failed",
			diagnosticcontract.CodeMCPInstallEventPersistFailed,
			diagnosticcontract.CategoryMCP,
			"MCP install event was not persisted",
			"The MCP server is installed, but its marketplace install event could not be persisted: "+err.Error(),
			diagnosticcontract.SeverityWarn,
			diagnosticcontract.FreshnessLive,
		)
		return &item
	}
	return nil
}
