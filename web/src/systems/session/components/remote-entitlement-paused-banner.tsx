import { Sparkles, TriangleAlert } from "@compozy/ui/icons";

import { Button } from "@compozy/ui";

export interface RemoteEntitlementPausedBannerProps {
  computerName: string;
  onUpgrade?: () => void;
}

/**
 * Remote-session entitlement-paused banner shown above the read-only composer
 * when the account's Pro plan ends (`_uiux.md` U6, `remote-entitlement-paused`).
 */
export function RemoteEntitlementPausedBanner({
  computerName,
  onUpgrade,
}: RemoteEntitlementPausedBannerProps) {
  return (
    <div
      className="mx-auto flex w-full max-w-3xl items-center gap-3 px-6 pt-2 pb-5"
      data-testid="remote-entitlement-paused-banner"
      role="status"
    >
      <div className="flex min-w-0 flex-1 items-center justify-between gap-3 rounded-lg border border-line bg-surface-2 px-3 py-2.5 text-small-body">
        <span className="flex min-w-0 items-center gap-2 text-fg-2">
          <TriangleAlert aria-hidden="true" className="size-3.5 shrink-0 text-warning" />
          <span className="truncate">
            <span className="font-medium text-fg">Remote access paused:</span> your Pro plan ended.
            Running agents on {computerName} keep running.
          </span>
        </span>
        {onUpgrade ? (
          <Button className="shrink-0" onClick={onUpgrade} size="xs" type="button" variant="default">
            <Sparkles aria-hidden="true" data-icon="inline-start" />
            Upgrade
          </Button>
        ) : null}
      </div>
    </div>
  );
}
