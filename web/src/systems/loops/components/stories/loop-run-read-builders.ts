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

/** States the run has not reached, which therefore carry no timing. */
const UNREACHED_STATES = new Set<string>(["pending", "queued", "not_taken"]);
/** States the run has finished with, which carry both a start and an end. */
const SETTLED_STATES = new Set<string>([
  "succeeded",
  "partial",
  "failed",
  "canceled",
  "quarantined",
]);

export function makeRosterNode(
  nodeId: string,
  state: StoryRosterState,
  overrides: Partial<LoopRosterNode> = {}
): LoopRosterNode {
  // Timing follows the state rather than being left off. The daemon never
  // serves a succeeded node without a span, and a fixture that did made the
  // roster print "not started" — and now "unknown" — beside a settled step.
  const timing: Partial<LoopRosterNode> = UNREACHED_STATES.has(state)
    ? {}
    : SETTLED_STATES.has(state)
      ? { started_at: storyAt(14), ended_at: storyAt(10) }
      : { started_at: storyAt(3) };
  return {
    generation: 2,
    node_id: nodeId,
    item_index: 0,
    state,
    attempt: 1,
    attempts: [],
    ...timing,
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

/** The fields a briefing carries that the run record cannot supply. */
type ServerOwnedBriefingField = "run_id" | "status" | "progress" | "usage";

/**
 * The two usage facts a scenario may state, because nothing else can.
 *
 * `tokens` and `budget_used_pct` are arithmetic on the run record. `cost_usd`
 * and `duration` are computed by the daemon and have no source in the record,
 * so a scenario either supplies them or the briefing goes without — which is
 * also what a live briefing does before the daemon has priced the run.
 */
export type StoryUsageFacts = Pick<LoopBriefing["usage"], "cost_usd" | "duration">;

/**
 * What a scenario has to state for itself, and what it never gets to invent.
 *
 * The tone, the sentence, the settled outcome and what the run produced are
 * written by the daemon and have no source in the run record. A story that
 * cannot say them has not staged a verdict.
 *
 * The server-owned four are excluded rather than merely discouraged. When they
 * were reachable through a `Partial<LoopBriefing>` arm, the doc comment below
 * was the only thing standing between a story and a briefing that contradicted
 * the run beside it — and a comment does not fail a build.
 */
export type StoryVerdict = Pick<LoopBriefing, "tone" | "headline"> &
  Partial<Omit<LoopBriefing, ServerOwnedBriefingField>> & { usage?: StoryUsageFacts };

/**
 * The briefing the daemon would serve for a staged run.
 *
 * `run_id`, `status`, `progress` and token usage are server-owned fields the run
 * record already carries, so they are derived here rather than re-decided: a
 * scenario cannot end up with a briefing that contradicts the run beside it.
 * Everything else is the caller's to state, which is why `tone` and `headline`
 * are required — the placeholder verdict this replaced let a story render a
 * plausible strip for a state it had never staged.
 *
 * The verdict is spread *first* and the derived fields land last, so the
 * invariant survives a future widening of `StoryVerdict` instead of depending
 * on it.
 */
export function briefingFor(run: LoopRunRecord, verdict: StoryVerdict): LoopBriefing {
  const { usage: stated, ...rest } = verdict;
  const usage: LoopBriefing["usage"] = { ...stated, tokens: run.tokens_used };
  if (run.budget_tokens > 0) {
    usage.budget_used_pct = (run.tokens_used / run.budget_tokens) * 100;
  }
  return {
    blockers: [],
    artifacts: [],
    ...rest,
    run_id: run.id,
    status: run.status,
    progress: run.progress,
    usage,
  };
}

/**
 * A raw served briefing, for tests that project one in isolation.
 *
 * This takes no run record and therefore states no relationship to one, so it
 * is deliberately **not** a way to stage a story: a scenario built from it can
 * photograph a spend or a status its own run record contradicts, which is how
 * every register capture came to show 82.4k tokens over a run recording 68k.
 * Scenarios use `briefingFor`; this stays for unit tests that feed
 * `buildBriefingView` a payload directly and need to vary the server-owned
 * fields on purpose.
 */
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
