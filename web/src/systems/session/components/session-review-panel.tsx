import { CodeHighlightPoolProvider, Tabs, TabsList, TabsTrigger } from "@compozy/ui";
import { createCodeHighlightWorker } from "@compozy/ui/lib/code-highlight-worker";

import { FilesExplorer, fileViewerTarget } from "@/systems/files";

import type { UseSessionReviewPanelStateResult } from "../hooks/use-session-review-panel-state";
import { SessionPanelToggle } from "./session-panel-toggle";
import { sessionHasFileRoot } from "../lib/session-file-root";

/**
 * The Review panel toggle; absent for a session without a file root (user scope,
 * US-001.EC-1), where there is nothing to browse.
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
 * The session window's right Review panel (`_uiux.md` S1): tabs over the session root. Files is
 * the explorer over the bound worktree, else the project checkout; the Changes tab joins it.
 */
export function SessionReviewPanel({ sessionId, workspaceId, review }: SessionReviewPanelProps) {
  return (
    <CodeHighlightPoolProvider workerFactory={createCodeHighlightWorker}>
      <Tabs
        value={review.tab}
        onValueChange={value => review.setTab(value as typeof review.tab)}
        className="flex h-full min-h-0 flex-col gap-0 border-l border-line-soft bg-canvas"
        data-testid="session-review-panel"
      >
        <div className="flex h-10 shrink-0 items-center gap-2 border-b border-line-soft px-2">
          <TabsList>
            <TabsTrigger value="files" data-testid="session-review-tab-files">
              Files
            </TabsTrigger>
          </TabsList>
        </div>
        <div className="flex min-h-0 flex-1 flex-col">
          <FilesExplorer
            root={{ kind: "session", workspaceId, sessionId }}
            target={review.target}
            onOpenFile={path => review.openFile(fileViewerTarget(path))}
            treeLabel="Session files"
          />
        </div>
      </Tabs>
    </CodeHighlightPoolProvider>
  );
}
