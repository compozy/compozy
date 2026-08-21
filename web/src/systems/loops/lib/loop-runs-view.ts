import type { PillTone } from "@compozy/ui";

import type { LoopRun, LoopRunStatus } from "../types";
import {
  isLoopRunStatus,
  isTerminalLoopStatus,
  loopStatusLabel,
  loopStatusSignal,
} from "./loop-formatters";

/** The runs roster's client-side outcome filter: `all` or one daemon run status. */
export type LoopOutcomeValue = "all" | LoopRunStatus;

function statusOf(run: LoopRun): LoopRunStatus | null {
  return isLoopRunStatus(run.status) ? run.status : null;
}

/**
 * The runs roster, answering "which runs need me" before anything else.
 *
 * The ordering is the daemon's. It ranks needs-you above live above terminal in
 * SQL, before the page is cut, so a run that needs a person cannot be stranded
 * on page two by a client-side sort of whatever happened to load (B-001). This
 * model only groups what arrives — it never reorders inside a group, and it
 * never re-reads a run to find out whether it needs attention.
 */

export type LoopRunGroupId = "needs-you" | "active" | "recent";

export interface LoopRunRow {
  run: LoopRun;
  statusLabel: string;
  statusTone: PillTone;
  statusPulse: boolean;
  needsYou: boolean;
  /** A sentence that adds something the status pill has not already said. */
  summaryLine: string | null;
  /** "step 4 of 6" while live, "2 rounds" once finished, "not started" before. */
  progressLabel: string;
}

export interface LoopRunGroup {
  id: LoopRunGroupId;
  label: string;
  rows: LoopRunRow[];
}

export interface LoopRunsEmptyState {
  title: string;
  body: string;
  actionLabel: string;
}

export interface LoopRunsRosterModel {
  /** Only groups with rows, in server order: needs-you, then active, then recent. */
  groups: LoopRunGroup[];
  total: number;
  needsYouCount: number;
  /** Set only when there is genuinely nothing — never when a read is in flight. */
  emptyState: LoopRunsEmptyState | null;
}

const GROUP_LABELS: Record<LoopRunGroupId, string> = {
  "needs-you": "Needs you",
  active: "Active",
  recent: "Recent",
};

/** Group order is the server's ranking, restated so the page cannot drift from it. */
const GROUP_ORDER: LoopRunGroupId[] = ["needs-you", "active", "recent"];

function groupOf(run: LoopRun): LoopRunGroupId {
  // Attention is a served summary, not something the page infers from status.
  if (run.attention) return "needs-you";
  return isTerminalLoopStatus(run.status) ? "recent" : "active";
}

/**
 * Sentences that earn their place. A terminal row's pill already says "Done", so
 * repeating it underneath would be noise — and the artifact that would make the
 * line worth reading is not on the list read, which is exactly the N+1 the
 * server-owned summary exists to avoid.
 */
const LIVE_SUMMARIES: Partial<Record<LoopRunStatus, string>> = {
  running: "moving on its own",
  watching: "watching for its event",
  queued: "waiting for a free slot",
  paused: "paused by an operator",
  blocked: "blocked and waiting",
};

const ATTENTION_SUMMARIES: Record<string, string> = {
  approval: "an approval is waiting",
  quarantine: "a step is quarantined",
  request: "a question is waiting",
};

function summaryLine(run: LoopRun): string | null {
  const attention = run.attention;
  if (attention) {
    const base = ATTENTION_SUMMARIES[attention.kind] ?? "waiting on you";
    const gate = run.active_gate_id?.trim();
    return gate && attention.kind === "approval" ? `${base} on “${gate}”` : base;
  }
  const status = statusOf(run);
  return status ? (LIVE_SUMMARIES[status] ?? null) : null;
}

function progressLabel(run: LoopRun): string {
  const progress = run.progress;
  if (isTerminalLoopStatus(run.status)) {
    const rounds = Math.max(progress.round, 1);
    return rounds === 1 ? "1 round" : `${rounds} rounds`;
  }
  if (progress.steps_total <= 0) return "not started";
  const base = `step ${progress.steps_done} of ${progress.steps_total}`;
  // The round counter earns its place only past round 1, on every surface.
  return progress.round > 1 ? `${base} · round ${progress.round}` : base;
}

function buildRow(run: LoopRun): LoopRunRow {
  const needsYou = Boolean(run.attention);
  const signal = loopStatusSignal(run.status);
  return {
    run,
    // A run that needs a person leads with that fact, not with the mechanism
    // that produced it.
    statusLabel: needsYou ? "Needs you" : loopStatusLabel(run.status),
    statusTone: needsYou ? "warning" : signal.tone,
    statusPulse: needsYou ? false : signal.pulse,
    needsYou,
    summaryLine: summaryLine(run),
    progressLabel: progressLabel(run),
  };
}

export function buildRunsRoster(
  runs: readonly LoopRun[],
  outcome: LoopOutcomeValue = "all"
): LoopRunsRosterModel {
  const filtered = outcome === "all" ? runs : runs.filter(run => run.status === outcome);
  const buckets = new Map<LoopRunGroupId, LoopRunRow[]>();
  for (const run of filtered) {
    const id = groupOf(run);
    const rows = buckets.get(id);
    // Server order is preserved by appending: grouping partitions, it never sorts.
    if (rows) rows.push(buildRow(run));
    else buckets.set(id, [buildRow(run)]);
  }
  const groups = GROUP_ORDER.filter(id => (buckets.get(id)?.length ?? 0) > 0).map(id => ({
    id,
    label: GROUP_LABELS[id],
    rows: buckets.get(id) ?? [],
  }));
  return {
    groups,
    total: filtered.length,
    needsYouCount: buckets.get("needs-you")?.length ?? 0,
    emptyState:
      filtered.length === 0
        ? {
            title: outcome === "all" ? "No runs yet" : "No runs match this filter",
            body:
              outcome === "all"
                ? "Start a loop from the catalog and its runs will collect here."
                : "Clear the filter to see the rest of this workspace's runs.",
            actionLabel: outcome === "all" ? "Browse loops" : "Clear filter",
          }
        : null,
  };
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

/**
 * Where a run came from (`schedule · nightly`, `cli · pedro`), read from the origin
 * the daemon recorded. Falls back to the em dash rather than guessing a starter.
 */
export function loopRunOriginLine(
  run: Pick<
    LoopRun,
    "started_origin_kind" | "started_by_kind" | "started_by_ref" | "started_origin_ref"
  >
): string {
  const originKind = run.started_origin_kind || "";
  const originRef = run.started_origin_ref || "";
  if (originKind && originRef) return `${originKind} · ${originRef}`;
  const actorKind = run.started_by_kind || "";
  const actorRef = run.started_by_ref || "";
  if (actorKind && actorRef) return `${actorKind} · ${actorRef}`;
  return originKind || originRef || actorKind || actorRef || "—";
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
