import { ArrowDownUp, FolderPlus, MessageSquarePlus, Plus } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
  Icon,
} from "@compozy/ui";

import { SESSION_LIST_BAR_CLASS } from "../../lib/session-list-bar-class";
import { SESSION_LIST_SORTS, type SessionListSort } from "../../lib/session-list-preferences";
import { SessionArchivedToggle, SessionScopeToggle } from "./session-toolbar-toggles";

const SORT_LABELS: Record<SessionListSort, string> = {
  created: "Created",
  name: "Name",
  last_activity: "Last activity",
  attention: "Attention first",
  manual: "Manual",
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
  /** Present only where folders apply; turns (+) into a New session / New folder menu. */
  onNewFolder?: () => void;
  testIdPrefix: string;
}

/**
 * Breadth, contents, order, and the create action for a session list.
 *
 * Breadth and order persist per operator through the daemon, so the choice made
 * here is the same choice the CLI reports; the archive is a way of looking right
 * now, so it resets when the list is reopened.
 *
 * Creation order keeps rows still while agents work; activity and attention
 * remain explicit choices.
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
  onNewFolder,
  testIdPrefix,
}: SessionListToolbarProps) {
  return (
    <div className={SESSION_LIST_BAR_CLASS}>
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
        </DropdownMenuContent>
      </DropdownMenu>
      <SessionListCreateMenu
        onNewSession={onNewSession}
        onNewFolder={onNewFolder}
        testIdPrefix={testIdPrefix}
      />
    </div>
  );
}

/**
 * The list's create control. Secondary, not the inverted primary: the composer's
 * send owns that (P4). Where folders apply it opens a menu; elsewhere it creates
 * a session directly.
 */
function SessionListCreateMenu({
  onNewSession,
  onNewFolder,
  testIdPrefix,
}: Pick<SessionListToolbarProps, "onNewSession" | "onNewFolder" | "testIdPrefix">) {
  if (!onNewFolder) {
    return (
      <Button
        type="button"
        variant="secondary"
        size="icon-sm"
        className="shrink-0"
        aria-label="New session"
        data-testid={`${testIdPrefix}-new-session`}
        onClick={onNewSession}
      >
        <Icon as={Plus} size="sm" />
      </Button>
    );
  }
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label="New session or folder"
        data-testid={`${testIdPrefix}-new`}
        render={<Button type="button" variant="secondary" size="icon-sm" className="shrink-0" />}
      >
        <Icon as={Plus} size="sm" />
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-44">
        <DropdownMenuItem
          data-testid={`${testIdPrefix}-new-session`}
          onClick={() => onNewSession()}
        >
          <Icon as={MessageSquarePlus} size="sm" />
          New session
        </DropdownMenuItem>
        <DropdownMenuItem data-testid={`${testIdPrefix}-new-folder`} onClick={onNewFolder}>
          <Icon as={FolderPlus} size="sm" />
          New folder
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
