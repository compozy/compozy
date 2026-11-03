import { Button, Spinner, Tooltip, TooltipContent, TooltipTrigger } from "@compozy/ui";
import { CheckCheck } from "@compozy/ui/icons";

import { useMarkSessionSeenAction } from "../hooks/use-mark-session-seen";
import type { SessionPayload } from "../types";

/**
 * The session header's Mark as seen: shown only while the daemon reports a
 * Needs you the operator can clear, so a pending answer is never hidden here.
 */
export function SessionMarkSeenAction({ session }: { session: SessionPayload }) {
  const { pending, onMarkSeen } = useMarkSessionSeenAction();
  if (!session.attention_dismissible) return null;
  const Glyph = pending ? Spinner : CheckCheck;
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            className="focus-visible:shadow-focus-inset"
            onClick={() => onMarkSeen(session)}
            disabled={pending}
            data-testid="session-mark-seen-button"
            aria-label="Mark as seen"
          />
        }
      >
        <Glyph aria-hidden="true" className="size-3.5" />
      </TooltipTrigger>
      <TooltipContent>Mark as seen</TooltipContent>
    </Tooltip>
  );
}
