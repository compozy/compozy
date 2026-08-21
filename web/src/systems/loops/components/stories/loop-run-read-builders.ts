import type {
  LoopBriefing,
  LoopRosterNode,
  LoopRunGeneration,
  LoopRunRecord,
  LoopTimelineEntry,
} from "../../types";
import { STORY_NOW, STORY_RUN_ID } from "./loop-run-page-fixture-world";

/**
 * Typed builders for the three run reads, shared by every staged scenario.
 *
 * These exist to make a contract change *fail*. The register and visual-contract
 * fixtures each grew their own copy of these defaults and finished with
 * `as LoopRosterNode`, which tells TypeScript to stop checking exactly where the
 * checking mattered most: these objects are the evidence that a capture matches
 * the generated read contract. A whole-object cast means a renamed or newly
 * required field compiles here and only fails against a live daemon.
 *
 * Every timestamp is derived from `STORY_NOW` rather than written as a literal,
 * so one snapshot stays internally ordered and stays put between captures.
 */

/**
 * The briefing's `status` is an open string on the wire; the run record's is a
 * closed union. A fixture that stages both has to cross that gap deliberately —
 * an unknown briefing status keeps the run where it was rather than inventing a
 * lifecycle value the daemon does not have.
 */
const RUN_STATUSES = [
  "blocked",
  "canceled",
  "done",
  "exhausted",
  "failed",
  "needs-approval",
  "no-op",
  "paused",
  "queued",
  "running",
  "stalled",
  "watching",
] as const satisfies readonly NonNullable<LoopRunRecord["status"]>[];

export function asRunStatus(
  value: string,
  fallback: LoopRunRecord["status"]
): LoopRunRecord["status"] {
  return (RUN_STATUSES as readonly string[]).includes(value)
    ? (value as LoopRunRecord["status"])
    : fallback;
}

/** `minutesAgo` against the pinned story clock. */
export function storyAt(minutesAgo: number): string {
  return new Date(STORY_NOW - minutesAgo * 60_000).toISOString();
}

/** The roster states the fixtures stage, checked against the generated read. */
export type StoryRosterState = LoopRosterNode["state"];

export function makeRosterNode(
  nodeId: string,
  state: StoryRosterState,
  overrides: Partial<LoopRosterNode> = {}
): LoopRosterNode {
  return {
    generation: 2,
    node_id: nodeId,
    item_index: 0,
    state,
    attempt: 1,
    attempts: [],
    ...overrides,
  };
}

export function makeTimelineEntry(
  seq: number,
  kind: LoopTimelineEntry["kind"],
  title: string,
  overrides: Partial<LoopTimelineEntry> = {}
): LoopTimelineEntry {
  return {
    seq,
    kind,
    title,
    at: storyAt(2),
    generation: 2,
    ...overrides,
  };
}

export function makeBriefing(overrides: Partial<LoopBriefing> = {}): LoopBriefing {
  return {
    run_id: STORY_RUN_ID,
    status: "running",
    tone: "ok",
    headline: "Reviewing the second draft",
    detail: "Nothing needs you. Two of four steps are done in round 2.",
    blockers: [],
    artifacts: [],
    progress: { round: 2, steps_done: 2, steps_total: 4 },
    usage: { tokens: 82_400, cost_usd: 0.31, budget_used_pct: 12, duration: "9m40s" },
    ...overrides,
  };
}

export function makeGeneration(
  generation: number,
  overrides: Partial<LoopRunGeneration> = {}
): LoopRunGeneration {
  return {
    generation,
    origin: generation === 1 ? "initial" : "gate_next_generation",
    outputs: [],
    parent_generation: generation - 1,
    route_causes: [],
    verdicts: [],
    ...overrides,
  };
}
