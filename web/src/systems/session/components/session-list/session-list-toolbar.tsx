import type { ReactNode } from "react";

import { SESSION_LIST_BAR_CLASS } from "../../lib/session-list-bar-class";
import type { SessionListGroupBy, SessionListSort } from "../../lib/session-list-preferences";
import { SessionGroupByMenu, SessionSortMenu } from "../session-order-menus";
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
  /** Groupings available now; with only `none` the Group by control is hidden. */
  groupByOptions: readonly SessionListGroupBy[];
  onShelfChange: (shelf: SessionListShelf) => void;
  onSortChange: (sort: SessionListSort) => void;
  onGroupByChange: (groupBy: SessionListGroupBy) => void;
  /** Project grouping also lists projects with no sessions in view (#1085). */
  showEmptyProjects: boolean;
  onShowEmptyProjectsChange: (show: boolean) => void;
  onNewSession: () => void;
  /** Present only where folders apply; turns (+) into a New session / New folder menu. */
  onNewFolder?: () => void;
  /** Why this list cannot hold folders; New folder shows disabled with it. */
  newFolderUnavailable?: string | null;
  /** Replaces the plain `+`: the live list's menu with Add stays-on agent. */
  createMenu?: ReactNode;
  testIdPrefix: string;
}

/**
 * Contents, order, grouping, and the create action for a session list. Breadth
 * is the topbar globe's (ADR-004); this toolbar has no breadth control.
 *
 * Order and grouping are two explicit controls. They persist per window and are
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
  onNewSession,
  onNewFolder,
  newFolderUnavailable,
  createMenu,
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
      <SessionSortMenu
        className="ml-auto"
        onValueChange={onSortChange}
        testId={`${testIdPrefix}-sort`}
        value={sort}
      />
      <SessionGroupByMenu
        onValueChange={onGroupByChange}
        options={groupByOptions}
        showEmptyProjects={showEmptyProjects}
        onShowEmptyProjectsChange={onShowEmptyProjectsChange}
        testId={`${testIdPrefix}-group-by`}
        value={groupBy}
      />
      {createMenu ?? (
        <SessionListCreateMenu
          onNewSession={onNewSession}
          onNewFolder={onNewFolder}
          newFolderUnavailable={newFolderUnavailable}
          testId={testIdPrefix}
        />
      )}
    </div>
  );
}
