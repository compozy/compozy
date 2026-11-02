// Pure presentation rules for subagent rows (`_uiux.md` S1–S3, S5, S9, S10 and
// DESIGN-NOTES §Signal). Every duration derives from daemon timestamps; the
// components only pick a clock source (frozen vs the shared ticker).

import type { StateGlyphState, StatusDotTone } from "@compozy/ui";

import { markdownLineToPlainText } from "../../lib/markdown-plain-text";
import type { SubagentLocationView, SubagentStatus, SubagentView } from "./types";

export const SUBAGENT_BANNER_STOP_FAILED = "Could not stop subagents.";
export const SUBAGENT_ROW_STOP_FAILED = "Could not stop subagent";

/** Avatar dot per state. Queued draws a hollow indicator ring (`ring` = null tone). */
export interface SubagentDotSignal {
  tone: StatusDotTone | null;
  pulse: boolean;
}

export function subagentDot(status: SubagentStatus): SubagentDotSignal {
  switch (status) {
    case "queued":
      return { tone: null, pulse: false };
    case "running":
      return { tone: "success", pulse: true };
    case "waiting":
      return { tone: "brand", pulse: false };
    case "completed":
      return { tone: "success", pulse: false };
    case "failed":
      return { tone: "destructive", pulse: false };
    case "canceled":
    case "interrupted":
      return { tone: "faint", pulse: false };
  }
}

const LIVE_STATUSES: ReadonlySet<SubagentStatus> = new Set(["queued", "running", "waiting"]);

/** A waiting subagent counts as live: it has not settled. */
export function isSubagentLive(status: SubagentStatus): boolean {
  return LIVE_STATUSES.has(status);
}

export function isSubagentStoppable(subagent: SubagentView): boolean {
  return subagent.origin === "delegated" && isSubagentLive(subagent.status);
}

/** Whitespace collapsed to single spaces. */
export function collapseWhitespace(text: string | null | undefined): string {
  return (text ?? "").replace(/\s+/g, " ").trim();
}

/** First non-empty line as plain text (agent output is markdown, D-09), whitespace collapsed. */
export function firstLine(text: string | null | undefined): string {
  for (const line of (text ?? "").split(/\r?\n/)) {
    const plain = collapseWhitespace(markdownLineToPlainText(line));
    if (plain !== "") return plain;
  }
  return "";
}

export type SubagentLineTwoTone = "default" | "destructive";

export interface SubagentCardLines {
  /** Progress or result text under the title; empty when there is none (the trail carries the word). */
  lineTwo: string;
  lineTwoTone: SubagentLineTwoTone;
}

/**
 * Card line 2 (UT-W05): live → progress; completed → the result's first line;
 * failed → the error's first line in danger; canceled or interrupted → the
 * result (or last progress). The status word lives in the trail, never here.
 */
export function subagentCardLines(subagent: SubagentView): SubagentCardLines {
  const failed = subagent.status === "failed";
  let text: string;
  if (isSubagentLive(subagent.status)) {
    text = firstLine(subagent.progress);
  } else if (failed) {
    text = firstLine(subagent.error) || firstLine(subagent.result_preview);
  } else if (subagent.status === "completed") {
    text = firstLine(subagent.result_preview);
  } else {
    text = firstLine(subagent.result_preview) || firstLine(subagent.progress);
  }
  return { lineTwo: text, lineTwoTone: failed ? "destructive" : "default" };
}

function parseMs(iso: string | null | undefined): number | null {
  if (!iso) return null;
  const ms = Date.parse(iso);
  return Number.isFinite(ms) ? ms : null;
}

/**
 * The clock behind one elapsed label: `ticking` counts from `startMs` on the
 * shared ticker; `frozen` is a fixed server-side span; `none` shows nothing.
 */
export type SubagentElapsedClock =
  | { kind: "none" }
  | { kind: "frozen"; ms: number }
  | { kind: "ticking"; startMs: number };

/**
 * Elapsed source (UT-W06): server `started_at` to `settled_at`. A live row
 * ticks; a stale row (stream not confirmed since reconnect) freezes at its last
 * server `updated_at` instead of ticking.
 */
export function subagentElapsedClock(
  subagent: SubagentView,
  options: { stale?: boolean } = {}
): SubagentElapsedClock {
  const startMs = parseMs(subagent.started_at);
  if (startMs === null) return { kind: "none" };
  const settledMs = parseMs(subagent.settled_at);
  if (settledMs !== null) return { kind: "frozen", ms: settledMs - startMs };
  if (!isSubagentLive(subagent.status)) return { kind: "none" };
  if (options.stale) {
    const updatedMs = parseMs(subagent.updated_at);
    return updatedMs === null ? { kind: "none" } : { kind: "frozen", ms: updatedMs - startMs };
  }
  return { kind: "ticking", startMs };
}

/** Word ink: `live` for work in flight, `attention` for needs-you, `settled` for finished rows. */
export type SubagentDisplayTone = "live" | "attention" | "destructive" | "settled";

/** One row's status, shared by every subagent surface (sidebar, roster, card, hover, group). */
export interface SubagentDisplay {
  glyph: StateGlyphState;
  word: string;
  tone: SubagentDisplayTone;
  /** Finished without needing anyone (Done, Idle, Canceled, Interrupted): the title dims. */
  settled: boolean;
  /** The trail's clock: ticking while Working, frozen once Done, otherwise none. */
  clock: SubagentElapsedClock;
}

const NO_CLOCK: SubagentElapsedClock = { kind: "none" };
const READ_DELIVERIES: ReadonlySet<string> = new Set(["delivered", "acknowledged"]);

const display = (
  glyph: StateGlyphState,
  word: string,
  tone: SubagentDisplayTone,
  clock: SubagentElapsedClock = NO_CLOCK
): SubagentDisplay => ({ glyph, word, tone, settled: tone === "settled", clock });

/**
 * The one derived display state, in the session `StateGlyph` vocabulary for native and delegated
 * rows alike. A settled child that resumed reads Working; a delivered one gone quiet reads Idle.
 */
export function subagentDisplay(
  subagent: SubagentView,
  options: { stale?: boolean } = {}
): SubagentDisplay {
  const { status } = subagent;
  if (!isSubagentLive(status) && subagent.child_activity === "working") {
    return display("running", "Working", "live");
  }
  switch (status) {
    case "queued":
      return display("queued", "Queued", "live");
    case "running":
      return subagent.work_state === "waiting_for_children"
        ? display("delegated", "Waiting on subagents", "live")
        : display("running", "Working", "live", subagentElapsedClock(subagent, options));
    case "waiting":
      return display("attention", "Waiting for you", "attention");
    case "completed":
      // Idle keeps the delegation's frozen duration: reading the result does
      // not change how long the work took.
      return subagent.child_activity === "idle" && READ_DELIVERIES.has(subagent.delivery)
        ? display("idle", "Idle", "settled", subagentElapsedClock(subagent, options))
        : display("done", "Done", "settled", subagentElapsedClock(subagent, options));
    case "failed":
      return display("failed", "Failed", "destructive");
    case "canceled":
      return display("stopped", "Canceled", "settled");
    case "interrupted":
      return display("stopped", "Interrupted", "settled");
  }
}

const GLYPH_URGENCY: readonly StateGlyphState[] = [
  "attention",
  "failed",
  "running",
  "delegated",
  "queued",
  "done",
  "idle",
  "stopped",
];

/** Group aggregate (transcript VC-03): the most urgent member glyph present. */
export function subagentGroupGlyph(subagents: readonly SubagentView[]): StateGlyphState {
  const present = new Set(subagents.map(subagent => subagentDisplay(subagent).glyph));
  return GLYPH_URGENCY.find(glyph => present.has(glyph)) ?? "idle";
}

/**
 * Group span (UT-W07): first `started_at` to last `settled_at`, ticking while a
 * member is live. Withheld while any member's end is unknown: no start yet, a
 * stale live row, or a settled row without `settled_at`.
 */
export function subagentGroupClock(
  subagents: readonly SubagentView[],
  options: { stale?: boolean } = {}
): SubagentElapsedClock {
  let firstStart = Number.POSITIVE_INFINITY;
  let lastSettle = Number.NEGATIVE_INFINITY;
  let live = false;
  for (const subagent of subagents) {
    const startMs = parseMs(subagent.started_at);
    if (startMs === null) return { kind: "none" };
    firstStart = Math.min(firstStart, startMs);
    if (isSubagentLive(subagent.status)) {
      if (options.stale) return { kind: "none" };
      live = true;
      continue;
    }
    // A resumed child works past its delegation's settle instant, so no
    // delegation span describes the group's current work.
    if (subagent.child_activity === "working") return { kind: "none" };
    const settledMs = parseMs(subagent.settled_at);
    if (settledMs === null) return { kind: "none" };
    lastSettle = Math.max(lastSettle, settledMs);
  }
  if (subagents.length === 0) return { kind: "none" };
  if (live) return { kind: "ticking", startMs: firstStart };
  return { kind: "frozen", ms: lastSettle - firstStart };
}

export type SubagentSummaryTone = "live" | "failed" | "settled";

export interface SubagentGroupSummary {
  label: string;
  summary: string;
  tone: SubagentSummaryTone;
  settled: boolean;
}

const plural = (count: number, one: string, many = `${one}s`) => (count === 1 ? one : many);

/**
 * Group copy (UT-W07): `3 subagents` and `2 working · 1 needs you · 1 done ·
 * 1 failed`. Tone is `failed` when any failed, else `live` while any is live.
 */
export function subagentGroupSummary(subagents: readonly SubagentView[]): SubagentGroupSummary {
  const counts = { working: 0, waiting: 0, done: 0, failed: 0, canceled: 0, interrupted: 0 };
  for (const subagent of subagents) {
    const { glyph } = subagentDisplay(subagent);
    if (glyph === "attention") counts.waiting += 1;
    else if (glyph === "failed") counts.failed += 1;
    else if (glyph === "done" || glyph === "idle") counts.done += 1;
    else if (glyph === "stopped")
      counts[subagent.status === "canceled" ? "canceled" : "interrupted"] += 1;
    else counts.working += 1;
  }
  const parts = [
    counts.working > 0 ? `${counts.working} working` : null,
    counts.waiting > 0 ? `${counts.waiting} needs you` : null,
    counts.done > 0 ? `${counts.done} done` : null,
    counts.failed > 0 ? `${counts.failed} failed` : null,
    counts.canceled > 0 ? `${counts.canceled} canceled` : null,
    counts.interrupted > 0 ? `${counts.interrupted} interrupted` : null,
  ].filter((part): part is string => part !== null);
  const live = counts.working + counts.waiting > 0;
  return {
    label: `${subagents.length} ${plural(subagents.length, "subagent")}`,
    summary: parts.join(" · "),
    tone: counts.failed > 0 ? "failed" : live ? "live" : "settled",
    settled: !live,
  };
}

export const SUBAGENT_MODEL_NOT_REPORTED = "Not reported";

export interface SubagentRuntimeLabel {
  model: string | null;
  effort: string | null;
  fast: boolean;
}

/** Hover runtime row (UT-W08): `null` model renders `Not reported` and drops effort. */
export function subagentRuntimeLabel(subagent: SubagentView): SubagentRuntimeLabel {
  const model = subagent.runtime.model?.trim() || null;
  const effort = subagent.runtime.reasoning_effort?.trim() || null;
  return {
    model,
    effort: model === null ? null : effort,
    fast: subagent.runtime.speed === "fast",
  };
}

export interface SubagentLocationRow {
  label: "Workspace" | "Worktree";
  value: string;
}

/** Workspace/worktree rows only where the child differs from its parent (UT-W08). */
export function subagentLocationRows(
  child: SubagentLocationView | undefined,
  parent: SubagentLocationView | undefined
): SubagentLocationRow[] {
  const rows: SubagentLocationRow[] = [];
  const workspace = child?.workspace?.trim();
  if (workspace && workspace !== parent?.workspace?.trim()) {
    rows.push({ label: "Workspace", value: workspace });
  }
  const worktree = child?.worktree?.trim();
  if (worktree && worktree !== parent?.worktree?.trim()) {
    rows.push({ label: "Worktree", value: worktree });
  }
  return rows;
}

// Sidebar order: waiting > failed > working > queued > settled.
const urgency = (subagent: SubagentView): number => {
  const { glyph } = subagentDisplay(subagent);
  if (glyph === "attention") return 0;
  if (glyph === "failed") return 1;
  if (glyph === "running" || glyph === "delegated") return 2;
  if (glyph === "queued") return 3;
  return 4;
};

// Newest first by creation; a row without a creation instant (an older daemon,
// a partial fixture) falls back to its last update instead of breaking the sort.
const orderKey = (subagent: SubagentView): string =>
  subagent.created_at || subagent.updated_at || "";
const byNewest = (a: SubagentView, b: SubagentView) => orderKey(b).localeCompare(orderKey(a));

/** Most urgent first (waiting > failed > working > queued > settled), then newest. */
export function sortSubagentsByUrgency(subagents: readonly SubagentView[]): SubagentView[] {
  return [...subagents].sort((a, b) => urgency(a) - urgency(b) || byNewest(a, b));
}

/** Inspector roster split (UT-W19): failed pinned, then live (waiting first), then previous. */
export interface SubagentRosterGroups {
  failed: SubagentView[];
  live: SubagentView[];
  previous: SubagentView[];
}

export function subagentRosterGroups(subagents: readonly SubagentView[]): SubagentRosterGroups {
  // A settled child that resumed reads Working, so it rejoins the live rows.
  const ordered = [...subagents].sort(byNewest).map(subagent => ({
    subagent,
    ...subagentDisplay(subagent),
  }));
  const pick = (keep: (row: (typeof ordered)[number]) => boolean) =>
    ordered.flatMap(row => (keep(row) ? [row.subagent] : []));
  const live = (row: (typeof ordered)[number]) => !row.settled && row.glyph !== "failed";
  return {
    failed: pick(row => row.glyph === "failed"),
    live: [
      ...pick(row => row.glyph === "attention"),
      ...pick(row => live(row) && row.glyph !== "attention"),
    ],
    previous: pick(row => row.settled),
  };
}

export function subagentRosterTitle(liveCount: number): string {
  return liveCount > 0 ? `Subagents · ${liveCount} running` : "Subagents";
}

/**
 * Waiting banner rows (UT-W13): the live delegated subagents, shown only when
 * the parent has no running turn. Provider-native rows never raise it.
 */
export function subagentWaitingBannerRows(
  subagents: readonly SubagentView[],
  parentTurnRunning: boolean
): SubagentView[] {
  if (parentTurnRunning) return [];
  return subagents.filter(isSubagentStoppable);
}

const WAKE_PENDING_DELIVERIES: ReadonlySet<string> = new Set(["pending", "claimed"]);

/**
 * A settled result is queued to wake the parent (`delivery` pending or
 * claimed): the waiting banner's delegated glyph breathes (composer VC-01).
 */
export function subagentWakePending(subagents: readonly SubagentView[]): boolean {
  return subagents.some(subagent => WAKE_PENDING_DELIVERIES.has(subagent.delivery));
}

/** Per-parent counts from the rows' display state (a resumed child is live), like `subagent_summary`. */
export interface SubagentCounts {
  live: number;
  total: number;
  failed: number;
  /** Live subagents waiting for the operator. */
  attention: number;
}

export function subagentCounts(subagents: readonly SubagentView[]): SubagentCounts {
  const counts: SubagentCounts = { live: 0, total: subagents.length, failed: 0, attention: 0 };
  for (const { tone } of subagents.map(subagent => subagentDisplay(subagent))) {
    if (tone === "live" || tone === "attention") counts.live += 1;
    if (tone === "attention") counts.attention += 1;
    if (tone === "destructive") counts.failed += 1;
  }
  return counts;
}

export interface SubagentChipState {
  glyph: StateGlyphState;
  text: string;
  ariaLabel: string;
}

/**
 * Sidebar chip (UT-W17): hidden once everything settled without a failure or
 * pending attention. Glyph by urgency — attention > failed > running, with
 * `delegated` replacing running while the parent has no running turn. Text is
 * `live/total` while anything is live, else the total alone.
 */
export function subagentChipState(
  counts: SubagentCounts,
  parentTurnRunning: boolean
): SubagentChipState | null {
  const { live, total, failed, attention } = counts;
  if (total === 0 || (live === 0 && failed === 0 && attention === 0)) return null;
  const glyph: StateGlyphState =
    attention > 0
      ? "attention"
      : failed > 0
        ? "failed"
        : parentTurnRunning
          ? "running"
          : "delegated";
  if (live > 0) {
    return {
      glyph,
      text: `${live}/${total}`,
      // COPY.md "Subagent Terms" fixes this label; the attention glyph carries "needs you".
      ariaLabel: `${live} of ${total} ${plural(total, "subagent")} running`,
    };
  }
  return {
    glyph,
    text: String(total),
    ariaLabel: `${total} ${plural(total, "subagent")}, ${failed} failed`,
  };
}

export interface SubagentComposerSignal {
  /** `idle` while nothing runs; `attention` when a live subagent needs you. */
  state: "idle" | "running" | "attention";
  /** The live dot on the subagents icon, as on a subagent's avatar; `null` while nothing runs. */
  dot: SubagentDotSignal | null;
  /** Live subagents beside the icon; `null` while nothing runs. */
  count: number | null;
  ariaLabel: string;
}

/**
 * Composer control: a quiet icon until a subagent runs, then the avatar's live
 * dot (running pulses; one that needs you takes the accent) and the live count.
 * Failures stay in the popover: the control reports activity, not history.
 */
export function subagentComposerSignal(
  counts: SubagentCounts,
  failed = false
): SubagentComposerSignal {
  const { live, total, attention } = counts;
  if (live === 0) {
    // A failed read leaves the roster unknown, not empty.
    const idle = total === 0 ? "Subagents, none yet" : "Subagents, none running";
    return {
      state: "idle",
      dot: null,
      count: null,
      ariaLabel: failed ? "Subagents, couldn't load" : idle,
    };
  }
  const needsYou = attention > 0 ? `, ${attention} needs you` : "";
  return {
    state: attention > 0 ? "attention" : "running",
    dot: subagentDot(attention > 0 ? "waiting" : "running"),
    count: live,
    ariaLabel: `Subagents, ${live} running${needsYou}`,
  };
}
