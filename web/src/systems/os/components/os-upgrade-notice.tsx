import { ActionResultBanner, Button } from "@compozy/ui";
import { RefreshCw, X } from "@compozy/ui/icons";

import { useOsUpgradeNotice, type UpgradeNoticeState } from "../hooks/use-os-upgrade-notice";
import type { WindowManagerMigrationSummary } from "../lib/window-manager-types";

function count(value: number, one: string, many: string): string {
  return `${value} ${value === 1 ? one : many}`;
}

/** What the upgrade did, in the counts the migration status reports. */
export function upgradeNoticeDescription(summary: WindowManagerMigrationSummary | null): string {
  const floating = summary?.floatingToTabs ?? 0;
  const minimized = summary?.minimizedToTabs ?? 0;
  const moved = [
    floating > 0 ? count(floating, "floating window", "floating windows") : null,
    minimized > 0 ? count(minimized, "minimized window", "minimized windows") : null,
  ].filter(part => part !== null);
  const sentences = [
    moved.length === 0
      ? "Floating and minimized windows are tabs now."
      : `${moved.join(" and ")} ${floating + minimized === 1 ? "is a tab" : "are tabs"} now.`,
  ];
  const pins = summary?.pinsDropped ?? 0;
  if (pins === 1) sentences.push("1 pin inside a split was removed.");
  if (pins > 1) sentences.push(`${pins} pins inside splits were removed.`);
  return sentences.join(" ");
}

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
