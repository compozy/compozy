import {
  formatContextClock,
  formatContextPercent,
  formatContextTokens,
  formatContextTurn,
} from "./context-format";
import {
  sessionContextRingState,
  type SessionContextRingState,
  type SessionContextView,
} from "./session-context";

// View models for the composer control and the Context meter. Every branch that
// decides copy, tone, or geometry lives here so the components stay declarative.

export interface SessionContextRingGeometry {
  track: { stroke: string; dasharray?: string };
  /** Absent for unknown, loading, and used-only: without a share there is no arc. */
  arc?: { stroke: string; linecap: "round" | "butt"; dotted: boolean };
  /** Used-only: a centre dot says "a count, not a share". */
  dot: boolean;
}

export function describeSessionContextRing(
  state: SessionContextRingState,
  fraction: number,
  warning: boolean
): SessionContextRingGeometry {
  const dashed = state === "unknown" || state === "loading";
  const track = dashed
    ? { stroke: "var(--color-faint)", dasharray: "2 2.2" }
    : { stroke: "var(--color-line-strong)" };
  if (dashed || state === "used-only") return { track, dot: state === "used-only" };
  const dotted = state === "stale";
  const stroke = warning
    ? "var(--color-warning)"
    : dotted
      ? "var(--color-subtle)"
      : "var(--color-fg)";
  return { track, arc: { stroke, linecap: fraction > 0 ? "round" : "butt", dotted }, dot: false };
}

export interface SessionContextChipView {
  label: string;
  tone: "neutral" | "warning";
  form: "tint" | "hollow";
}

/** Meter chip: reported · stale · estimated size · near compaction · unavailable (hollow). */
export function describeSessionContextChip(context: SessionContextView): SessionContextChipView {
  if (context.loading && context.used == null)
    return { label: "loading", tone: "neutral", form: "tint" };
  if (context.state === "unavailable")
    return { label: "unavailable", tone: "neutral", form: "hollow" };
  if (context.stale) return { label: "stale", tone: "warning", form: "tint" };
  if (context.warning) return { label: "near compaction", tone: "warning", form: "tint" };
  if (context.state === "estimated_size")
    return { label: "estimated size", tone: "neutral", form: "tint" };
  return { label: context.state, tone: "neutral", form: "tint" };
}

export type SessionContextTooltipRow =
  | { kind: "numbers"; percent?: string; amount: string; warning: boolean }
  | { kind: "headline"; text: string }
  | {
      kind: "state";
      chip?: { label: "reported" | "stale"; tone: "neutral" | "warning" };
      asOf?: string;
    }
  | { kind: "sentence"; text: string }
  | { kind: "policy"; text: string };

export interface SessionContextControlView {
  state: SessionContextRingState;
  /** Accessible name; ", stale" is appended by the caller's freshness. */
  label: string;
  fraction: number;
  warning: boolean;
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

function compactionPolicy(context: SessionContextView): string | undefined {
  const threshold = agentThreshold(context);
  return threshold == null ? undefined : `Compaction runs at ${formatContextPercent(threshold)}`;
}

/** The threshold only means something against an agent-reported window. */
function agentThreshold(context: SessionContextView): number | undefined {
  if (context.size_source !== "agent" || context.pressure_threshold == null) return undefined;
  return context.pressure_threshold;
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
      rows.push({ kind: "sentence", text: "This agent hasn't reported context usage." });
    }
    return rows;
  }
  rows.push({
    kind: "numbers",
    percent: ratio != null ? formatContextPercent(ratio) : undefined,
    amount: amountLabel(used, context.size),
    warning: context.warning,
  });
  const asOf = context.reported_turn_id
    ? `as of turn ${formatContextTurn(context.reported_turn_id)}`
    : undefined;
  const chip = unavailable
    ? undefined
    : context.stale
      ? ({ label: "stale", tone: "warning" } as const)
      : ({ label: "reported", tone: "neutral" } as const);
  if (chip || asOf) rows.push({ kind: "state", chip, asOf });
  if (unavailable) rows.push({ kind: "sentence", text: "Usage unavailable" });
  if (context.size_source === "catalog") {
    rows.push({ kind: "sentence", text: "Window from model catalog." });
  }
  const policy = compactionPolicy(context);
  if (context.warning && policy) rows.push({ kind: "policy", text: policy });
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
    warning: context.warning,
    rows: tooltipRows(context, state, unavailable),
  };
}

export interface SessionContextTiersView {
  total: number;
  used: number;
  /** The agent's `used` dropped since these rows were sent: the CompozyOS tier dims. */
  stale: boolean;
  /** Threshold share of the window, only with an agent-reported window. */
  tick?: number;
  /** Absent when the daemon reported no attribution. */
  compozy?: { value: number; raw: number; exceeds: boolean };
  agent: number;
  free: number;
}

export type SessionContextMeterView =
  | { kind: "empty"; title: string; description?: string }
  | { kind: "unknown" }
  | {
      kind: "reported";
      value: string;
      amount: string;
      warning: boolean;
      chip: SessionContextChipView;
      tiers?: SessionContextTiersView;
      line: string[];
    };

function meterEmpty(context: SessionContextView, unavailable: boolean): SessionContextMeterView {
  if (context.loading) return { kind: "empty", title: "Loading context" };
  if (unavailable) return { kind: "empty", title: "Usage unavailable" };
  // Rows without a report: the agent never says how full its window is.
  if ((context.injected?.rows.length ?? 0) > 0) return { kind: "unknown" };
  return {
    kind: "empty",
    title: "No context report yet",
    description: "The meter fills once the agent reports its first turn.",
  };
}

/** "as of turn 12 · 18:03:01 · Compaction runs at 85%"; the policy leads while it is the warning. */
function reportLine(context: SessionContextView, unavailable: boolean): string[] {
  const policy = compactionPolicy(context);
  const clock = context.reported_at ? formatContextClock(context.reported_at) : "";
  const asOf = context.reported_turn_id
    ? `as of turn ${formatContextTurn(context.reported_turn_id)}${clock ? ` · ${clock}` : ""}`
    : undefined;
  const line = (context.warning ? [policy, asOf] : [asOf, policy]).filter(
    (part): part is string => part != null
  );
  if (context.size_source === "catalog") line.push("Window from model catalog.");
  if (unavailable) line.push("Usage unavailable");
  return line;
}

function meterTiers(
  context: SessionContextView,
  used: number
): SessionContextTiersView | undefined {
  const { display, injected } = context;
  if (!display) return undefined;
  const threshold = agentThreshold(context);
  return {
    total: display.total,
    used,
    stale: injected?.stale === true,
    tick: threshold == null ? undefined : Math.min(1, Math.max(0, threshold)),
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
    warning: context.warning,
    chip: describeSessionContextChip(context),
    tiers: meterTiers(context, used),
    line: reportLine(context, unavailable),
  };
}
