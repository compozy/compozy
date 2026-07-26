import type { LoopRunRecord } from "../types";
import type { LoopApprovalFact } from "./loop-events";
import { isTerminalLoopStatus } from "./loop-formatters";
import { formatTokenCount } from "./loop-runs-view";

/**
 * The Usage rail model (redesign spec §3/§6): four plain rows — Time, Tokens,
 * Cost, Rounds — plus the one policy note. Values tint warn at ≥90% of a
 * ceiling and danger at/over it; unbounded caps render `∞` (never a bar, never
 * an invented ceiling).
 */

export type LoopUsageTone = "neutral" | "warn" | "danger";

export type LoopUsageKey = "time" | "tokens" | "cost" | "rounds";

export interface LoopRunUsageRow {
  key: LoopUsageKey;
  label: string;
  value: string;
  /** Faint trailing slot (`/ 45m`, `/ ∞`, `estimate`). */
  max: string;
  tone: LoopUsageTone;
}

/**
 * Coarse display-only USD estimate per 1M tokens (ADR-017 §9.5.2). AGH enforces
 * no USD cap and the run projection carries no per-model price, so this single
 * documented blended heuristic exists for visibility ONLY — never a billed
 * figure and never a budget dimension. It is superseded the day the daemon
 * exposes a real per-run rate.
 */
export const LOOP_COST_PER_1M_TOKENS = 5;

/** `~$1.34` — the leading `~` keeps the estimate qualifier next to the value. */
export function deriveCostEstimate(tokensUsed: number): string {
  const usd = (Math.max(0, tokensUsed) / 1_000_000) * LOOP_COST_PER_1M_TOKENS;
  return `~$${usd.toFixed(2)}`;
}

/** Warn at 90% of a ceiling, danger at/over it. */
function ratioTone(ratio: number): LoopUsageTone {
  if (ratio >= 1) return "danger";
  if (ratio >= 0.9) return "warn";
  return "neutral";
}

/**
 * Run elapsed seconds (§5.4): only `running` ticks against the wall clock
 * (`now − started_at`); every parked and terminal state shows the frozen
 * `created_at → last_progress_at` span.
 */
export function runElapsedSeconds(run: LoopRunRecord, nowMs: number): number {
  if (run.status === "running") {
    const started = Date.parse(run.started_at);
    if (Number.isNaN(started)) return 0;
    return Math.max(0, Math.round((nowMs - started) / 1000));
  }
  const created = Date.parse(run.created_at);
  const last = Date.parse(run.last_progress_at);
  if (Number.isNaN(created) || Number.isNaN(last) || last < created) return 0;
  return Math.round((last - created) / 1000);
}

/**
 * Uses the terminal status event because the status CAS does not refresh
 * `last_progress_at`; malformed or missing events fall back to the durable span.
 */
export function terminalRunElapsedSeconds(
  run: LoopRunRecord,
  terminalAt: string | undefined,
  nowMs: number
): number {
  if (run.status !== "running" && terminalAt) {
    const created = Date.parse(run.created_at);
    const ended = Date.parse(terminalAt);
    if (!Number.isNaN(created) && !Number.isNaN(ended) && ended >= created) {
      return Math.round((ended - created) / 1000);
    }
  }
  return runElapsedSeconds(run, nowMs);
}

/** `22m 14s` / `1h 04m` — the ticking clock voice from the prototypes. */
export function formatClockDuration(seconds: number): string {
  const total = Math.max(0, Math.floor(seconds));
  const hours = Math.floor(total / 3600);
  const minutes = Math.floor((total % 3600) / 60);
  const secs = total % 60;
  if (hours > 0) return `${hours}h ${String(minutes).padStart(2, "0")}m`;
  return `${minutes}m ${String(secs).padStart(2, "0")}s`;
}

/** Compact wall-clock ceiling label (`45m`, `1.5h`, `2d`). */
function wallLabel(seconds: number): string {
  if (seconds <= 0) return "0s";
  const day = 86_400;
  const hour = 3_600;
  const minute = 60;
  if (seconds >= day) return `${(seconds / day).toFixed(seconds % day === 0 ? 0 : 1)}d`;
  if (seconds >= hour) {
    const hours = seconds / hour;
    return `${Number.isInteger(hours) ? hours : hours.toFixed(1)}h`;
  }
  if (seconds >= minute) return `${Math.round(seconds / minute)}m`;
  return `${seconds}s`;
}

/** Builds the four Usage rows from the run projection + the ticking elapsed. */
export function buildRunUsage(run: LoopRunRecord, elapsedSeconds: number): LoopRunUsageRow[] {
  const wallCapped = run.budget_wall_sec > 0;
  const tokensCapped = run.budget_tokens > 0;
  const roundsCapped = run.iteration_cap > 0;
  return [
    {
      key: "time",
      label: "Time",
      value: formatClockDuration(elapsedSeconds),
      max: wallCapped ? `/ ${wallLabel(run.budget_wall_sec)}` : "/ ∞",
      tone: wallCapped ? ratioTone(elapsedSeconds / run.budget_wall_sec) : "neutral",
    },
    {
      key: "tokens",
      label: "Tokens",
      value: formatTokenCount(run.tokens_used),
      max: tokensCapped ? `/ ${formatTokenCount(run.budget_tokens)}` : "/ ∞",
      tone: tokensCapped ? ratioTone(run.tokens_used / run.budget_tokens) : "neutral",
    },
    {
      key: "cost",
      label: "Cost",
      value: deriveCostEstimate(run.tokens_used),
      max: "estimate",
      tone: "neutral",
    },
    {
      key: "rounds",
      label: "Rounds",
      value: String(run.generation),
      max: roundsCapped ? `/ ${run.iteration_cap}` : "/ ∞",
      tone: roundsCapped ? ratioTone(run.generation / run.iteration_cap) : "neutral",
    },
  ];
}

/** The one Usage note (§6 copy + prototype state variants); null on terminal runs. */
export function usageNote(run: LoopRunRecord): string | null {
  if (isTerminalLoopStatus(run.status)) return null;
  if (run.status === "watching") {
    return "Watching costs nothing — time only counts while the run is working.";
  }
  if (run.status === "paused") {
    return "Paused runs use nothing. Limits pick up where they left off on resume.";
  }
  const escalate = run.budget_on_exceeded === "escalate";
  if (run.status === "needs-approval" && escalate) {
    return "Cost is an estimate (tokens × rate), never a cap. This run escalates limits to you instead of failing.";
  }
  return escalate
    ? "Cost is an estimate (tokens × rate), never a cap. If a limit is reached, this run pauses and asks you — it doesn't fail silently."
    : "Cost is an estimate (tokens × rate), never a cap. If a limit is reached, this run stops as exhausted.";
}

/**
 * The "Needs you" facts fallback (§3): when the `needs_approval` payload carries
 * no facts, the usage snapshot stands in — truthful, never fabricated context.
 */
export function usageSnapshotFacts(run: LoopRunRecord, elapsedSeconds: number): LoopApprovalFact[] {
  return [
    {
      label: "Time used",
      value: `${formatClockDuration(elapsedSeconds)} of ${
        run.budget_wall_sec > 0 ? wallLabel(run.budget_wall_sec) : "∞"
      }`,
    },
    {
      label: "Tokens",
      value: `${formatTokenCount(run.tokens_used)} / ${
        run.budget_tokens > 0 ? formatTokenCount(run.budget_tokens) : "∞"
      }`,
    },
    { label: "Cost", value: `${deriveCostEstimate(run.tokens_used)} est.` },
    { label: "Round", value: String(run.generation) },
  ];
}
