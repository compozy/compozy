import type * as React from "react";

import { cn, Pill, type PillTone } from "@agh/ui";

import type { SessionBadge, SessionPayload, SessionState } from "../types";

interface StateSignal {
  tone: PillTone;
  pulse?: boolean;
  label: string;
}

const BADGE_SIGNAL: Record<SessionBadge, StateSignal> = {
  running: { tone: "success", pulse: true, label: "running" },
  idle: { tone: "info", label: "idle" },
  unhealthy: { tone: "warning", label: "unhealthy" },
  hung: { tone: "danger", pulse: true, label: "hung" },
  "waiting-for-auth": { tone: "warning", label: "waiting-for-auth" },
  stopped: { tone: "neutral", label: "stopped" },
  failed: { tone: "danger", label: "failed" },
  unknown: { tone: "neutral", label: "unknown" },
};

const STATE_BADGE_FALLBACK: Record<SessionState, SessionBadge> = {
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
 * Daemon badge plus agent/provider identity for session chrome. Document
 * windows can move the state signal to the leading mark while retaining meta.
 */
export function SessionStatusLine({
  className,
  session,
  showState = true,
  ...props
}: SessionStatusLineProps) {
  const badge = session.badge ?? STATE_BADGE_FALLBACK[session.state] ?? "unknown";
  const signal = BADGE_SIGNAL[badge] ?? BADGE_SIGNAL.unknown;
  const agentLabel = session.agent_name.trim();
  const providerLabel = session.provider?.trim();

  return (
    <span
      data-testid="session-status-meta"
      className={cn("flex min-w-0 items-center gap-2", className)}
      {...props}
    >
      {showState ? (
        <>
          <Pill.Dot
            size="md"
            tone={signal.tone}
            pulse={signal.pulse}
            data-testid="agent-status-dot"
            aria-label={`Session badge: ${signal.label}`}
          />
          <span data-testid="session-status-badge" className="font-mono text-eyebrow text-faint">
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
    </span>
  );
}
