import type { SessionThreadBranchChange } from "../../lib/session-thread-branch";
import type { ProfileOwner, ProfileOwnerLabel } from "@/systems/profiles";

import { MessagesSquare } from "@compozy/ui/icons";

import { Empty } from "@compozy/ui";

import { cn } from "@/lib/utils";

import type { SessionListLayout } from "../../hooks/use-session-folders";
import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import {
  folderDropEdge,
  rowDragged,
  rowDropEdge,
  type SessionDropContainer,
  type SessionListDnd,
  sessionRowNestTarget,
  type SessionSelectionDrag,
} from "../../hooks/use-session-list-dnd";
import type { SessionRowSelection } from "../../hooks/use-session-selection";
import {
  sessionHasOwnPlacement,
  shiftId,
  type SessionListSections as Sections,
} from "../../lib/session-folders";
import { countThreadSessions, type SessionThreadModel } from "../../lib/session-list-threads";
import type { SessionFolderPayload, SessionListTarget, SessionPayload } from "../../types";
import type { SessionFolderDialogState } from "../../hooks/use-session-list-folder-layout";
import { SessionListFolder } from "./session-list-folder";
import { SessionListThread, type SessionThreadRootLayout } from "./session-list-thread";

export interface SessionListSectionsProps {
  ownerOf?: (session: ProfileOwnerLabel) => ProfileOwner;
  sections: Sections;
  layout: SessionListLayout;
  dnd?: SessionListDnd;
  /** What dragging a selected row carries; selected rows drag alone without it. */
  selectionDrag?: SessionSelectionDrag;
  /** Manual sort is on and no filter narrows the list, so order verbs apply. */
  orderable: boolean;
  /** A filter is active: every surviving folder opens. */
  filtering: boolean;
  collapsedThreads: ReadonlySet<string>;
  subagentThreads: ReadonlySet<string>;
  collapsedFolders: ReadonlySet<string>;
  currentSessionId?: string;
  onThreadBranchChange: SessionThreadBranchChange;
  onToggleFolder: (folderId: string) => void;
  onSelectSession: (session: SessionListTarget) => void;
  onNewSessionInFolder?: (folderId: string) => void;
  onFolderDialog: (state: SessionFolderDialogState) => void;
  /** Bulk selection; absent where the list is read-only (grouped lists). */
  selection?: SessionRowSelection;
  sessionActions: SessionLifecycleActionHandlers;
  testIdPrefix: string;
  emptyMessage: string;
}

/**
 * The workspace list with folders: folders on top in their manual order, each
 * holding its threads in served order, then the unfoldered root. Only a
 * thread's root row carries folder moves, order verbs, and drag wiring;
 * children always follow their parent.
 */
export function SessionListSections({
  sections,
  ownerOf,
  layout,
  dnd,
  selectionDrag,
  orderable,
  filtering,
  collapsedThreads,
  subagentThreads,
  collapsedFolders,
  currentSessionId,
  onThreadBranchChange,
  onToggleFolder,
  onSelectSession,
  onNewSessionInFolder,
  onFolderDialog,
  selection,
  sessionActions,
  testIdPrefix,
  emptyMessage,
}: SessionListSectionsProps) {
  const folderIds = sections.folders.map(section => section.folder.id);
  const roots = new Map(
    [...sections.folders.flatMap(section => section.threads), ...sections.root].map(thread => [
      thread.session.id,
      thread.session,
    ])
  );
  const nestParents = [
    ...sections.folders.flatMap(section => section.threads),
    ...sections.root,
  ].flatMap(thread => [thread.session, ...thread.childSessions]);
  const allFolderIds = layout.folders.map(folder => folder.id);
  const threadActions = { ...sessionActions, nestParents };
  const renderThreads = (threads: readonly SessionThreadModel[], folderId: string | null) => (
    <SectionThreads
      threads={threads}
      folderId={folderId}
      layout={layout}
      dnd={dnd}
      selectionDrag={selectionDrag}
      orderable={orderable}
      roots={roots}
      ownerOf={ownerOf}
      currentSessionId={currentSessionId}
      collapsedThreads={collapsedThreads}
      subagentThreads={subagentThreads}
      onThreadBranchChange={onThreadBranchChange}
      onSelectSession={onSelectSession}
      selection={selection}
      sessionActions={threadActions}
      testIdPrefix={testIdPrefix}
    />
  );
  const moveFolder = (folder: SessionFolderPayload, delta: -1 | 1) =>
    shiftId(allFolderIds, folder.id, delta) ? () => layout.moveFolder(folder.id, delta) : undefined;
  const rootZone = dnd?.rootZone(selectionDrag);
  const rootTarget = dnd?.indicator?.kind === "root";
  const empty = sections.folders.length === 0 && sections.root.length === 0;

  return (
    <>
      {sections.folders.map(section => {
        const { folder } = section;
        return (
          <SessionListFolder
            key={folder.id}
            folder={folder}
            count={countThreadSessions(section.threads)}
            expanded={filtering || !collapsedFolders.has(folder.id)}
            onToggle={() => onToggleFolder(folder.id)}
            onNewSession={onNewSessionInFolder ? () => onNewSessionInFolder(folder.id) : undefined}
            onRename={() => onFolderDialog({ kind: "rename", folder })}
            onDelete={() => onFolderDialog({ kind: "delete", folder })}
            onMoveUp={folderIds.length > 1 ? moveFolder(folder, -1) : undefined}
            onMoveDown={folderIds.length > 1 ? moveFolder(folder, 1) : undefined}
            drag={dnd?.folderHeader(folder.id)}
            dropZone={dnd?.folderZone(folder.id, selectionDrag)}
            dropEdge={folderDropEdge(dnd?.indicator ?? null, folder.id)}
            dragging={dnd?.dragging?.kind === "folder" && dnd.dragging.id === folder.id}
            testIdPrefix={testIdPrefix}
          >
            {renderThreads(section.threads, folder.id)}
          </SessionListFolder>
        );
      })}
      <div
        {...rootZone}
        data-testid={`${testIdPrefix}-root`}
        className={cn(
          "flex min-h-6 flex-col gap-0.5 rounded-md transition-colors",
          rootTarget && "bg-brand-tint ring-1 ring-brand ring-inset"
        )}
      >
        {renderThreads(sections.root, null)}
      </div>
      {empty ? (
        <Empty
          className="px-3 py-8"
          fill={false}
          icon={MessagesSquare}
          size="compact"
          title={emptyMessage}
        />
      ) : null}
    </>
  );
}

/**
 * One container's threads (a folder, or the root). Its own boundary keeps the
 * container's drop target and per-row wiring stable across renders that leave
 * the container alone, so each memoized thread re-renders only for itself.
 */
function SectionThreads({
  threads,
  folderId,
  layout,
  dnd,
  selectionDrag,
  orderable,
  roots,
  ownerOf,
  currentSessionId,
  collapsedThreads,
  subagentThreads,
  onThreadBranchChange,
  onSelectSession,
  selection,
  sessionActions,
  testIdPrefix,
}: Pick<
  SessionListSectionsProps,
  | "layout"
  | "dnd"
  | "selectionDrag"
  | "orderable"
  | "ownerOf"
  | "currentSessionId"
  | "collapsedThreads"
  | "subagentThreads"
  | "onThreadBranchChange"
  | "onSelectSession"
  | "selection"
  | "sessionActions"
  | "testIdPrefix"
> & {
  threads: readonly SessionThreadModel[];
  folderId: string | null;
  roots: ReadonlyMap<string, SessionPayload>;
}) {
  const container: SessionDropContainer = {
    folderId,
    ids: threads.flatMap(thread =>
      sessionHasOwnPlacement(thread.session) ? [thread.session.id] : []
    ),
  };
  const rootIds = new Set(threads.map(thread => thread.session.id));
  // Only a thread's root row carries folder moves, order verbs, and its own drag source.
  const layoutFor = (session: SessionPayload): SessionThreadRootLayout => {
    const id = session.id;
    if (!rootIds.has(id)) {
      return {
        drag: dnd?.sessionRow(
          id,
          container,
          undefined,
          sessionRowNestTarget(roots, session, sessionActions.onAdopt),
          true
        ),
        dropEdge: rowDropEdge(dnd?.indicator ?? null, id),
      };
    }
    const ownPlacement = sessionHasOwnPlacement(session);
    const reorder = (delta: -1 | 1) => {
      const next = shiftId(container.ids, id, delta);
      return next ? () => layout.reorderSessions(container.folderId, next) : undefined;
    };
    return {
      layoutMenu: ownPlacement
        ? {
            folders: layout.folders,
            folderId: container.folderId,
            onMove: folderId => layout.moveSession(id, folderId),
            ...(orderable ? { onMoveUp: reorder(-1), onMoveDown: reorder(1) } : {}),
          }
        : undefined,
      drag: dnd?.sessionRow(
        id,
        container,
        selectionDrag,
        sessionRowNestTarget(roots, session, sessionActions.onAdopt),
        !ownPlacement
      ),
      dropEdge: rowDropEdge(dnd?.indicator ?? null, id),
      dragging: rowDragged(dnd?.dragging ?? null, id),
    };
  };
  return threads.map(thread => (
    <SessionListThread
      key={thread.session.id}
      session={thread.session}
      owner={ownerOf?.(thread.session)}
      ownerOf={ownerOf}
      childSessions={thread.childSessions}
      currentSessionId={currentSessionId}
      collapsed={collapsedThreads.has(thread.session.id)}
      subagentThreads={subagentThreads}
      onThreadBranchChange={onThreadBranchChange}
      onSelectSession={onSelectSession}
      selection={selection}
      sessionActions={sessionActions}
      testIdPrefix={testIdPrefix}
      layoutFor={layoutFor}
    />
  ));
}
