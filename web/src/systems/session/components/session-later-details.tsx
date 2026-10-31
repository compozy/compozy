import { CalendarClock, MessageSquare } from "@compozy/ui/icons";

import { formatAbsoluteTime, Pill, Time } from "@compozy/ui";

import { cn } from "@/lib/utils";

import { formatLaterReturn, laterPendingLabel, sessionLaterDetails } from "../lib/session-later";
import type { SessionPayload } from "../types";

/**
 * The Later facts of a parked session for a list row or a board card: why it
 * waits, when it comes back, when it was parked, and the agent messages queued
 * for it meanwhile. Nothing renders for a session in the normal flow. The words
 * repeat the owner's accessible name (`sessionLaterSpoken`), so the line itself
 * stays out of the a11y tree.
 */
export function SessionLaterDetails({
  session,
  className,
}: {
  session: Pick<SessionPayload, "id" | "later">;
  className?: string;
}) {
  const details = sessionLaterDetails(session);
  if (details === null) return null;
  return (
    <span
      aria-hidden="true"
      className={cn(
        "flex min-w-0 flex-col gap-0.5 text-eyebrow whitespace-nowrap text-subtle",
        className
      )}
      data-testid={`session-later-details-${session.id}`}
    >
      {details.reason ? (
        <span
          className="min-w-0 truncate text-muted-foreground"
          data-testid={`session-later-reason-${session.id}`}
          title={details.reasonLines.join("\n")}
        >
          {details.reason}
        </span>
      ) : null}
      <span className="flex min-w-0 items-center gap-1.25 overflow-hidden">
        <CalendarClock aria-hidden="true" className="size-3 shrink-0" />
        {details.returnAt ? (
          <span
            className="shrink-0"
            data-testid={`session-later-return-${session.id}`}
            title={formatAbsoluteTime(details.returnAt)}
          >
            Back {formatLaterReturn(details.returnAt)}
          </span>
        ) : null}
        <span className="min-w-0 truncate" data-testid={`session-later-parked-${session.id}`}>
          {details.returnAt ? <span className="text-faint">· </span> : null}
          Parked <Time iso={details.parkedAt} mode="relative" />
        </span>
        {details.pendingMessages > 0 ? (
          <Pill
            className="ml-auto shrink-0"
            data-testid={`session-later-pending-${session.id}`}
            form="plain"
            title={laterPendingLabel(details.pendingMessages)}
            tone="brand"
          >
            <MessageSquare aria-hidden="true" className="size-3" />
            {details.pendingMessages}
          </Pill>
        ) : null}
      </span>
    </span>
  );
}
