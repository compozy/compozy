/** S10: the one-time notice after the layout moved to tabs, driven by the migration status. */
import { RefreshCw, X } from "@compozy/ui/icons";

import { ActionResultBanner, Button } from "@compozy/ui";

export type UpgradeNoticeState = "migrated" | "failed" | "retrying";

export function UpgradeNotice({
  state,
  onDismiss,
  onRetry,
}: {
  state: UpgradeNoticeState;
  onDismiss?: () => void;
  onRetry?: () => void;
}) {
  if (state === "migrated") {
    return (
      <div className="border-b border-line bg-canvas px-3 py-2" data-slot="os-upgrade-notice">
        <ActionResultBanner
          actions={
            <Button aria-label="Dismiss" onClick={onDismiss} size="icon-sm" variant="quiet">
              <X aria-hidden="true" className="size-3.75" />
            </Button>
          }
          description="3 floating windows and 2 minimized windows are tabs on this desktop. 1 pin inside a split was removed."
          title="Your windows are now tabs"
          tone="info"
        />
      </div>
    );
  }
  const retrying = state === "retrying";
  return (
    <div className="border-b border-line bg-canvas px-3 py-2" data-slot="os-upgrade-notice">
      <ActionResultBanner
        actions={
          <Button disabled={retrying} onClick={onRetry} size="sm" variant="neutral">
            <RefreshCw aria-hidden="true" className={retrying ? "animate-spin" : undefined} />
            {retrying ? "Retrying" : "Retry"}
          </Button>
        }
        description="Your previous layout is saved and nothing was lost. This desktop started empty so you can keep working."
        title="We could not move your layout to tabs"
        tone="warning"
      />
    </div>
  );
}
