import { Popover, PopoverContent, PopoverTrigger } from "@compozy/ui";
import { ProviderUsagePanel, ProviderUsageTrayMeter } from "@/systems/provider-usage";

import type { ProviderUsageSurfaceModel } from "../../hooks/use-provider-usage-surface";

export interface MenubarUsagePopoverProps {
  surface: ProviderUsageSurfaceModel;
  /** The tray icon-button footprint, owned by the menubar. */
  triggerClassName: string;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}

/**
 * The tray's Provider usage control and its popover. The meter is the trigger;
 * opening asks the daemon for a background refresh of rows older than 60 s
 * while the cached rows render at once. The footer destinations close the
 * popover before the window opens.
 */
export function MenubarUsagePopover({
  surface,
  triggerClassName,
  open,
  onOpenChange,
}: MenubarUsagePopoverProps) {
  const { usage, actions } = surface;
  const viewModel = usage.viewModel;
  const close = (then: () => void) => () => {
    onOpenChange(false);
    then();
  };

  return (
    <Popover
      open={open}
      onOpenChange={next => {
        if (next) surface.openRefresh();
        onOpenChange(next);
      }}
    >
      <PopoverTrigger
        render={
          <ProviderUsageTrayMeter
            tray={viewModel?.tray ?? null}
            measuredAt={viewModel?.trayMeasuredAt ?? null}
            loading={usage.loading}
            className={triggerClassName}
          />
        }
      />
      <PopoverContent
        align="end"
        className="w-80 max-w-[calc(100vw-16px)] p-2"
        data-testid="os-usage-popover"
      >
        <ProviderUsagePanel
          viewModel={viewModel}
          loading={usage.loading}
          daemonUnavailable={usage.daemonUnavailable}
          errorMessage={usage.errorMessage}
          onRefresh={actions.refreshAll}
          refreshing={actions.refreshing}
          onAction={(providerId, action) => {
            // Settings opens in its own window; the popover gets out of the way.
            if (action === "configure") onOpenChange(false);
            actions.runAction(providerId, action);
          }}
          pendingProviderIds={actions.pendingProviderIds}
          isMacHost={surface.isMacHost}
          remoteClient={surface.remoteClient}
          onOpenUsage={close(surface.openUsage)}
          onOpenSettings={close(surface.openSettings)}
        />
      </PopoverContent>
    </Popover>
  );
}
