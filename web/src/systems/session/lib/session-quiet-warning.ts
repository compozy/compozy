import type { SessionPayload } from "../types";

/**
 * The daemon's quiet episode, read as clock instants. Supervision emits the
 * warning once per episode (US-014.AC-1) and clears it the moment any work
 * signal returns (US-014.AC-3); this view model never outlives the payload.
 */
export interface SessionQuietWarning {
  /** Instant the last work signal expired: the elapsed clock counts from here. */
  quietSinceMs: number;
  /** Instant the daemon issued the single warning. */
  warnedAtMs: number;
  /** Instant the inactivity stop lands; `null` when automatic stop is off (`stop_grace = "0"`). */
  stopAtMs: number | null;
}

/** The two durations the warning states as facts, fixed at warn time. */
export interface SessionQuietWarningFacts {
  /** How long the session had been quiet when the warning fired ("30 minutes"). */
  quietFor: string;
  /** The grace the daemon granted before stopping ("10 minutes"); `null` when automatic stop is off. */
  stopsIn: string | null;
}

/** The live read of the same episode at `nowMs`, for the status row. */
export interface SessionQuietWarningClock {
  /** Time since the last work signal ("31m"). */
  quietFor: string;
  /** Time left before the inactivity stop ("9m"); `null` when automatic stop is off or the deadline passed. */
  stopsIn: string | null;
  /** The deadline passed and the daemon has not yet reported the stop. */
  stopDue: boolean;
}

function parseInstant(value: string | null | undefined): number | null {
  if (!value) return null;
  const parsed = Date.parse(value);
  return Number.isFinite(parsed) ? parsed : null;
}

/**
 * The quiet warning the open session must surface. Only the daemon's
 * `supervision.quiet_warning` makes it visible: no client timer, no guess from
 * activity. A stopped session carries no episode worth warning about, even if
 * a stale payload still holds one.
 */
export function sessionQuietWarning(
  session: Pick<SessionPayload, "state" | "supervision">
): SessionQuietWarning | null {
  const warning = session.supervision?.quiet_warning;
  if (!warning || session.state === "stopped") {
    return null;
  }
  const quietSinceMs = parseInstant(warning.quiet_since);
  const warnedAtMs = parseInstant(warning.warned_at);
  if (quietSinceMs === null || warnedAtMs === null) {
    return null;
  }
  return { quietSinceMs, warnedAtMs, stopAtMs: parseInstant(warning.stop_at) };
}

/**
 * Minute-granular status vocabulary ("31m", "1h 5m"); seconds only under a
 * minute so short configured thresholds still read truthfully.
 */
export function formatQuietDuration(ms: number): string {
  const totalSeconds = Math.max(0, Math.floor(ms / 1_000));
  if (totalSeconds < 60) {
    return `${totalSeconds}s`;
  }
  const hours = Math.floor(totalSeconds / 3_600);
  const minutes = Math.floor((totalSeconds % 3_600) / 60);
  if (hours > 0) {
    return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`;
  }
  return `${minutes}m`;
}

function pluralize(value: number, unit: string): string {
  return `${value} ${unit}${value === 1 ? "" : "s"}`;
}

/** Spelled-out duration for the notice sentence ("30 minutes", "1 hour 10 minutes", "45 seconds"). */
export function formatQuietDurationWords(ms: number): string {
  const totalSeconds = Math.max(0, Math.floor(ms / 1_000));
  if (totalSeconds < 60) {
    return pluralize(totalSeconds, "second");
  }
  const hours = Math.floor(totalSeconds / 3_600);
  const minutes = Math.floor((totalSeconds % 3_600) / 60);
  if (hours > 0) {
    return minutes > 0
      ? `${pluralize(hours, "hour")} ${pluralize(minutes, "minute")}`
      : pluralize(hours, "hour");
  }
  return pluralize(minutes, "minute");
}

/** What the daemon stated when it warned: quiet for `quiet_after`, stop after `stop_grace`. */
export function sessionQuietWarningFacts(warning: SessionQuietWarning): SessionQuietWarningFacts {
  return {
    quietFor: formatQuietDurationWords(warning.warnedAtMs - warning.quietSinceMs),
    stopsIn:
      warning.stopAtMs === null
        ? null
        : formatQuietDurationWords(warning.stopAtMs - warning.warnedAtMs),
  };
}

/** The episode as it stands now: elapsed since the last signal, time left before the stop. */
export function sessionQuietWarningClock(
  warning: SessionQuietWarning,
  nowMs: number
): SessionQuietWarningClock {
  const quietFor = formatQuietDuration(nowMs - warning.quietSinceMs);
  if (warning.stopAtMs === null) {
    return { quietFor, stopsIn: null, stopDue: false };
  }
  const remainingMs = warning.stopAtMs - nowMs;
  if (remainingMs <= 0) {
    return { quietFor, stopsIn: null, stopDue: true };
  }
  return { quietFor, stopsIn: formatQuietDuration(remainingMs), stopDue: false };
}
