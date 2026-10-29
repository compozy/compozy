import type {
  SessionProcessKind,
  SessionProcessPayload,
  SessionProcessResult,
} from "../adapters/session-processes-api";
import type { SessionPayload } from "../types";

type SessionProcessSummarySource = Pick<SessionPayload, "process_summary">;

/**
 * Processes the session left running: detached background work plus live
 * Terminals. Foreground tool calls are not counted; they end with their turn.
 */
export function sessionRunningProcesses(session: SessionProcessSummarySource): number {
  const summary = session.process_summary;
  if (!summary?.supported) return 0;
  return Math.max(0, summary.background + summary.terminals);
}

/** The listening ports the session's processes hold, in the daemon's order. */
export function sessionProcessPorts(session: SessionProcessSummarySource): readonly number[] {
  return session.process_summary?.supported ? (session.process_summary.ports ?? []) : [];
}

/** Running processes across a selection, for one batch choice. */
export function sessionsRunningProcesses(sessions: readonly SessionProcessSummarySource[]): number {
  return sessions.reduce((total, session) => total + sessionRunningProcesses(session), 0);
}

export function processCountLabel(count: number): string {
  return `${count} ${count === 1 ? "process" : "processes"}`;
}

/** Composer trigger aria: "Processes, {N} running". */
export function processControlLabel(count: number): string {
  return `Processes, ${count} running`;
}

const KIND_LABELS: Record<SessionProcessKind, string> = {
  background: "Background",
  terminal: "Terminal",
  tool: "Tool call",
};

export function processKindLabel(kind: SessionProcessKind): string {
  return KIND_LABELS[kind] ?? kind;
}

/** The command line as one row: newlines folded so a heredoc cannot push the row open. */
export function processCommandLabel(process: Pick<SessionProcessPayload, "command" | "argv0">) {
  const command = process.command.trim() || process.argv0.trim();
  return command.replace(/\s*\n\s*/g, " ↵ ");
}

/**
 * COPY.md: a process whose environment can't be read (a protected system binary)
 * is linked to its session by ancestry, from the first scan that saw it under the
 * live agent. The row says so quietly; the title carries the reason.
 */
export const PROCESS_LINEAGE_LABEL = "linked at launch";
export const PROCESS_LINEAGE_HINT =
  "This process hides its environment, so CompozyOS linked it to the session when it first saw it running under the agent.";

/** Only an explicit `lineage` reads as linked by ancestry; an absent field is the env marker. */
export function processLinkedByLineage(process: Pick<SessionProcessPayload, "attribution">) {
  return process.attribution === "lineage";
}

export function processPortHref(port: number): string {
  return `http://localhost:${port}`;
}

/** Stable identity for a row: a pid alone is reused by the OS. */
export function processIdentity(process: Pick<SessionProcessPayload, "pid" | "started_at">) {
  return `${process.pid}@${process.started_at}`;
}

/** Background and Terminal rows first (they outlive the turn), then tool calls; newest first. */
export function sortSessionProcesses(
  items: readonly SessionProcessPayload[]
): SessionProcessPayload[] {
  const rank = (kind: SessionProcessKind) => (kind === "tool" ? 1 : 0);
  return [...items].sort(
    (left, right) =>
      rank(left.kind) - rank(right.kind) ||
      Date.parse(right.started_at) - Date.parse(left.started_at)
  );
}

/** What the person reads after a stop, from the daemon's per-target results. */
export interface ProcessStopReport {
  tone: "success" | "info" | "error";
  title: string;
  description?: string;
}

const OUTCOME_COPY: Record<Exclude<SessionProcessResult["outcome"], "terminated">, string> = {
  already_exited: "It had already exited.",
  identity_mismatch: "It changed since the list was read. Refresh and try again.",
  unproven: "Stop was sent, but its exit couldn't be confirmed.",
  failed: "Couldn't stop it.",
};

/**
 * One toast for a stop: "2 processes stopped", or the partial outcome with each
 * refused process by command. `already_exited` counts as done: nothing runs.
 */
export function processStopReport(
  results: readonly SessionProcessResult[],
  commandFor: (result: SessionProcessResult) => string
): ProcessStopReport {
  const isDone = (result: SessionProcessResult) =>
    result.outcome === "terminated" || result.outcome === "already_exited";
  const done = results.filter(isDone);
  const refused = results.filter(result => !isDone(result));
  if (results.length === 0) return { tone: "info", title: "Nothing was running." };
  if (refused.length === 0) {
    if (done.every(result => result.outcome === "already_exited")) {
      return {
        tone: "info",
        title: done.length === 1 ? "Process already exited" : "Processes already exited",
      };
    }
    return { tone: "success", title: `${processCountLabel(done.length)} stopped` };
  }
  const description = refused
    .map(result => {
      const reason =
        result.outcome === "failed" && result.error
          ? result.error
          : OUTCOME_COPY[result.outcome as keyof typeof OUTCOME_COPY];
      return `${commandFor(result)}: ${reason}`;
    })
    .join(" · ");
  return {
    tone: "error",
    title:
      done.length > 0
        ? `${processCountLabel(done.length)} stopped, ${refused.length} not stopped`
        : `Couldn't stop ${processCountLabel(refused.length)}`,
    description,
  };
}
