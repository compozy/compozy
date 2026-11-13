import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuTrigger,
  Spinner,
  TopbarOverflowIcon,
} from "@compozy/ui";

import { getSessionDisplayTitle } from "../lib/session-display-title";
import { SessionRowMenuItems, type SessionRowMenuProps } from "./session-row-menu-items";

export type SessionRowActionsProps = SessionRowMenuProps;

/** The `…` menu of a session row or card: the same tree as its right-click menu. */
export function SessionRowActions(props: SessionRowActionsProps) {
  const { session, actions } = props;
  const pending = actions.pendingSessionIds.includes(session.id) ? actions.pendingAction : null;
  const disabled = actions.pendingAction !== null;
  const title = getSessionDisplayTitle(session);

  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        aria-label={`More actions for ${title}`}
        data-testid={`session-row-actions-${session.id}`}
        render={<Button type="button" variant="ghost" size="icon-sm" disabled={disabled} />}
      >
        {pending ? <Spinner className="size-3" /> : <TopbarOverflowIcon aria-hidden="true" />}
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="min-w-56">
        <SessionRowMenuItems {...props} host="dropdown" />
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
