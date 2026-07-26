import type { PillTone } from "@agh/ui";

import type { LoopRunEventFrame } from "../types";
import { parseLoopActionFailure } from "./action-failure";
import type {
  LoopStoryIcon,
  LoopStoryIssue,
  LoopStoryRow,
  LoopStoryTaskLink,
} from "./loop-run-story-types";

/**
 * The per-kind row builders (redesign spec §5.3): each folds one replayed SSE
 * frame into a plain-language story row plus its verbatim mono `micro` trail.
 * Status transitions phrase the lifecycle beats; `finalizeSuccessRow` collapses
 * grouped branch successes. Shared coercion + humanization helpers live here so
 * the orchestrator and the next-note builder read from one place.
 */

export function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : null;
}

export function str(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

export function num(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

/** `fix_batches` → "fix batches" — the generic humanization of a node id. */
export function humanizeLoopNodeId(nodeId: string): string {
  return nodeId.replace(/[_-]+/g, " ").trim();
}

function sentenceCase(value: string): string {
  return value.charAt(0).toUpperCase() + value.slice(1);
}

function confidenceClause(confidence: number | undefined): string {
  return confidence === undefined ? "" : ` · confidence ${confidence.toFixed(2)}`;
}

export interface MutableStoryRow extends LoopStoryRow {
  /** Consecutive same-node success grouping accumulator (§5.3). */
  group?: { nodeId: string; generation: number; count: number; indexes: number[] };
}

export function runningKey(nodeId: string, itemIndex: number | undefined): string {
  return `${nodeId}:${itemIndex ?? "x"}`;
}

export function branchKey(generation: number, nodeId: string): string {
  return `${generation}:${nodeId}`;
}

export function taskLink(payload: Record<string, unknown>): LoopStoryTaskLink | undefined {
  const taskId = str(payload.task_id);
  const taskRunId = str(payload.task_run_id);
  return taskId && taskRunId ? { taskId, taskRunId } : undefined;
}

export function generationStartedRow(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>,
  fallbackStrategy: string | undefined
): MutableStoryRow {
  const generation = num(payload.generation) ?? 0;
  const strategy = str(payload.reattempt_strategy, fallbackStrategy ?? "");
  const failedOnly = strategy === "failed_only" && generation > 1;
  return {
    key: `f-${frame.seq}`,
    kind: "generation_started",
    seq: num(frame.seq) ?? 0,
    at: str(frame.at),
    tone: "neutral",
    icon: "round",
    title: `Round ${generation} started${failedOnly ? " — redoing the failed group(s)" : ""}`,
    sub: failedOnly
      ? "Failed-only re-attempt: clean groups carry over untouched; only the failed groups run again."
      : undefined,
    micro: `generation_started · gen ${generation}`,
    generation,
  };
}

export function gateVerdictRow(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>
): MutableStoryRow {
  const passed = str(payload.verdict) === "pass";
  const confidence = num(payload.confidence);
  const rawIssues = Array.isArray(payload.blocking_issues) ? payload.blocking_issues : [];
  const issues: LoopStoryIssue[] = [];
  for (const item of rawIssues) {
    const record = asRecord(item);
    const id = str(record?.id);
    if (id !== "") issues.push({ id, note: str(record?.note) || undefined });
  }
  return {
    key: `f-${frame.seq}`,
    kind: "gate_verdict",
    seq: num(frame.seq) ?? 0,
    at: str(frame.at),
    tone: passed ? "success" : "warning",
    icon: passed ? "check-pass" : "check-warn",
    title: passed ? "Check passed — everything handled" : "Check: not clean yet",
    sub: `Verdict ${passed ? "pass" : "revise"}${confidenceClause(confidence)}${passed ? "" : "."}`,
    issues: passed ? undefined : issues,
    micro: `gate_verdict · ${passed ? "pass" : "revise"}`,
    nodeId: str(payload.node_id) || undefined,
    generation: num(payload.generation),
  };
}

export function nodeFailedRow(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>,
  failedOnlyLive: boolean
): MutableStoryRow {
  const nodeId = str(payload.node_id);
  const itemIndex = num(payload.item_index);
  const failure = parseLoopActionFailure(str(payload.output_ref) || undefined);
  const redoClause = failedOnlyLive
    ? " An incomplete group fails as a whole and is queued to be redone."
    : "";
  return {
    key: `f-${frame.seq}`,
    kind: "node_failed",
    seq: num(frame.seq) ?? 0,
    at: str(frame.at),
    tone: "danger",
    icon: "node-failed",
    title: `${sentenceCase(humanizeLoopNodeId(nodeId))} came up short`,
    sub: failure ? `${failure.cause}${redoClause}` : redoClause.trim() || undefined,
    micro: `node_failed · ${nodeId}${itemIndex === undefined ? "" : `[${itemIndex}]`}`,
    nodeId,
    generation: num(payload.generation),
  };
}

export function needsApprovalRow(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>
): MutableStoryRow {
  return {
    key: `f-${frame.seq}`,
    kind: "needs_approval",
    seq: num(frame.seq) ?? 0,
    at: str(frame.at),
    tone: "warning",
    icon: "approval",
    title: "Asked for your approval",
    sub: str(payload.title) || undefined,
    micro: `needs_approval · ${str(payload.gate_id, "approve")}`,
    generation: num(payload.generation),
  };
}

interface Transition {
  tone: PillTone;
  icon: LoopStoryIcon;
  title: string;
  sub?: string;
}

function wakeTransition(cause: string): Transition {
  const title =
    cause === "watch_poll" ? "A scheduled check woke the run" : "A new event woke the run";
  return {
    tone: "info",
    icon: "watching",
    title,
    sub: "The run left watching and started a new round.",
  };
}

function terminalTransition(
  to: string,
  cause: string,
  failureCause: string | undefined
): Transition | null {
  switch (to) {
    case "failed":
      return { tone: "danger", icon: "node-failed", title: "Run failed", sub: failureCause };
    case "done":
      return { tone: "success", icon: "done", title: "Finished as Done" };
    case "no-op":
      return { tone: "neutral", icon: "done", title: "Ran — nothing to do" };
    case "blocked":
      return { tone: "warning", icon: "check-warn", title: "Run blocked", sub: failureCause };
    case "exhausted":
      return {
        tone: "warning",
        icon: "check-warn",
        title:
          cause === "iteration_cap"
            ? "Round cap reached — run exhausted"
            : "A limit was reached — run exhausted",
        sub: failureCause,
      };
    case "stalled":
      return {
        tone: "neutral",
        icon: "check-warn",
        title: "Run stalled — no progress",
        sub:
          cause === "no_progress"
            ? "The same open points kept coming back round after round."
            : failureCause,
      };
    default:
      return null;
  }
}

function statusTransition(
  from: string,
  to: string,
  cause: string,
  failureCause: string | undefined
): Transition | null {
  if (cause === "operator_stop") {
    return { tone: "neutral", icon: "stopped", title: "Stopped by you" };
  }
  const terminal = terminalTransition(to, cause, failureCause);
  if (terminal) return terminal;
  switch (to) {
    case "running":
      if (from === "watching") return wakeTransition(cause);
      if (from === "paused") {
        return {
          tone: "neutral",
          icon: "resumed",
          title: "Run resumed",
          sub: "Continuing exactly where it left off.",
        };
      }
      if (from === "needs-approval") {
        return { tone: "neutral", icon: "resumed", title: "Approved — the run resumed" };
      }
      return { tone: "neutral", icon: "started", title: "Run started" };
    case "watching":
      return from === "running"
        ? {
            tone: "info",
            icon: "watching",
            title: "Went back to watching",
            sub: "Nothing left to do until a new event arrives.",
          }
        : { tone: "info", icon: "watching", title: "Started watching" };
    case "paused":
      return {
        tone: "neutral",
        icon: "paused",
        title: "Run paused",
        sub: "Requested by you; the run parked at a safe point.",
      };
    case "queued":
      return { tone: "neutral", icon: "started", title: "Run queued", sub: "Waiting for a slot." };
    // `needs-approval` renders through its own `needs_approval` frame — a status
    // row here would duplicate the story beat.
    default:
      return null;
  }
}

export function statusChangedRow(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>
): MutableStoryRow | null {
  const from = str(payload.from);
  const to = str(payload.to, str(payload.status));
  const cause = str(payload.cause);
  const failure = asRecord(payload.failure);
  const transition = statusTransition(from, to, cause, str(failure?.cause) || undefined);
  if (!transition) return null;
  return {
    key: `f-${frame.seq}`,
    kind: "status_changed",
    seq: num(frame.seq) ?? 0,
    at: str(frame.at),
    tone: transition.tone,
    icon: transition.icon,
    title: transition.title,
    sub: transition.sub,
    micro: `status_changed · ${from || "?"} → ${to || "?"}`,
  };
}

/** Finalizes grouped success rows: `{label} — {k} of {n} clean` + `[i–j]` micro. */
export function finalizeSuccessRow(row: MutableStoryRow, branchTotal: number): LoopStoryRow {
  const { group, ...publicRow } = row;
  if (!group) return publicRow;
  const label = humanizeLoopNodeId(group.nodeId);
  const indexes = [...group.indexes].sort((a, b) => a - b);
  const hasBranches = branchTotal > 1;
  const micro =
    indexes.length === 0
      ? `node_succeeded · ${group.nodeId}`
      : indexes.length === 1
        ? `node_succeeded · ${group.nodeId}[${indexes[0]}]`
        : `node_succeeded · ${group.nodeId}[${indexes[0]}–${indexes[indexes.length - 1]}]`;
  return {
    ...publicRow,
    // Casing is context-aware: the branch label leads its own sentence (sentence-case),
    // but after the "Finished" prefix the label is a mid-sentence continuation and stays
    // lowercase — "Finished Fetch issues" would be grammatically wrong (COPY.md sentence case).
    title: hasBranches
      ? `${sentenceCase(label)} — ${group.count} of ${branchTotal} clean`
      : `Finished ${label}`,
    micro,
  };
}
