import type * as React from "react";

import { cn, OwnerAvatar, Time } from "@compozy/ui";

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
  /** A continued or forked session's origin, rendered as a pill after the agent chip. */
  origin?: SessionOriginView | null;
  onOpenOriginSource?: (sessionId: string) => void;
}

/**
 * Session identity for window chrome (prototype `.head` trail): the agent chip
 * — its avatar monogram, name and, once a runtime is bound, the provider it
 * runs on — then the origin pill, the archived word, and the muted time of the
 * last change. The model is not repeated here; the composer's runtime selector
 * owns it. Document windows move the state signal to the leading mark
 * (`showState={false}`), keeping the state word for assistive tech.
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
  const agentLabel = session.agent_name.trim();
  const providerLabel = session.runtime.effective?.provider.trim();

  return (
    <span
      data-testid="session-status-meta"
      className={cn("flex min-w-0 items-center gap-2.5", className)}
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
      {agentLabel ? (
        <span
          data-testid="session-status-agent-chip"
          className="inline-flex min-w-0 items-center gap-1.5 text-eyebrow"
        >
          <OwnerAvatar name={agentLabel} ownerId={agentLabel} ownerKind="agent" size="sm" />
          <span data-testid="session-status-agent" className="truncate font-medium text-fg-2">
            {agentLabel}
          </span>
          {providerLabel ? (
            <>
              <span aria-hidden="true" className="text-subtle">
                ·
              </span>
              <span
                data-testid="session-status-provider"
                // Yields width before the origin pill, which keeps its verb.
                className="min-w-0 truncate font-mono text-subtle"
              >
                {providerLabel}
              </span>
            </>
          ) : null}
        </span>
      ) : null}
      {origin ? <SessionOriginPill onOpenSource={onOpenOriginSource} origin={origin} /> : null}
      {session.archived_at !== null ? (
        <span data-testid="session-status-archived" className="text-eyebrow text-subtle">
          Archived
        </span>
      ) : null}
      <Time
        data-testid="session-status-time"
        iso={session.updated_at}
        className="shrink-0 text-eyebrow text-subtle"
      />
    </span>
  );
}
