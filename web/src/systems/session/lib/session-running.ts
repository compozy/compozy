import type { SessionPayload } from "../types";

const RUNNING_BADGES = new Set(["running"]);
const NON_RUNNING_BADGES = new Set(["hung", "stopped", "unhealthy"]);
const NON_RUNNING_STATES = new Set(["starting", "stopped"]);
const PUBLIC_SESSION_TYPES = new Set<NonNullable<SessionPayload["type"]>>([
  "user",
  "system",
  "coordinator",
  "spawned",
]);

function nonEmpty(value: unknown): boolean {
  return typeof value === "string" && value.trim().length > 0;
}

function sessionHealthState(session: SessionPayload): string {
  const health = session.health as { state?: unknown } | null | undefined;
  return typeof health?.state === "string" ? health.state : "";
}

/**
 * A prompt is in flight, whatever the display badge says. A stalled prompt keeps its
 * turn while the badge turns `hung` or `unhealthy`, and the daemon still refuses new
 * work for it (`session_busy`), so work admission must not read the badge as "idle".
 */
export function hasActivePrompt(session: SessionPayload): boolean {
  if (NON_RUNNING_STATES.has(session.state)) {
    return false;
  }

  if (nonEmpty(session.activity?.turn_id)) {
    return true;
  }

  if (RUNNING_BADGES.has(session.badge)) {
    return true;
  }

  return sessionHealthState(session) === "prompting";
}

/** Display predicate: a stalled or stopped badge reads as not running even with a prompt open. */
export function isSessionRunning(session: SessionPayload): boolean {
  if (NON_RUNNING_BADGES.has(session.badge)) {
    return false;
  }

  return hasActivePrompt(session);
}

/** The daemon's subagent summary counts a live subagent under the session. */
function hasLiveSubagents(session: SessionPayload): boolean {
  return (session.subagent_summary?.live ?? 0) > 0;
}

/**
 * The session's own work is running: its turn or a live subagent (#1926). The sidebar's
 * leading column reads it for session rows and for the folders that hold them.
 */
export function isSessionWorkRunning(session: SessionPayload): boolean {
  return isSessionRunning(session) || hasLiveSubagents(session);
}

/** Work runs below the session: a live subagent of it or of a descendant, or a running descendant. */
export function hasRunningDescendant(
  session: SessionPayload,
  descendants: readonly SessionPayload[]
): boolean {
  return hasLiveSubagents(session) || descendants.some(isSessionWorkRunning);
}

export function hasRunningSession(sessions: SessionPayload[] | undefined): boolean {
  return sessions?.some(isSessionRunning) ?? false;
}

export function runningAgentNames(sessions: SessionPayload[] | undefined): Set<string> {
  const names = new Set<string>();
  for (const session of sessions ?? []) {
    if (isSessionRunning(session)) {
      names.add(session.agent_name);
    }
  }
  return names;
}

export function idleAttachableAgentNames(sessions: SessionPayload[] | undefined): Set<string> {
  const names = new Set<string>();
  for (const session of sessions ?? []) {
    if (
      session.archived_at === null &&
      session.state === "active" &&
      session.attachable &&
      !isSessionRunning(session)
    ) {
      names.add(session.agent_name);
    }
  }
  return names;
}

export function isUserControllableSession(session: SessionPayload): boolean {
  return (session.type ?? "user") === "user";
}

function isPublicSession(session: SessionPayload): boolean {
  return session.type !== undefined && PUBLIC_SESSION_TYPES.has(session.type);
}

/**
 * A process-exited session has durable history but no ACP peer to resume.
 * Health alone cannot identify this state: intentionally stopped sessions are
 * also reported as dead until they are resumed.
 */
export function hasUnrecoverableRuntime(session: SessionPayload): boolean {
  const health = session.health;
  return (
    session.state === "stopped" &&
    session.failure?.kind === "process_exit" &&
    health?.health === "dead" &&
    !health.attachable &&
    !health.eligible_for_wake
  );
}

export function canPromptSession(session: SessionPayload): boolean {
  if (hasUnrecoverableRuntime(session)) {
    return false;
  }
  return (
    isPublicSession(session) &&
    session.archived_at === null &&
    (session.state === "active" || session.state === "stopped")
  );
}
