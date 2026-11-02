import { SESSION_LIST_BAR_CLASS } from "../../lib/session-list-bar-class";
import type { SessionListGroupBy, SessionListSort } from "../../lib/session-list-preferences";
import { SessionViewOptionsMenu } from "../session-view-options-menu";
import { SessionListCreateMenu } from "./session-list-create-menu";
import type { SessionListShelf } from "../../lib/session-list-shelf";
import { SessionArchivedToggle, SessionLaterToggle } from "./session-toolbar-toggles";

export interface SessionListToolbarProps {
  /** Active, the archive, or Later: the two toggles switch between them. */
  shelf: SessionListShelf;
  /** Sessions parked in Later across the list's scope; `undefined` until counted. */
  laterCount?: number;
  sort: SessionListSort;
  /** The grouping that renders (an inapplicable stored value renders as `none`). */
  groupBy: SessionListGroupBy;
  /** Groupings available now; with only `none` the Group by section is hidden. */
  groupByOptions: readonly SessionListGroupBy[];
  onShelfChange: (shelf: SessionListShelf) => void;
  onSortChange: (sort: SessionListSort) => void;
  onGroupByChange: (groupBy: SessionListGroupBy) => void;
  /** Project grouping also lists projects with no sessions in view (#1085). */
  showEmptyProjects: boolean;
  onShowEmptyProjectsChange: (show: boolean) => void;
  /** Collapse all is on: folders, groups, child session lists and subagent lists. */
  allCollapsed: boolean;
  onAllCollapsedChange: (collapsed: boolean) => void;
  onNewSession: () => void;
  /** Present only where folders apply; turns (+) into a New session / New folder menu. */
  onNewFolder?: () => void;
  /** Why this list cannot hold folders; New folder shows disabled with it. */
  newFolderUnavailable?: string | null;
  testIdPrefix: string;
}

/**
 * Contents, order, grouping, and the create action for a session list. Breadth
 * is the topbar globe's (ADR-004); this toolbar has no breadth control.
 *
 * Order, grouping and the folds live in one View options menu. They persist per window and are
 * shared with that window's Kanban; the archive and Later are ways of looking
 * right now, so they reset when the list is reopened. Turning one on turns the
 * other off; turning it off returns to the active list.
 *
 * Creation order keeps rows still while agents work; activity and attention
 * remain explicit choices.
 */
export function SessionListToolbar({
  shelf,
  laterCount,
  sort,
  groupBy,
  groupByOptions,
  onShelfChange,
  onSortChange,
  onGroupByChange,
  showEmptyProjects,
  onShowEmptyProjectsChange,
  allCollapsed,
  onAllCollapsedChange,
  onNewSession,
  onNewFolder,
  newFolderUnavailable,
  testIdPrefix,
}: SessionListToolbarProps) {
  return (
    <div className={SESSION_LIST_BAR_CLASS}>
      <SessionArchivedToggle
        archived={shelf === "archive"}
        onArchivedChange={on => onShelfChange(on ? "archive" : "active")}
        testId={`${testIdPrefix}-archived`}
      />
      <SessionLaterToggle
        count={laterCount}
        later={shelf === "later"}
        onLaterChange={on => onShelfChange(on ? "later" : "active")}
        testId={`${testIdPrefix}-later`}
      />
      <SessionViewOptionsMenu
        allCollapsed={allCollapsed}
        className="ml-auto"
        groupBy={groupBy}
        groupByOptions={groupByOptions}
        onAllCollapsedChange={onAllCollapsedChange}
        onGroupByChange={onGroupByChange}
        onShowEmptyProjectsChange={onShowEmptyProjectsChange}
        onSortChange={onSortChange}
        showEmptyProjects={showEmptyProjects}
        sort={sort}
        testId={testIdPrefix}
      />
      <SessionListCreateMenu
        onNewSession={onNewSession}
        onNewFolder={onNewFolder}
        newFolderUnavailable={newFolderUnavailable}
        testId={testIdPrefix}
      />
    </div>
  );
}
