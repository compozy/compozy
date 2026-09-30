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
  /** A continued or forked session's origin, rendered as a pill after the provider. */
  origin?: SessionOriginView | null;
  onOpenOriginSource?: (sessionId: string) => void;
}

/**
 * Daemon badge plus agent/runtime identity for session chrome. Document
 * windows can move the state signal to the leading mark while retaining meta.
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
      className={cn("flex min-w-0 items-center gap-2", className)}
      {...props}
    >
      {showState ? (
        <>
          <SessionBadgeGlyph badge={badge} data-testid="agent-status-dot" />
          <span
            data-testid="session-status-badge"
            data-badge={signal.label}
            className={cn("text-eyebrow", sessionBadgeWordClass(badge))}
          >
            {signal.displayLabel}
          </span>
        </>
      ) : (
        <span className="sr-only">Session status: {signal.displayLabel}</span>
      )}
      {agentLabel ? (
        <>
          {showState ? (
            <span aria-hidden="true" className="text-subtle">
              ·
            </span>
          ) : null}
          <span
            data-testid="session-status-agent"
            className="truncate text-eyebrow font-medium text-fg-2"
          >
            {agentLabel}
          </span>
        </>
      ) : null}
      {providerLabel ? (
        <>
          {showState || agentLabel ? (
            <span aria-hidden="true" className="text-subtle">
              ·
            </span>
          ) : null}
          <span
            data-testid="session-status-provider"
            // Yields width before the origin pill, which keeps its verb.
            className="min-w-0 truncate font-mono text-eyebrow text-faint"
          >
            {providerLabel}
          </span>
        </>
      ) : null}
      {origin ? (
        <>
          {showState || agentLabel || providerLabel ? (
            <span aria-hidden="true" className="text-subtle">
              ·
            </span>
          ) : null}
          <SessionOriginPill onOpenSource={onOpenOriginSource} origin={origin} />
        </>
      ) : null}
      {session.archived_at !== null ? (
        <span data-testid="session-status-archived" className="text-eyebrow text-subtle">
          Archived
        </span>
      ) : null}
    </span>
  );
}
