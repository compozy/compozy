import type { LucideIcon } from "lucide-react";
import {
  Archive,
  Check,
  CircleAlert,
  CircleDashed,
  Loader,
  Minus,
  Pause,
  Radio,
  TriangleAlert,
} from "lucide-react";

import type { LoopBriefing, LoopBriefingBlocker } from "../types";

/**
 * The briefing strip: the run's verdict, as the daemon decided it.
 *
 * The cascade that picks a tone and writes the headline runs server-side, over
 * the same reads this page renders. Nothing here re-decides it (Safety Invariant
 * 12) — this model chooses a glyph, a weight, and whether there is somewhere to
 * point, and leaves the sentence exactly as it arrived.
 */

export type LoopBriefingTone = "ok" | "needs_you" | "degraded" | "failed";

/** How loudly the strip sits on the page. Only a failure earns the danger fill. */
export type LoopBriefingWeight = "calm" | "lead" | "danger";

export interface LoopBriefingBlockerView {
  key: string;
  kind: string;
  /** What is being asked, in plain words. */
  label: string;
  nodeId: string | null;
  gateId: string | null;
  waitingSince: string;
  expiresAt: string | null;
  expired: boolean;
  /** The exact command that clears this blocker, for the operator register. */
  unblocker: string;
}

export interface LoopBriefingAction {
  label: string;
  /**
   * Where the strip points. It never carries the decision itself: the needs-you
   * card owns Approve/Reject and is the only primary on the page. Two primaries
   * for one decision in one viewport is a defect, not a convenience.
   */
  target: "needs-you" | "inspect";
}

export interface LoopBriefingView {
  tone: LoopBriefingTone;
  weight: LoopBriefingWeight;
  icon: LucideIcon;
  /** Server-authored. Rendered verbatim. */
  headline: string;
  detail: string | null;
  action: LoopBriefingAction | null;
  blockers: LoopBriefingBlockerView[];
  blockerCount: number;
}

const BRIEFING_TONES = new Set<LoopBriefingTone>(["ok", "needs_you", "degraded", "failed"]);

function readTone(tone: string): LoopBriefingTone {
  return BRIEFING_TONES.has(tone as LoopBriefingTone) ? (tone as LoopBriefingTone) : "ok";
}

/**
 * The glyph follows the run's own state, so a watching run reads as watching
 * rather than as a generic "in progress".
 */
const STATUS_ICONS: Record<string, LucideIcon> = {
  queued: CircleDashed,
  running: Loader,
  watching: Radio,
  paused: Pause,
  "needs-approval": TriangleAlert,
  done: Check,
  "no-op": Minus,
  canceled: Minus,
  blocked: TriangleAlert,
  failed: CircleAlert,
  exhausted: TriangleAlert,
  stalled: TriangleAlert,
};

function briefingIcon(briefing: LoopBriefing, tone: LoopBriefingTone): LucideIcon {
  if (tone === "needs_you") return TriangleAlert;
  if (tone === "failed") return CircleAlert;
  // A finished run whose output survived only as a name says so with the archive
  // glyph rather than a clean check it has not quite earned.
  if (briefing.status === "done" && briefing.artifacts.some(a => a.availability === "pruned")) {
    return Archive;
  }
  return STATUS_ICONS[briefing.status] ?? Loader;
}

function briefingWeight(tone: LoopBriefingTone): LoopBriefingWeight {
  if (tone === "failed") return "danger";
  // Needs-you inks warning-family on the page; danger stays with failure and the
  // attention bell. Two danger vocabularies on one surface would flatten both.
  if (tone === "needs_you" || tone === "degraded") return "lead";
  return "calm";
}

/**
 * Blocker kinds in plain words. The daemon's cascade orders them
 * approval > quarantine > request > failure > backoff/quota, so the first entry
 * is always the one the headline is about.
 */
const BLOCKER_LABELS: Record<string, string> = {
  approval: "An approval is waiting for you",
  quarantine: "A step is quarantined and needs a decision",
  request: "A question is waiting for your answer",
  failure: "A step failed",
  backoff: "Waiting to retry",
  quota: "Waiting on capacity",
};

function blockerLabel(kind: string): string {
  return BLOCKER_LABELS[kind] ?? "Waiting on you";
}

function blockerKey(blocker: LoopBriefingBlocker, index: number): string {
  return [blocker.kind, blocker.gate_id ?? "", blocker.node_id ?? "", index].join(":");
}

function projectBlocker(blocker: LoopBriefingBlocker, index: number): LoopBriefingBlockerView {
  return {
    key: blockerKey(blocker, index),
    kind: blocker.kind,
    label: blockerLabel(blocker.kind),
    nodeId: blocker.node_id ?? null,
    gateId: blocker.gate_id ?? null,
    waitingSince: blocker.waiting_since,
    expiresAt: blocker.expires_at ?? null,
    expired: blocker.expired === true,
    unblocker: blocker.unblocker,
  };
}

/** Blockers a person can answer in place, as opposed to ones they can only watch. */
const ANSWERABLE_KINDS = new Set(["approval", "quarantine", "request"]);

function briefingAction(
  tone: LoopBriefingTone,
  blockers: readonly LoopBriefingBlockerView[]
): LoopBriefingAction | null {
  if (blockers.some(blocker => ANSWERABLE_KINDS.has(blocker.kind))) {
    // Quiet, and it only leads: the card below owns the decision.
    return { label: "Review the request", target: "needs-you" };
  }
  if (tone === "failed") {
    // No invented control. The register is where node verbs live, and the
    // daemon decides there which of them this node will actually accept.
    return { label: "Open the failed step", target: "inspect" };
  }
  return null;
}

export function buildBriefingView(briefing: LoopBriefing): LoopBriefingView {
  const tone = readTone(briefing.tone);
  const blockers = briefing.blockers.map(projectBlocker);
  const detail = briefing.detail?.trim();
  return {
    tone,
    weight: briefingWeight(tone),
    icon: briefingIcon(briefing, tone),
    headline: briefing.headline,
    detail: detail ? detail : null,
    action: briefingAction(tone, blockers),
    blockers,
    blockerCount: blockers.length,
  };
}
