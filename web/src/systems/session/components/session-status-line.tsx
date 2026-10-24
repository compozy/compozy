import type * as React from "react";

import { cn } from "@compozy/ui";

import { sessionBadgeOf, sessionBadgeSignal } from "../lib/session-badge";
import { sessionBadgeWordClass } from "../lib/session-badge-classes";
import type { SessionOriginView } from "../lib/session-origin";
import { SessionBadgeGlyph } from "./session-badge-mark";
import { SessionOriginPill } from "./session-origin-pill";
import type { SessionPayload } from "../types";

export interface SessionStatusLineProps extends Omit<React.ComponentProps<"span">, "children"> {
  session: SessionPayload;
  /** The document-head variant renders state as the leading mark instead. */
  showState?: boolean;
  /** A continued or forked session's origin, rendered as a pill after the state. */
  origin?: SessionOriginView | null;
  onOpenOriginSource?: (sessionId: string) => void;
}

/**
 * Session lineage and lifecycle for window chrome: the state word (document
 * windows move it to the leading mark, `showState={false}`, keeping it for
 * assistive tech), the Continue/Fork origin pill and the archived word. Agent,
 * provider and time live in the session list; the topbar status zone spends its
 * width on the PR chip instead (`_uiux.md` S1).
 */
export function SessionStatusLine({
  className,
  session,
  showState = true,
  origin = null,
  onOpenOriginSource,
  ...props
}: SessionStatusLineProps) {
  const badge = sessionBadgeOf(session);
  const signal = sessionBadgeSignal(badge);
  const archived = session.archived_at !== null;

  return (
    <span
      data-testid="session-status-meta"
      className={cn(
        "flex h-(--size-avatar-sm) min-w-0 flex-wrap content-start items-center gap-x-2.5 overflow-hidden",
        className
      )}
      {...props}
    >
      {showState ? (
        <span className="inline-flex shrink-0 items-center gap-1.5">
          <SessionBadgeGlyph badge={badge} data-testid="agent-status-dot" />
          <span
            data-testid="session-status-badge"
            data-badge={signal.label}
            className={cn("text-eyebrow", sessionBadgeWordClass(badge))}
          >
            {signal.displayLabel}
          </span>
        </span>
      ) : (
        <span className="sr-only">Session status: {signal.displayLabel}</span>
      )}
      {origin ? <SessionOriginPill onOpenSource={onOpenOriginSource} origin={origin} /> : null}
      {archived ? (
        <span data-testid="session-status-archived" className="text-eyebrow text-subtle">
          Archived
        </span>
      ) : null}
    </span>
  );
}
