import type { SessionUsageTurnsResponse } from "../types";
import { formatContextPercent, formatContextTokens } from "./context-format";
import {
  contextClearedBy,
  sessionContextRingState,
  type SessionContextRingState,
  type SessionContextView,
} from "./session-context";

// View models for the composer control and the Context meter. Every branch that
// decides copy, tone, or geometry lives here so the components stay declarative.
// Both surfaces default to quiet: a plain report shows its numbers only, and a
// chip appears just when the reading carries a caveat.

export interface SessionContextRingGeometry {
  track: { stroke: string; dasharray?: string };
  /** Absent for unknown, loading, and used-only: without a share there is no arc. */
  arc?: { stroke: string; linecap: "round" | "butt"; dotted: boolean };
  /** Used-only: a centre dot says "a count, not a share". */
  dot: boolean;
}

export function describeSessionContextRing(
  state: SessionContextRingState,
  fraction: number
): SessionContextRingGeometry {
  const dashed = state === "unknown" || state === "loading";
  const track = dashed
    ? { stroke: "var(--color-faint)", dasharray: "2 2.2" }
    : { stroke: "var(--color-line-strong)" };
  if (dashed || state === "used-only") return { track, dot: state === "used-only" };
  const dotted = state === "stale";
  const stroke = dotted ? "var(--color-subtle)" : "var(--color-fg)";
  return { track, arc: { stroke, linecap: fraction > 0 ? "round" : "butt", dotted }, dot: false };
}

export interface SessionContextChipView {
  label: string;
  tone: "neutral" | "warning";
  form: "tint" | "hollow";
}

/** Meter chip: loading · unavailable (hollow) · stale · estimated size; a plain report has none. */
export function describeSessionContextChip(
  context: SessionContextView
): SessionContextChipView | undefined {
  if (context.loading && context.used == null)
    return { label: "loading", tone: "neutral", form: "tint" };
  if (context.state === "unavailable")
    return { label: "unavailable", tone: "neutral", form: "hollow" };
  if (context.stale) return { label: "stale", tone: "warning", form: "tint" };
  if (context.state === "estimated_size")
    return { label: "estimated size", tone: "neutral", form: "tint" };
  return undefined;
}

/** Why a context reading is empty, in the sentence the tooltip and the meter share. */
export const CONTEXT_NEVER_REPORTED_SENTENCE = "This agent hasn't reported context usage.";
export const CONTEXT_COMPACTED_SENTENCE =
  "Context compacted. Waiting for the agent's next usage report.";

function unknownReadingSentence(context: SessionContextView): string {
  return contextClearedBy(context) ? CONTEXT_COMPACTED_SENTENCE : CONTEXT_NEVER_REPORTED_SENTENCE;
}

export type SessionContextTooltipRow =
  | { kind: "numbers"; percent?: string; amount: string }
  | { kind: "headline"; text: string }
  | { kind: "stale" }
  | { kind: "sentence"; text: string };

export interface SessionContextControlView {
  state: SessionContextRingState;
  /** Accessible name; ", stale" is appended by the caller's freshness. */
  label: string;
  fraction: number;
  rows: SessionContextTooltipRow[];
}

function controlLabel(
  context: SessionContextView,
  state: SessionContextRingState,
  unavailable: boolean
): string {
  if (state === "loading") return "Context usage loading";
  if (context.ratio != null) return `Context ${formatContextPercent(context.ratio)} used`;
  if (context.used != null) return `Context ${formatContextTokens(context.used)} used`;
  return unavailable ? "Context usage unavailable" : "Context usage unknown";
}

function amountLabel(used: number, size: number | null | undefined): string {
  return `${formatContextTokens(used)}${size != null ? ` / ${formatContextTokens(size)}` : " used"}`;
}

function tooltipRows(
  context: SessionContextView,
  state: SessionContextRingState,
  unavailable: boolean
): SessionContextTooltipRow[] {
  const rows: SessionContextTooltipRow[] = [];
  const { used, ratio } = context;
  if (used == null) {
    const text =
      state === "loading"
        ? "Loading context usage"
        : unavailable
          ? "Usage unavailable"
          : "Context usage unknown";
    rows.push({ kind: "headline", text });
    if (state === "unknown" && !unavailable) {
      rows.push({ kind: "sentence", text: unknownReadingSentence(context) });
    }
    return rows;
  }
  rows.push({
    kind: "numbers",
    percent: ratio != null ? formatContextPercent(ratio) : undefined,
    amount: amountLabel(used, context.size),
  });
  if (!unavailable && context.stale) rows.push({ kind: "stale" });
  if (unavailable) rows.push({ kind: "sentence", text: "Usage unavailable" });
  if (context.size_source === "catalog") {
    rows.push({ kind: "sentence", text: "Size from the model's specs." });
  }
  return rows;
}

export function describeSessionContextControl(
  context: SessionContextView
): SessionContextControlView {
  const state = sessionContextRingState(context);
  const unavailable = context.state === "unavailable";
  return {
    state,
    label: controlLabel(context, state, unavailable),
    fraction: context.ratio == null ? 0 : Math.max(0, Math.min(1, context.ratio)),
    rows: tooltipRows(context, state, unavailable),
  };
}

export interface SessionContextTiersView {
  total: number;
  used: number;
  /** The agent's `used` dropped since these rows were sent: the CompozyOS tier dims. */
  stale: boolean;
  /** Absent when the daemon reported no attribution. */
  compozy?: { value: number; raw: number; exceeds: boolean };
  agent: number;
  free: number;
}

export type SessionContextMeterView =
  | { kind: "empty"; title: string; description?: string }
  | { kind: "unknown"; sentence: string }
  | {
      kind: "reported";
      value: string;
      amount: string;
      chip?: SessionContextChipView;
      tiers?: SessionContextTiersView;
    };

function meterEmpty(context: SessionContextView, unavailable: boolean): SessionContextMeterView {
  if (context.loading) return { kind: "empty", title: "Loading context" };
  if (unavailable) return { kind: "empty", title: "Usage unavailable" };
  // Rows without a report, or a compaction that cleared the reading: the agent
  // has not said how full its window is.
  if (contextClearedBy(context) || (context.injected?.rows.length ?? 0) > 0) {
    return { kind: "unknown", sentence: unknownReadingSentence(context) };
  }
  return {
    kind: "empty",
    title: "No context report yet",
    description: "The meter fills once the agent reports its first turn.",
  };
}

function meterTiers(
  context: SessionContextView,
  used: number
): SessionContextTiersView | undefined {
  const { display, injected } = context;
  if (!display) return undefined;
  return {
    total: display.total,
    used,
    stale: injected?.stale === true,
    compozy: injected
      ? { value: display.compozy, raw: injected.tokens, exceeds: context.estimateExceedsReported }
      : undefined,
    agent: display.agent,
    free: display.free,
  };
}

export function describeSessionContextMeter(context: SessionContextView): SessionContextMeterView {
  const unavailable = context.state === "unavailable";
  const { used, size, ratio } = context;
  if (used == null) return meterEmpty(context, unavailable);
  return {
    kind: "reported",
    value: ratio != null ? formatContextPercent(ratio) : formatContextTokens(used),
    amount: `${ratio != null ? formatContextTokens(used) : "used"}${size != null ? ` / ${formatContextTokens(size)}` : ""}`,
    chip: describeSessionContextChip(context),
    tiers: meterTiers(context, used),
  };
}

export interface SessionCompactionMarkerView {
  key: string;
  /** Position in the session's event ledger; markers interleave with turns by it. */
  sequence: number;
  label: "Agent compaction" | "Requested compaction";
  /** The daemon's status, vendor values included, verbatim. */
  status: string;
  statusLabel: string;
  trigger: string;
  /** Occupancy when the compaction was observed; absent when the agent sent none. */
  before?: number;
  /**
   * The first occupancy the agent reported after the compaction ended, as the daemon
   * established it (`context_after`); absent until one exists. Never read off `turns[]`:
   * those rows merge counters per turn and cannot say when an occupancy was observed.
   */
  after?: number;
}

/** One row per observed agent compaction, in ledger order. */
export function describeSessionCompactionMarkers(
  data: SessionUsageTurnsResponse | undefined
): SessionCompactionMarkerView[] {
  const markers = [...(data?.compactions ?? [])].sort((a, b) => a.sequence - b.sequence);
  return markers.map(marker => ({
    key: marker.compaction_id,
    sequence: marker.sequence,
    label: marker.trigger === "requested" ? "Requested compaction" : "Agent compaction",
    status: marker.status,
    statusLabel: marker.status === "in_progress" ? "in progress" : marker.status,
    trigger: marker.trigger,
    before: marker.context_used ?? undefined,
    after: marker.context_after?.used,
  }));
}
