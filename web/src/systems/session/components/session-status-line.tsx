import type * as React from "react";

import { cn } from "@compozy/ui";

import { sessionBadgeSignal, type SessionBadgeToken } from "../lib/session-badge";
import { sessionBadgeWordClass } from "../lib/session-badge-classes";
import { SessionBadgeGlyph } from "./session-badge-mark";
import type { SessionPayload, SessionState } from "../types";

const STATE_BADGE_FALLBACK: Record<SessionState, SessionBadgeToken> = {
  active: "idle",
  starting: "running",
  stopping: "running",
  stopped: "stopped",
};

export interface SessionStatusLineProps extends Omit<React.ComponentProps<"span">, "children"> {
  session: SessionPayload;
  /** The document-head variant renders state as the leading mark instead. */
  showState?: boolean;
}

/**
 * Daemon badge plus agent/runtime identity for session chrome. Document
 * windows can move the state signal to the leading mark while retaining meta.
 */
export function SessionStatusLine({
  className,
  session,
  showState = true,
  ...props
}: SessionStatusLineProps) {
  const badge = session.badge || STATE_BADGE_FALLBACK[session.state];
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
            className={cn("font-mono text-eyebrow", sessionBadgeWordClass(badge))}
          >
            {signal.label}
          </span>
        </>
      ) : (
        <span className="sr-only">Session badge: {signal.label}</span>
      )}
      {agentLabel ? (
        <>
          {showState ? (
            <span aria-hidden="true" className="text-subtle">
              ·
            </span>
          ) : null}
          <span data-testid="session-status-agent" className="truncate text-eyebrow text-muted">
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
          <span data-testid="session-status-provider" className="font-mono text-eyebrow text-faint">
            {providerLabel}
          </span>
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
