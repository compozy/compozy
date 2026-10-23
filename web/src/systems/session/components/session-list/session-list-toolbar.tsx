import { ArrowDownUp } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
  Icon,
} from "@compozy/ui";

import { SESSION_LIST_BAR_CLASS } from "../../lib/session-list-bar-class";
import {
  SESSION_LIST_SORTS,
  type SessionListGroupBy,
  type SessionListSort,
} from "../../lib/session-list-preferences";
import { SessionListCreateMenu } from "./session-list-create-menu";
import { SessionArchivedToggle } from "./session-toolbar-toggles";

const SORT_LABELS: Record<SessionListSort, string> = {
  created: "Created",
  name: "Name",
  last_activity: "Last activity",
  attention: "Attention first",
  manual: "Manual",
};

const GROUP_BY_LABELS: Record<SessionListGroupBy, string> = {
  none: "None",
  project: "Project",
  profile: "Profile",
};

export interface SessionListToolbarProps {
  archived: boolean;
  sort: SessionListSort;
  /** The grouping that renders (an inapplicable stored value renders as `none`). */
  groupBy: SessionListGroupBy;
  /** Groupings available now; with only `none` the Group by section is hidden. */
  groupByOptions: readonly SessionListGroupBy[];
  disabled?: boolean;
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
 * Order and grouping persist per operator through the daemon, so the choice made
 * here is the same choice the CLI reports; the archive is a way of looking right
 * now, so it resets when the list is reopened.
 *
 * Creation order keeps rows still while agents work; activity and attention
 * remain explicit choices.
 */
export function SessionListToolbar({
  archived,
  sort,
  groupBy,
  groupByOptions,
  disabled = false,
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
      <DropdownMenu>
        <DropdownMenuTrigger
          aria-label="Sort sessions"
          data-testid={`${testIdPrefix}-sort-trigger`}
          render={
            <Button
              type="button"
              variant="quiet"
              size="icon-sm"
              className="ml-auto shrink-0"
              disabled={disabled}
            />
          }
        >
          <Icon as={ArrowDownUp} size="sm" />
        </DropdownMenuTrigger>
        <DropdownMenuContent align="end" className="min-w-44">
          <DropdownMenuRadioGroup
            value={sort}
            onValueChange={value => onSortChange(value as SessionListSort)}
          >
            {SESSION_LIST_SORTS.map(value => (
              <DropdownMenuRadioItem
                key={value}
                value={value}
                data-testid={`${testIdPrefix}-sort-${value}`}
              >
                {SORT_LABELS[value]}
              </DropdownMenuRadioItem>
            ))}
          </DropdownMenuRadioGroup>
          {groupByOptions.length > 1 ? (
            <>
              <DropdownMenuSeparator />
              <DropdownMenuGroup>
                <DropdownMenuLabel>Group by</DropdownMenuLabel>
                <DropdownMenuRadioGroup
                  value={groupBy}
                  onValueChange={value => onGroupByChange(value as SessionListGroupBy)}
                >
                  {groupByOptions.map(value => (
                    <DropdownMenuRadioItem
                      key={value}
                      value={value}
                      data-testid={`${testIdPrefix}-group-by-${value}`}
                    >
                      {GROUP_BY_LABELS[value]}
                    </DropdownMenuRadioItem>
                  ))}
                </DropdownMenuRadioGroup>
              </DropdownMenuGroup>
            </>
          ) : null}
        </DropdownMenuContent>
      </DropdownMenu>
      <SessionListCreateMenu
        onNewSession={onNewSession}
        onNewFolder={onNewFolder}
        newFolderUnavailable={newFolderUnavailable}
        testId={testIdPrefix}
      />
    </div>
  );
}
