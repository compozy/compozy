import { StatusDot, Time } from "@compozy/ui";

import { terminalWaitingLead } from "../lib/permission-receipt-copy";
import type { PermissionRequest } from "../types";

export interface PermissionWaitingLineProps {
  permission: PermissionRequest;
  /** ISO timestamp from the permission event, when the daemon published one. */
  askedAt?: string;
}

/**
 * Honest still-waiting line in the transcript while the composer dock holds
 * the decision. Elapsed time comes from `<Time>` — never `Date.now()` in render.
 */
export function PermissionWaitingLine({ permission, askedAt }: PermissionWaitingLineProps) {
  const lead = terminalWaitingLead(permission);
  if (!lead) return null;
  return (
    <p
      className="flex min-h-transcript-row min-w-0 items-center gap-transcript-inline-gap px-1 py-0.5 text-transcript-body text-subtle"
      data-testid="permission-waiting-line"
      role="status"
    >
      <StatusDot
        className="motion-safe:animate-pulse motion-reduce:animate-none"
        size="sm"
        tone="warning"
      />
      <span className="min-w-0 truncate">
        Waiting for your approval to {lead.verb}
        {lead.command ? (
          <>
            {" "}
            <code className="font-mono text-muted">{lead.command}</code>
          </>
        ) : null}
        {askedAt ? (
          <>
            {" "}
            — asked <Time className="text-subtle" iso={askedAt} />
          </>
        ) : null}
      </span>
    </p>
  );
}
