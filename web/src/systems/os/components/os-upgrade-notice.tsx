import { ActionResultBanner, Button, ConfirmDialog } from "@compozy/ui";
import { RefreshCw, X } from "@compozy/ui/icons";

import { useOsUpgradeNotice, type UpgradeNoticeState } from "../hooks/use-os-upgrade-notice";
import { upgradeNoticeDescription } from "../lib/upgrade-notice-description";
import type { WindowManagerMigrationSummary } from "../lib/window-manager-types";

const NOTICE_FRAME = "border-b border-line bg-canvas px-3 py-2";
const FAILED_TITLE = "We could not move your layout to tabs";

/**
 * S10: the one-time notice after the layout moved to tabs, its failure variant with Retry, and,
 * once the layout changed after the failure, a confirmed "Replace current layout".
 */
export function OsUpgradeNoticeView({
  state,
  summary,
  confirmingReplace = false,
  onDismiss,
  onRetry,
  onRequestReplace,
  onCancelReplace,
  onConfirmReplace,
}: {
  state: UpgradeNoticeState;
  summary: WindowManagerMigrationSummary | null;
  confirmingReplace?: boolean;
  onDismiss: () => void;
  onRetry: () => void;
  onRequestReplace: () => void;
  onCancelReplace: () => void;
  onConfirmReplace: () => void;
}) {
  const dismiss = (
    <Button aria-label="Dismiss" onClick={onDismiss} size="icon-sm" variant="quiet">
      <X aria-hidden="true" className="size-3.75" />
    </Button>
  );
  if (state === "migrated") {
    return (
      <div className={NOTICE_FRAME} data-slot="os-upgrade-notice" data-state={state}>
        <ActionResultBanner
          actions={dismiss}
          description={upgradeNoticeDescription(summary)}
          title="Your windows are now tabs"
          tone="info"
        />
      </div>
    );
  }
  if (state === "layout-changed") {
    return (
      <div className={NOTICE_FRAME} data-slot="os-upgrade-notice" data-state={state}>
        <ActionResultBanner
          actions={
            <>
              <Button onClick={onRequestReplace} size="sm" variant="neutral">
                Replace current layout
              </Button>
              {dismiss}
            </>
          }
          description="Your previous layout is saved and nothing was lost. This layout changed since, so moving the saved one to tabs replaces it."
          title={FAILED_TITLE}
          tone="warning"
        />
        <ConfirmDialog
          cancelLabel="Cancel"
          confirmLabel="Replace layout"
          description="The tabs and windows you opened since the upgrade close, and your previous layout opens in their place as tabs."
          onConfirm={onConfirmReplace}
          onOpenChange={open => {
            if (!open) onCancelReplace();
          }}
          open={confirmingReplace}
          title="Replace current layout?"
          tone="destructive"
        />
      </div>
    );
  }
  const retrying = state === "retrying";
  return (
    <div className={NOTICE_FRAME} data-slot="os-upgrade-notice" data-state={state}>
      <ActionResultBanner
        actions={
          <>
            <Button disabled={retrying} onClick={onRetry} size="sm" variant="neutral">
              <RefreshCw aria-hidden="true" className={retrying ? "animate-spin" : undefined} />
              {retrying ? "Retrying" : "Retry"}
            </Button>
            {dismiss}
          </>
        }
        description="Your previous layout is saved and nothing was lost. This desktop started empty so you can keep working."
        title={FAILED_TITLE}
        tone="warning"
      />
    </div>
  );
}

/** The notice between the tab bar and the content, while the migration status asks for it. */
export function OsUpgradeNotice() {
  const notice = useOsUpgradeNotice();
  if (notice === null) return null;
  return (
    <OsUpgradeNoticeView
      state={notice.state}
      summary={notice.summary}
      confirmingReplace={notice.confirmingReplace}
      onDismiss={notice.dismiss}
      onRetry={notice.retry}
      onRequestReplace={notice.requestReplace}
      onCancelReplace={notice.cancelReplace}
      onConfirmReplace={notice.confirmReplace}
    />
  );
}
