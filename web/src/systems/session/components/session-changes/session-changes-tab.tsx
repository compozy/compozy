import { useRef, type KeyboardEvent } from "react";

import { DiffViewer, type DiffViewerHandle, type DiffViewerItem } from "@compozy/ui";
import { FileDiff } from "@compozy/ui/icons";

import { useSessionChangesView } from "../../hooks/use-session-changes-view";
import type { UseSessionReviewPanelStateResult } from "../../hooks/use-session-review-panel-state";
import { noChangesText, type SessionChangeItem } from "../../lib/session-changes";
import { SessionChangeFileHeader } from "./session-change-file-header";
import { SessionChangedFilesTree } from "./session-changed-files-tree";
import { SessionChangesHeader } from "./session-changes-header";
import {
  ChangesErrorState,
  ChangesLargeBanner,
  ChangesLoadingState,
  ChangesMessageState,
  ChangesRefreshFailed,
  ChangesTruncatedNote,
} from "./session-changes-states";

function isTextEntry(target: EventTarget): boolean {
  return (
    target instanceof HTMLElement &&
    (target.isContentEditable || ["INPUT", "TEXTAREA", "SELECT"].includes(target.tagName))
  );
}

/** The viewer hands back the item it was given; the summary row rides along on it. */
function asChange(item: DiffViewerItem): SessionChangeItem {
  return item as SessionChangeItem;
}

export interface SessionChangesTabProps {
  workspaceId: string;
  sessionId: string;
  review: UseSessionReviewPanelStateResult;
}

/**
 * The Review panel's Changes tab (`_uiux.md` S2; US-007–US-010): the header row, the
 * virtualized diff list, and the changed-files tree. `[`/`]` step files; Alt+S toggles
 * split/unified.
 */
export function SessionChangesTab({ workspaceId, sessionId, review }: SessionChangesTabProps) {
  const rootRef = useRef<HTMLDivElement>(null);
  const diffRef = useRef<DiffViewerHandle>(null);
  const view = useSessionChangesView({ workspaceId, sessionId, review, rootRef, diffRef });
  const { changes } = view;
  const { summary } = changes;

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (isTextEntry(event.target) || event.metaKey || event.ctrlKey) return;
    if (event.altKey && event.code === "KeyS") {
      event.preventDefault();
      view.toggleMode();
    } else if (!event.altKey && (event.key === "[" || event.key === "]")) {
      event.preventDefault();
      view.step(event.key === "[" ? -1 : 1);
    }
  };

  const body = (() => {
    if (changes.loading) return <ChangesLoadingState />;
    if (!summary) {
      return (
        <ChangesErrorState
          code={changes.errorCode}
          message={changes.errorMessage}
          base={view.baseRef}
          onPickBase={() => view.setBaseOpen(true)}
          onRetry={changes.retry}
        />
      );
    }
    if (summary.files.length === 0) {
      return (
        <ChangesMessageState
          icon={FileDiff}
          title={noChangesText(view.scope, summary.base.ref)}
          testId="changes-empty"
        />
      );
    }
    return (
      <div className="flex min-h-0 flex-1">
        <DiffViewer
          ref={diffRef}
          className="min-w-0 flex-1"
          items={view.shownItems}
          mode={view.mode}
          loadPatch={item => changes.loadPatch(asChange(item))}
          loadContents={item => changes.loadContents(asChange(item))}
          onVisibleItemsChange={view.onVisibleItemsChange}
          fileHeader={(item, state) => {
            const change = asChange(item);
            return (
              <SessionChangeFileHeader
                item={change}
                state={state}
                note={changes.noteOf(change.id)}
                revealed={view.revealed.has(change.path)}
                selected={change.path === view.selectedPath}
                onToggleCollapsed={() => view.toggleCollapsed(change, state.collapsed)}
                onOpenInViewer={() => view.openInViewer(change)}
                onCopyPath={() => view.copyPath(change.path)}
                onReveal={() => view.reveal(change.path)}
                onRetry={() => diffRef.current?.reloadItem(change.id)}
              />
            );
          }}
          data-testid="changes-diff-list"
        />
        {view.treeOpen ? (
          <div className="flex w-60 shrink-0 flex-col border-l border-line-soft">
            <SessionChangedFilesTree
              files={summary.files}
              selectedPath={view.selectedPath}
              onSelect={view.select}
              resetKey={`${summary.scope}|${summary.base.ref}`}
            />
          </div>
        ) : null}
      </div>
    );
  })();

  return (
    <div
      ref={rootRef}
      className="flex h-full min-h-0 flex-col"
      onKeyDown={onKeyDown}
      data-testid="session-changes-tab"
      data-large={view.large ? "true" : undefined}
    >
      <SessionChangesHeader
        workspaceId={workspaceId}
        sessionId={sessionId}
        summary={summary}
        scope={view.scope}
        baseRef={view.baseRef}
        branchAvailable={view.branchAvailable}
        baseOpen={view.baseOpen}
        onBaseOpenChange={view.setBaseOpen}
        onScopeChange={view.setScope}
        onBaseChange={view.setBase}
        refreshing={changes.refreshing}
        onRefresh={changes.refresh}
        diffMode={view.mode}
        diffModeLocked={view.narrow}
        onDiffModeToggle={view.toggleMode}
        allCollapsed={view.allCollapsed}
        onCollapseAllToggle={view.toggleCollapseAll}
        treeOpen={view.treeOpen}
        onTreeToggle={view.toggleTree}
      />
      {changes.refreshFailed ? <ChangesRefreshFailed onRetry={changes.retry} /> : null}
      {summary?.truncated ? (
        <ChangesTruncatedNote shown={summary.files.length} total={summary.totals.files} />
      ) : null}
      {summary && view.large && summary.files.length > 0 ? (
        <ChangesLargeBanner
          index={view.selectedIndex}
          count={summary.files.length}
          onStep={view.step}
        />
      ) : null}
      {body}
    </div>
  );
}
