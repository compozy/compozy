import { ArrowDownUp, Plus } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
  Icon,
} from "@compozy/ui";

import { SESSION_LIST_SORTS, type SessionListSort } from "../../lib/session-list-preferences";
import { SessionArchivedToggle, SessionScopeToggle } from "./session-toolbar-toggles";

const SORT_LABELS: Record<SessionListSort, string> = {
  last_activity: "Last activity",
  attention: "Attention first",
};

export interface SessionListToolbarProps {
  allWorkspaces: boolean;
  archived: boolean;
  sort: SessionListSort;
  disabled?: boolean;
  onAllWorkspacesChange: (allWorkspaces: boolean) => void;
  onArchivedChange: (archived: boolean) => void;
  onSortChange: (sort: SessionListSort) => void;
  onNewSession: () => void;
  testIdPrefix: string;
}

/**
 * Breadth, contents, order, and the create action for a session list.
 *
 * Breadth and order persist per operator through the daemon, so the choice made
 * here is the same choice the CLI reports; the archive is a way of looking right
 * now, so it resets when the list is reopened.
 *
 * Attention-first is an option, not a takeover: last activity stays the default
 * because a list that reorders itself around urgency is harder to navigate when
 * nothing is urgent.
 */
export function SessionListToolbar({
  allWorkspaces,
  archived,
  sort,
  disabled = false,
  onAllWorkspacesChange,
  onArchivedChange,
  onSortChange,
  onNewSession,
  testIdPrefix,
}: SessionListToolbarProps) {
  return (
    <div className="flex items-center gap-1 px-3 py-1.5">
      <SessionScopeToggle
        allWorkspaces={allWorkspaces}
        busy={disabled}
        onAllWorkspacesChange={onAllWorkspacesChange}
        testId={`${testIdPrefix}-scope`}
      />
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
              variant="ghost"
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
        </DropdownMenuContent>
      </DropdownMenu>
      <Button
        type="button"
        size="icon-sm"
        className="shrink-0"
        aria-label="New session"
        data-testid={`${testIdPrefix}-new-session`}
        onClick={onNewSession}
      >
        <Icon as={Plus} size="sm" />
      </Button>
    </div>
  );
}
