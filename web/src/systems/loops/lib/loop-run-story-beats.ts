import type { PillTone } from "@compozy/ui";

import type { LoopTimelineEntry } from "../types";
import { loopAttemptLabel } from "./loop-run-fanout-band";
import type { LoopStoryIcon } from "./loop-story-icons";

/**
 * The story, told as beats.
 *
 * Titles arrive already written: the daemon renders each timeline entry into
 * plain English at read time, so the same sentence reaches the page, the CLI and
 * an agent. What this model adds is register — a tone and a glyph per kind — and
 * the arithmetic of coalescing.
 *
 * The one hard rule, in the narrated default register: an event kind is never
 * user-visible text. `node_failed` is a wire value; "the style reviewer failed"
 * is a beat. If a title ever arrives empty, the fallback below writes a sentence
 * rather than printing the enum. The disclosed raw-events lane is the documented
 * exception — it renders `kind` on purpose, so an operator can line the timeline
 * up against the CLI or the logs (`_uiux.md` S5).
 */

export interface LoopStoryBeat {
  key: string;
  /**
   * The wire kind. Never rendered as text in the narrated register; the
   * disclosed raw-events lane shows it deliberately, for CLI/log correlation.
   */
  kind: string;
  seq: number;
  /** Earliest raw sequence this beat covers; equals `seq` unless coalesced. */
  firstSeq: number;
  at: string;
  tone: PillTone;
  icon: LoopStoryIcon;
  title: string;
  /** Raw events folded into this beat. Greater than 1 only for chatter runs. */
  count: number;
  nodeId: string | null;
  generation: number | null;
  /** Attempts are metadata on the beat, never a beat of their own. */
  attemptLabel: string | null;
  /**
   * The run on the other side of a fork, when this beat is one (US-009.EC-3).
   *
   * The timeline entry does not carry it — the daemon records the branch on the
   * run itself — so it is resolved from the run's own `forked_from` / `forks`
   * lineage by matching the round the beat happened in. Null whenever that match
   * cannot be made, because a fork beat pointing at a guessed run is worse than
   * one that only names the fork.
   */
  relatedRunId: string | null;
}

/** A run's fork lineage, exactly as the run projection carries it. */
export interface LoopRunLineage {
  forkedFrom: { run_id: string; generation: number } | null;
  forks: readonly { run_id: string; generation: number }[];
}

interface BeatRegister {
  tone: PillTone;
  icon: LoopStoryIcon;
}

/**
 * Every kind the timeline can carry, classified once.
 *
 * Exhaustive over the generated event-kind union, and typed to stay that way: a
 * kind added to the contract fails typecheck here until its tone and glyph have
 * been chosen. The runtime fallback below stays regardless, for a daemon newer
 * than the client — that is a forward-compatibility question, not a licence to
 * leave a known kind unclassified.
 */
const BEAT_REGISTER: Record<LoopTimelineEntry["kind"], BeatRegister> = {
  // Lifecycle
  generation_started: { tone: "neutral", icon: "round" },
  status_changed: { tone: "neutral", icon: "started" },
  node_running: { tone: "accent", icon: "started" },
  node_succeeded: { tone: "success", icon: "node-done" },
  node_failed: { tone: "danger", icon: "node-failed" },
  node_retry_scheduled: { tone: "warning", icon: "retry" },
  node_paused: { tone: "warning", icon: "paused" },
  node_resumed: { tone: "info", icon: "resumed" },
  node_canceled: { tone: "neutral", icon: "canceled" },
  node_quarantined: { tone: "danger", icon: "quarantined" },
  node_requeued: { tone: "info", icon: "requeued" },
  node_amended: { tone: "info", icon: "amended" },
  // Waiting on people or on the world
  needs_approval: { tone: "warning", icon: "approval" },
  request_opened: { tone: "warning", icon: "request-opened" },
  request_answered: { tone: "info", icon: "request-answered" },
  request_expired: { tone: "danger", icon: "request-expired" },
  request_canceled: { tone: "neutral", icon: "canceled" },
  node_wait_started: { tone: "warning", icon: "waiting" },
  node_wait_resumed: { tone: "info", icon: "resumed" },
  node_attention_flagged: { tone: "warning", icon: "attention" },
  node_attention_cleared: { tone: "info", icon: "attention-cleared" },
  // Shape
  gate_verdict: { tone: "info", icon: "check-pass" },
  route_taken: { tone: "neutral", icon: "route-taken" },
  branch_pruned: { tone: "neutral", icon: "pruned" },
  run_forked: { tone: "info", icon: "forked" },
  // Goals
  goal_turn_started: { tone: "neutral", icon: "started" },
  goal_turn_completed: { tone: "neutral", icon: "done" },
  goal_status_changed: { tone: "neutral", icon: "started" },
  // Machinery — chatter tier; present under `all`, coalesced by the daemon.
  token_tick: { tone: "neutral", icon: "effect" },
  channel_msg: { tone: "neutral", icon: "effect" },
  runtime_applied: { tone: "neutral", icon: "effect" },
  predicate_diagnostic: { tone: "neutral", icon: "check-warn" },
  effect_results: { tone: "neutral", icon: "effect" },
  custom_event: { tone: "neutral", icon: "effect" },
  duplicate_suppressed: { tone: "neutral", icon: "suppressed" },
  target_breaker_transition: { tone: "warning", icon: "breaker-open" },
  stale_schedule_dropped: { tone: "neutral", icon: "suppressed" },
  late_arrival: { tone: "neutral", icon: "suppressed" },
};

const DEFAULT_REGISTER: BeatRegister = { tone: "neutral", icon: "started" };

function beatRegister(kind: string): BeatRegister {
  return (BEAT_REGISTER as Record<string, BeatRegister>)[kind] ?? DEFAULT_REGISTER;
}

/**
 * Sentences for the case where a title never arrived. These are deliberately
 * vague rather than mechanical: saying "a step finished" without naming which is
 * honest, where printing `node_succeeded` would be a leak.
 */
const FALLBACK_TITLES: Record<string, string> = {
  generation_started: "A new round started",
  status_changed: "The run changed state",
  node_running: "A step started",
  node_succeeded: "A step finished",
  node_failed: "A step failed",
  node_retry_scheduled: "A step will try again",
  node_paused: "A step was paused",
  node_resumed: "A step resumed",
  node_canceled: "A step was canceled",
  node_quarantined: "A step was quarantined",
  node_requeued: "A step was queued again",
  node_amended: "A step was amended",
  needs_approval: "An approval opened",
  request_opened: "A question was asked",
  request_answered: "A question was answered",
  request_expired: "A question expired",
  request_canceled: "A question was withdrawn",
  node_wait_started: "A step started waiting",
  node_wait_resumed: "A step stopped waiting",
  node_attention_flagged: "A step was flagged for attention",
  node_attention_cleared: "A step no longer needs attention",
  gate_verdict: "A gate reached a verdict",
  route_taken: "The run chose a route",
  branch_pruned: "A branch was dropped",
  run_forked: "The run was forked",
};

function fallbackTitle(kind: string): string {
  return FALLBACK_TITLES[kind] ?? "Something happened in this run";
}

/**
 * Server-side coalescing folds a run of heartbeat-class events into one entry
 * spanning `first_seq..seq`. The count is the span, so resuming after the beat
 * never replays what it folded.
 */
function beatCount(entry: LoopTimelineEntry): number {
  const first = entry.first_seq;
  if (typeof first !== "number" || first >= entry.seq) return 1;
  return entry.seq - first + 1;
}

/**
 * Which run a fork beat points at, from the run's own lineage.
 *
 * A fork beat in round R is either the point this run was forked *from* — the
 * parent recorded that same round — or the point a child was forked *to*. Both
 * sides are server-owned; the round is the only thing that ties a beat to one of
 * them, so an unmatched beat resolves to nothing rather than to the first fork
 * in the list.
 */
function relatedForkRunId(
  entry: LoopTimelineEntry,
  lineage: LoopRunLineage | undefined
): string | null {
  if (entry.kind !== "run_forked" || !lineage) return null;
  const generation = typeof entry.generation === "number" ? entry.generation : null;
  if (generation === null) return null;
  if (lineage.forkedFrom?.generation === generation) return lineage.forkedFrom.run_id;
  return lineage.forks.find(fork => fork.generation === generation)?.run_id ?? null;
}

export function buildStoryBeat(entry: LoopTimelineEntry, lineage?: LoopRunLineage): LoopStoryBeat {
  const register = beatRegister(entry.kind);
  const title = entry.title?.trim();
  const attempt = entry.attempt;
  return {
    key: `${entry.seq}`,
    kind: entry.kind,
    seq: entry.seq,
    firstSeq: typeof entry.first_seq === "number" ? entry.first_seq : entry.seq,
    at: entry.at,
    tone: register.tone,
    icon: register.icon,
    title: title ? title : fallbackTitle(entry.kind),
    count: beatCount(entry),
    nodeId: entry.node_id ?? null,
    generation: typeof entry.generation === "number" ? entry.generation : null,
    attemptLabel: typeof attempt === "number" ? loopAttemptLabel(attempt) : null,
    relatedRunId: relatedForkRunId(entry, lineage),
  };
}

/**
 * Beats in display order, newest first, one per sequence.
 *
 * De-duplication is by `seq` because that is the only identity the run
 * guarantees: the durable pages and the live stream overlap by design at the
 * seam, and the same event must not appear twice because it arrived twice.
 */
export function buildStoryBeats(
  entries: readonly LoopTimelineEntry[],
  lineage?: LoopRunLineage
): LoopStoryBeat[] {
  const bySeq = new Map<number, LoopStoryBeat>();
  for (const entry of entries) {
    const beat = buildStoryBeat(entry, lineage);
    // Later writers win: a live frame is fresher than the page that preceded it.
    bySeq.set(beat.seq, beat);
  }
  return [...bySeq.values()].sort((left, right) => right.seq - left.seq);
}
