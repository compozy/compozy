import { Gauge } from "lucide-react";

import { useTopbarSlot } from "@compozy/ui";
import { ProviderUsagePanel } from "@/systems/provider-usage";

import { useProviderUsageSurface } from "../../hooks/use-provider-usage-surface";

/**
 * The Usage app: every target as a collapsed row, live from the same profile
 * stream as the tray, under a slim freshness + Refresh row (the window chrome
 * carries the title), with a link to the provider usage settings.
 * Feature-off renders the panel's turned-off state with that same link.
 */
export function UsageWindow() {
  const surface = useProviderUsageSurface();
  const { usage, actions } = surface;
  useTopbarSlot({ glyph: <Gauge />, crumb: "Usage" });

  return (
    // The Settings content column: bars stay a readable length on wide windows.
    <div
      data-testid="os-usage-app"
      className="mx-auto flex min-h-0 w-full max-w-settings-page-form flex-1 flex-col px-6 pt-4 pb-4"
    >
      <ProviderUsagePanel
        chrome="window"
        title="Usage"
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
