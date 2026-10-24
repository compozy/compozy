import { SESSION_LIST_BAR_CLASS } from "../../lib/session-list-bar-class";
import type { SessionListGroupBy, SessionListSort } from "../../lib/session-list-preferences";
import { SessionGroupByMenu, SessionSortMenu } from "../session-order-menus";
import { SessionListCreateMenu } from "./session-list-create-menu";
import { SessionArchivedToggle } from "./session-toolbar-toggles";

export interface SessionListToolbarProps {
  archived: boolean;
  sort: SessionListSort;
  /** The grouping that renders (an inapplicable stored value renders as `none`). */
  groupBy: SessionListGroupBy;
  /** Groupings available now; with only `none` the Group by control is hidden. */
  groupByOptions: readonly SessionListGroupBy[];
  onArchivedChange: (archived: boolean) => void;
  onSortChange: (sort: SessionListSort) => void;
  onGroupByChange: (groupBy: SessionListGroupBy) => void;
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
 * Order and grouping are two explicit controls. They persist per window and are
 * shared with that window's Kanban; the archive is a way of looking right now,
 * so it resets when the list is reopened.
 *
 * Creation order keeps rows still while agents work; activity and attention
 * remain explicit choices.
 */
export function SessionListToolbar({
  archived,
  sort,
  groupBy,
  groupByOptions,
  onArchivedChange,
  onSortChange,
  onGroupByChange,
  onNewSession,
  onNewFolder,
  newFolderUnavailable,
  testIdPrefix,
}: SessionListToolbarProps) {
  return (
    <div className={SESSION_LIST_BAR_CLASS}>
      <SessionArchivedToggle
        archived={archived}
        onArchivedChange={onArchivedChange}
        testId={`${testIdPrefix}-archived`}
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
        testId={`${testIdPrefix}-group-by`}
        value={groupBy}
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
