import { LOOP_RUN_TERMINAL_STATUSES } from "@/generated/loop-enums";

import type { LoopRun, LoopRunStatus } from "../types";
import { isLoopRunStatus, isTerminalLoopStatus, loopStatusLabel } from "./loop-formatters";

/** Canonical status order (live first, then terminal) for the outcome filter. */
const STATUS_ORDER: readonly LoopRunStatus[] = [
  "running",
  "watching",
  "needs-approval",
  "paused",
  "queued",
  ...LOOP_RUN_TERMINAL_STATUSES,
];

/** Terminal problem states that surface in the "Needs a look" KPI. */
const NEEDS_A_LOOK: readonly LoopRunStatus[] = ["failed", "exhausted", "stalled", "blocked"];

/** Live states that count toward "Active now". */
const LIVE_STATUSES: readonly LoopRunStatus[] = [
  "running",
  "watching",
  "needs-approval",
  "paused",
  "queued",
];

function statusOf(run: LoopRun): LoopRunStatus | null {
  return isLoopRunStatus(run.status) ? run.status : null;
}

function countByStatus(runs: readonly LoopRun[]): Map<LoopRunStatus, number> {
  const counts = new Map<LoopRunStatus, number>();
  for (const run of runs) {
    const status = statusOf(run);
    if (status) counts.set(status, (counts.get(status) ?? 0) + 1);
  }
  return counts;
}

function breakdown(counts: Map<LoopRunStatus, number>, statuses: readonly LoopRunStatus[]): string {
  const parts: string[] = [];
  for (const status of statuses) {
    const count = counts.get(status) ?? 0;
    if (count > 0) parts.push(`${count} ${loopStatusLabel(status).toLowerCase()}`);
  }
  return parts.join(" · ");
}

function isSameLocalDay(iso: string | undefined, now: Date): boolean {
  if (!iso) return false;
  const at = new Date(iso);
  if (Number.isNaN(at.getTime())) return false;
  return (
    at.getFullYear() === now.getFullYear() &&
    at.getMonth() === now.getMonth() &&
    at.getDate() === now.getDate()
  );
}

export interface LoopKpi {
  count: number;
  detail: string;
}

export interface LoopRunKpis {
  activeNow: LoopKpi;
  awaitingYou: LoopKpi;
  doneToday: LoopKpi;
  needsALook: LoopKpi;
}

/**
 * Derives the four workspace-wide Runs KPIs from the run set (a derived read
 * model, §9.11 — truthful UI, no invented metric). "Done today" uses
 * `last_progress_at` as the terminal timestamp because the run projection exposes
 * no dedicated `ended_at`.
 */
export function buildRunKpis(runs: readonly LoopRun[], now: Date = new Date()): LoopRunKpis {
  const counts = countByStatus(runs);
  const liveCount = LIVE_STATUSES.reduce((total, status) => total + (counts.get(status) ?? 0), 0);
  const awaiting = counts.get("needs-approval") ?? 0;
  const needsLook = NEEDS_A_LOOK.reduce((total, status) => total + (counts.get(status) ?? 0), 0);
  const doneToday = runs.reduce((total, run) => {
    if (statusOf(run) !== "done") return total;
    return total + (isSameLocalDay(run.last_progress_at, now) ? 1 : 0);
  }, 0);
  return {
    activeNow: {
      count: liveCount,
      detail: breakdown(counts, LIVE_STATUSES) || "none active",
    },
    awaitingYou: {
      count: awaiting,
      detail: awaiting > 0 ? "at a human gate" : "none waiting",
    },
    doneToday: {
      count: doneToday,
      detail: doneToday > 0 ? "since midnight" : "none yet today",
    },
    needsALook: {
      count: needsLook,
      detail: breakdown(counts, NEEDS_A_LOOK) || "all clear",
    },
  };
}

export interface LoopOutcomeSegment {
  value: "all" | LoopRunStatus;
  label: string;
  count: number;
}

/**
 * Builds the data-driven outcome filter: `All` plus one segment per status
 * actually present in the window, in canonical order. Paused/Queued only appear
 * once such runs exist (design §4.5).
 */
export function buildOutcomeSegments(runs: readonly LoopRun[]): LoopOutcomeSegment[] {
  const counts = countByStatus(runs);
  const segments: LoopOutcomeSegment[] = [{ value: "all", label: "All", count: runs.length }];
  for (const status of STATUS_ORDER) {
    const count = counts.get(status) ?? 0;
    if (count > 0) segments.push({ value: status, label: loopStatusLabel(status), count });
  }
  return segments;
}

export interface LoopRunPartition {
  active: LoopRun[];
  past: LoopRun[];
}

/** Splits runs into the Active (live) and Past (terminal) tables, applying the outcome filter. */
export function partitionRuns(
  runs: readonly LoopRun[],
  outcome: "all" | LoopRunStatus = "all"
): LoopRunPartition {
  const active: LoopRun[] = [];
  const past: LoopRun[] = [];
  for (const run of runs) {
    if (outcome !== "all" && run.status !== outcome) continue;
    if (isTerminalLoopStatus(run.status)) past.push(run);
    else active.push(run);
  }
  return { active, past };
}

/** Compact token count for a run's budget cell (`412K`, `2.4M`, `12K`, `0`). */
export function formatTokenCount(tokens: number): string {
  if (!Number.isFinite(tokens) || tokens <= 0) return "0";
  if (tokens >= 1_000_000) {
    const millions = tokens / 1_000_000;
    return `${Number.isInteger(millions) ? millions : millions.toFixed(1)}M`;
  }
  if (tokens >= 1_000) {
    const thousands = tokens / 1_000;
    // One-decimal precision, matching `formatTokenBudget` (1500 -> "1.5K", never "2K").
    return `${Number.isInteger(thousands) ? thousands : thousands.toFixed(1)}K`;
  }
  return String(Math.round(tokens));
}

/** `generation / cap` label for a run row (`2 / 50`, `5 / ∞`). */
export function runGenerationLabel(run: Pick<LoopRun, "generation" | "iteration_cap">): string {
  const cap = run.iteration_cap === 0 ? "∞" : String(run.iteration_cap);
  return `${run.generation} / ${cap}`;
}

function formatInputValue(value: unknown): string {
  if (typeof value === "string") return value;
  if (typeof value === "boolean" || typeof value === "number") return String(value);
  return JSON.stringify(value);
}

/**
 * A compact preview of a run's resolved inputs for the runs table (`slug: x ·
 * branch: main`), truthful to `loop_run.inputs`. The run projection carries no
 * per-run goal, so the inputs stand in for "what this run is about".
 */
export function formatRunInputs(inputs?: Record<string, unknown>, max = 2): string {
  if (!inputs) return "";
  return Object.entries(inputs)
    .slice(0, max)
    .map(([key, value]) => `${key}: ${formatInputValue(value)}`)
    .join(" · ");
}

export type LoopBudgetTone = "neutral" | "warn" | "danger";

export interface LoopBudgetBar {
  tokensLabel: string;
  hasCap: boolean;
  /** Fill percent `0..100` when a budget is set; null when unlimited. */
  percent: number | null;
  tone: LoopBudgetTone;
}

/**
 * Models a run's token budget mini-bar. Uncapped runs (`budget_tokens == 0`,
 * opt-in budgets off) show the raw token count with no fill; a set budget renders
 * a clamped fill that warns near the ceiling and turns danger at/over it.
 */
export function loopBudgetBar(run: Pick<LoopRun, "tokens_used" | "budget_tokens">): LoopBudgetBar {
  const tokensLabel = `${formatTokenCount(run.tokens_used)} tok`;
  if (!run.budget_tokens || run.budget_tokens <= 0) {
    return { tokensLabel, hasCap: false, percent: null, tone: "neutral" };
  }
  const ratio = run.tokens_used / run.budget_tokens;
  const percent = Math.max(0, Math.min(100, Math.round(ratio * 100)));
  const tone: LoopBudgetTone = ratio >= 1 ? "danger" : ratio >= 0.9 ? "warn" : "neutral";
  return { tokensLabel, hasCap: true, percent, tone };
}
