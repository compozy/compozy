import { Gauge } from "lucide-react";

import { useTopbarSlot } from "@compozy/ui";
import { ProviderUsagePanel } from "@/systems/provider-usage";

import { useProviderUsageSurface } from "../../hooks/use-provider-usage-surface";

/**
 * The Usage app: every target expanded, live from the same profile stream as
 * the tray, with a header Refresh and a link to the provider usage settings.
 * Feature-off renders the panel's turned-off state with that same link.
 */
export function UsageWindow() {
  const surface = useProviderUsageSurface();
  const { usage, actions } = surface;
  useTopbarSlot({ glyph: <Gauge />, crumb: "Usage" });

  return (
    <div data-testid="os-usage-app" className="flex min-h-0 flex-1 flex-col p-4">
      <ProviderUsagePanel
        chrome="window"
        title="Usage"
        expandAll
        viewModel={usage.viewModel}
        loading={usage.loading}
        daemonUnavailable={usage.daemonUnavailable}
        errorMessage={usage.errorMessage}
        onRefresh={actions.refreshAll}
        refreshing={actions.refreshing}
        onAction={actions.runAction}
        pendingProviderIds={actions.pendingProviderIds}
        isMacHost={surface.isMacHost}
        remoteClient={surface.remoteClient}
        onOpenSettings={surface.openSettings}
        className="flex-1"
      />
    </div>
  );
}
