import { Pin, PinOff } from "@compozy/ui/icons";

import { Button, Icon, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";

import type { SessionLifecycleActionHandlers } from "../../hooks/use-session-lifecycle-actions";
import { getSessionDisplayTitle } from "../../lib/session-display-title";
import { isSubagentSession } from "../../lib/session-hierarchy";
import { sessionIsInLater } from "../../lib/session-later";
import { SESSION_MENU_ACTIONS } from "../../lib/session-menu-actions";
import type { SessionPayload } from "../../types";

/**
 * The row's quiet Pin to top / Unpin, beside its `…` in the end slot, so it reveals on hover and
 * focus the same way. Absent where the host has no Pinned section and on rows the menu offers
 * no pin verb for (a subagent; an archived or parked session not pinned).
 */
export function SessionListPinButton({
  session,
  actions,
}: {
  session: SessionPayload;
  actions: Pick<SessionLifecycleActionHandlers, "onPin" | "onUnpin" | "pendingAction">;
}) {
  const pinned = Boolean(session.pinned_at);
  const verb = pinned ? actions.onUnpin : actions.onPin;
  if (!verb || isSubagentSession(session)) return null;
  if (!pinned && (session.archived_at !== null || sessionIsInLater(session))) return null;
  const label = SESSION_MENU_ACTIONS[pinned ? "unpin" : "pin"].label;
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            aria-label={`${label}: ${getSessionDisplayTitle(session)}`}
            aria-pressed={pinned}
            data-testid={`session-row-pin-${session.id}`}
            disabled={actions.pendingAction !== null}
            onClick={() => verb(session)}
          />
        }
      >
        <Icon as={pinned ? PinOff : Pin} size="sm" />
      </TooltipTrigger>
      <TooltipContent>{label}</TooltipContent>
    </Tooltip>
  );
}
