import { CodeHighlightPoolProvider } from "@compozy/ui";
import { createCodeHighlightWorker } from "@compozy/ui/lib/code-highlight-worker";

import type { UseSessionReviewPanelStateResult } from "../hooks/use-session-review-panel-state";
import { SessionChangesTab } from "./session-changes/session-changes-tab";
import { SessionPanelToggle } from "./session-panel-toggle";
import { sessionHasFileRoot } from "../lib/session-file-root";

/**
 * The Review panel toggle; absent for a session without a file root (user scope,
 * US-001.EC-1), where there are no changes to review.
 */
export function SessionReviewPanelToggle({
  workspaceId,
  open,
  onToggle,
}: {
  workspaceId: string | null | undefined;
  open: boolean;
  onToggle: () => void;
}) {
  if (!sessionHasFileRoot(workspaceId)) return null;
  return <SessionPanelToggle panel="review" open={open} onToggle={onToggle} />;
}

export interface SessionReviewPanelProps {
  sessionId: string;
  workspaceId: string;
  review: UseSessionReviewPanelStateResult;
}

/**
 * The session window's right Review panel (`_uiux.md` S1): the session's Changes over its root.
 * Files are browsed in the Files window, where transcript paths and changed files open.
 */
export function SessionReviewPanel({ sessionId, workspaceId, review }: SessionReviewPanelProps) {
  return (
    <CodeHighlightPoolProvider workerFactory={createCodeHighlightWorker}>
      <div
        className="flex h-full min-h-0 flex-col border-l border-line-soft bg-canvas"
        data-testid="session-review-panel"
      >
        {/* Keyed by the checkout: a worktree change starts the Changes state over. */}
        <div key={review.rootKey} className="flex min-h-0 flex-1 flex-col">
          <SessionChangesTab workspaceId={workspaceId} sessionId={sessionId} review={review} />
        </div>
      </div>
    </CodeHighlightPoolProvider>
  );
}
