import type { PillTone, StateGlyphState } from "@compozy/ui";

export const LOOP_REQUEST_KINDS = ["ask", "review"] as const;
export type LoopRequestKind = (typeof LOOP_REQUEST_KINDS)[number];

export const LOOP_REQUEST_STATES = ["pending", "answered", "expired", "canceled"] as const;
export type LoopRequestState = (typeof LOOP_REQUEST_STATES)[number];

export const LOOP_REQUEST_DECISIONS = ["approve", "edit", "reject", "respond"] as const;
export type LoopRequestDecision = (typeof LOOP_REQUEST_DECISIONS)[number];

export const LOOP_DIFF_CHANGES = ["changed", "rerun", "skipped", "carried", "verdict"] as const;
export type LoopDiffChange = (typeof LOOP_DIFF_CHANGES)[number];

function member<T extends string>(values: readonly T[], value: string): value is T {
  return (values as readonly string[]).includes(value);
}

export function isLoopRequestKind(value: string): value is LoopRequestKind {
  return member(LOOP_REQUEST_KINDS, value);
}

export function isLoopRequestState(value: string): value is LoopRequestState {
  return member(LOOP_REQUEST_STATES, value);
}

export function isLoopRequestDecision(value: string): value is LoopRequestDecision {
  return member(LOOP_REQUEST_DECISIONS, value);
}

export function isLoopDiffChange(value: string): value is LoopDiffChange {
  return member(LOOP_DIFF_CHANGES, value);
}

export interface LoopSignal {
  tone: PillTone;
  /** Canonical `StateGlyph` state beside the word. */
  glyph: StateGlyphState;

  word: string;
}

export const LOOP_REQUEST_STATE_SIGNAL: Record<LoopRequestState, LoopSignal> = {
  pending: { tone: "warning", glyph: "attention", word: "pending" },
  answered: { tone: "info", glyph: "done", word: "answered" },
  expired: { tone: "danger", glyph: "stopped", word: "expired" },
  canceled: { tone: "neutral", glyph: "stopped", word: "canceled" },
};

export const LOOP_REQUEST_NEAR_EXPIRY_SIGNAL: LoopSignal = {
  tone: "warning",
  glyph: "attention",
  word: "expires soon",
};

export const LOOP_DIFF_CHANGE_TONE: Record<LoopDiffChange, PillTone> = {
  changed: "accent",
  rerun: "info",
  skipped: "neutral",
  carried: "neutral",
  verdict: "info",
};

export const LOOP_DIFF_CHANGE_LABEL: Record<LoopDiffChange, string> = {
  changed: "Changed",
  rerun: "Rerun",
  skipped: "Skipped",
  carried: "Carried",
  verdict: "Verdict",
};

export const LOOP_REQUEST_WAIT_SENTENCE: Record<LoopRequestKind, string> = {
  ask: "is waiting for an answer",
  review: "is waiting for a decision on its proposed action",
};

export const LOOP_REQUEST_KIND_TITLE: Record<LoopRequestKind, string> = {
  ask: "Answer requested",
  review: "Decision requested",
};

export const LOOP_REQUEST_DECISION_LABEL: Record<LoopRequestDecision, string> = {
  approve: "Approve",
  edit: "Edit and approve",
  reject: "Reject",
  respond: "Respond",
};
