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
import { useSessionRowMenuItems } from "../hooks/use-session-row-menu-items";

export interface SessionRowActionsProps {
  session: SessionPayload;
  actions: SessionLifecycleActionHandlers;
}

/** State-gated lifecycle menu shared by every session list row. */
export function SessionRowActions({ session, actions }: SessionRowActionsProps) {
  const pending = actions.pendingSessionId === session.id ? actions.pendingAction : null;
  const disabled = actions.pendingAction !== null;
  const title = getSessionDisplayTitle(session);
  const items = useSessionRowMenuItems(session, actions);

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
        {items.map(item => (
          <DropdownMenuItem
            key={item.id}
            data-testid={`session-row-${item.id}-${session.id}`}
            disabled={disabled}
            onClick={item.onSelect}
            variant={item.destructive ? "destructive" : undefined}
          >
            <item.icon aria-hidden="true" className="size-3" />
            {item.label}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
