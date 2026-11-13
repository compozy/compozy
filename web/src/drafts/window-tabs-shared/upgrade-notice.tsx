/**
 * S10: the notice of a failed move to tabs, driven by the migration status. A successful move is
 * announced in the bell (`layout-upgrade`), not here.
 */
import { RefreshCw, X } from "@compozy/ui/icons";

import { ActionResultBanner, Button, ConfirmDialog } from "@compozy/ui";

export type UpgradeNoticeState = "failed" | "retrying" | "layout-changed" | "replace-confirm";

export function UpgradeNotice({
  state,
  onDismiss,
  onRetry,
}: {
  state: UpgradeNoticeState;
  onDismiss?: () => void;
  onRetry?: () => void;
}) {
  if (state === "layout-changed" || state === "replace-confirm") {
    return (
      <div className="border-b border-line bg-canvas px-3 py-2" data-slot="os-upgrade-notice">
        <ActionResultBanner
          actions={
            <>
              <Button size="sm" variant="neutral">
                Replace current layout
              </Button>
              <Button aria-label="Dismiss" onClick={onDismiss} size="icon-sm" variant="quiet">
                <X aria-hidden="true" className="size-3.75" />
              </Button>
            </>
          }
          description="Your previous layout is saved and nothing was lost. This layout changed since, so moving the saved one to tabs replaces it."
          title="We could not move your layout to tabs"
          tone="warning"
        />
        <ConfirmDialog
          cancelLabel="Cancel"
          confirmLabel="Replace layout"
          description="The tabs and windows you opened since the upgrade close, and your previous layout opens in their place as tabs."
          onConfirm={() => undefined}
          open={state === "replace-confirm"}
          title="Replace current layout?"
          tone="destructive"
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
