import { ActionResultBanner, Button } from "@compozy/ui";
import { RefreshCw, X } from "@compozy/ui/icons";

import { useOsUpgradeNotice, type UpgradeNoticeState } from "../hooks/use-os-upgrade-notice";
import { upgradeNoticeDescription } from "../lib/upgrade-notice-description";
import type { WindowManagerMigrationSummary } from "../lib/window-manager-types";

const NOTICE_FRAME = "border-b border-line bg-canvas px-3 py-2";

/** S10: the one-time notice after the layout moved to tabs, and its failure variant with Retry. */
export function OsUpgradeNoticeView({
  state,
  summary,
  onDismiss,
  onRetry,
}: {
  state: UpgradeNoticeState;
  summary: WindowManagerMigrationSummary | null;
  onDismiss: () => void;
  onRetry: () => void;
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
        title="We could not move your layout to tabs"
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
      onDismiss={notice.dismiss}
      onRetry={notice.retry}
    />
  );
}
