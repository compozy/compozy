import { WorktreeExitProgress } from "./worktree-exit-progress";
import type { WorktreeDetailModel } from "../hooks/use-worktree-detail-context";

/**
 * The single desktop-level report for a detached exit operation.
 *
 * It deliberately lives outside the detail dialog: closing that dialog cannot
 * hide or cancel daemon work that continues independently of the HTTP request.
 */
export function WorktreeExitProgressSurface({ model }: { model: WorktreeDetailModel }) {
  if (!model.progress) return null;

  return (
    <aside
      aria-label="Worktree exit progress"
      className="fixed right-4 bottom-4 z-50 max-w-[min(28rem,calc(100vw-2rem))] rounded-lg border border-line bg-popover p-3 shadow-overlay"
      data-slot="worktree-exit-progress-surface"
    >
      <WorktreeExitProgress
        onCancel={model.onCancelProgress}
        onCta={model.onProgressCta}
        onDismiss={model.onDismissProgress}
        progress={model.progress}
      />
    </aside>
  );
}
