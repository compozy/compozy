import { CodeHighlightPoolProvider, Tabs, TabsList, TabsTrigger } from "@compozy/ui";
import { createCodeHighlightWorker } from "@compozy/ui/lib/code-highlight-worker";

import { FilesExplorer, fileViewerTarget, requestFilesAppPlace } from "@/systems/files";
import { useOpenFilesApp } from "@/systems/os";

import type { UseSessionReviewPanelStateResult } from "../hooks/use-session-review-panel-state";
import { SessionChangesTab } from "./session-changes/session-changes-tab";
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
 * the explorer over the bound worktree, else the project checkout; Changes is the session's diff.
 */
export function SessionReviewPanel({ sessionId, workspaceId, review }: SessionReviewPanelProps) {
  const openFiles = useOpenFilesApp();
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
            <TabsTrigger value="changes" data-testid="session-review-tab-changes">
              Changes
            </TabsTrigger>
            <TabsTrigger value="files" data-testid="session-review-tab-files">
              Files
            </TabsTrigger>
          </TabsList>
        </div>
        <div className="flex min-h-0 flex-1 flex-col">
          {review.tab === "changes" ? (
            <SessionChangesTab workspaceId={workspaceId} sessionId={sessionId} review={review} />
          ) : (
            <FilesExplorer
              root={{ kind: "session", workspaceId, sessionId }}
              target={review.target}
              onOpenFile={(path, line, find) => review.openFile(fileViewerTarget(path, line, find))}
              treeLabel="Session files"
              onOpenInFilesWindow={
                openFiles
                  ? (root, path) => {
                      requestFilesAppPlace(root.workspace_id, {
                        worktreeId: root.worktree_id ?? null,
                        path,
                      });
                      openFiles(root.workspace_id);
                    }
                  : undefined
              }
            />
          )}
        </div>
      </Tabs>
    </CodeHighlightPoolProvider>
  );
}
