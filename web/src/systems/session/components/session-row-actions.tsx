import { Archive, RotateCcw, Square, Trash2 } from "lucide-react";

import {
  Button,
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
  Spinner,
  TopbarOverflowIcon,
} from "@compozy/ui";

import type { SessionLifecycleActionHandlers } from "../hooks/use-session-lifecycle-actions";
import { getSessionDisplayTitle } from "../lib/session-display-title";
import type { SessionPayload } from "../types";

function isStopEligible(session: SessionPayload): boolean {
  return session.state === "active" || session.state === "starting";
}

export interface SessionRowActionsProps {
  session: SessionPayload;
  actions: SessionLifecycleActionHandlers;
}

/** State-gated lifecycle menu shared by every session list row. */
export function SessionRowActions({ session, actions }: SessionRowActionsProps) {
  const pending = actions.pendingSessionId === session.id ? actions.pendingAction : null;
  const isArchived = session.archived_at !== null;
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
      <DropdownMenuContent align="end">
        {!isArchived && isStopEligible(session) ? (
          <DropdownMenuItem
            data-testid={`session-row-stop-${session.id}`}
            disabled={disabled}
            onClick={() => actions.onStop(session)}
          >
            <Square aria-hidden="true" className="size-3" />
            Stop session
          </DropdownMenuItem>
        ) : null}
        {isArchived ? (
          <DropdownMenuItem
            data-testid={`session-row-unarchive-${session.id}`}
            disabled={disabled}
            onClick={() => actions.onUnarchive(session)}
          >
            <RotateCcw aria-hidden="true" className="size-3" />
            Unarchive session
          </DropdownMenuItem>
        ) : session.state === "stopped" ? (
          <DropdownMenuItem
            data-testid={`session-row-archive-${session.id}`}
            disabled={disabled}
            onClick={() => actions.onArchive(session)}
          >
            <Archive aria-hidden="true" className="size-3" />
            Archive session
          </DropdownMenuItem>
        ) : null}
        <DropdownMenuItem
          data-testid={`session-row-delete-${session.id}`}
          disabled={disabled}
          onClick={() => actions.onDelete(session)}
          variant="destructive"
        >
          <Trash2 aria-hidden="true" className="size-3" />
          Delete session
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
