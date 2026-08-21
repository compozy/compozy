import type { LoopRunEventFrame, LoopTimelineEntry } from "../../types";

/**
 * The timeline read a scenario's own events would produce.
 *
 * Story fixtures stage `frames` — the SSE events — richly, and left `timeline`
 * unset. The story pane reads the durable timeline, not the frames, so a run
 * that had plainly reached round 5 and spent 21k tokens rendered "Nothing has
 * happened in this run yet." Several visual-contract rows captured that empty
 * pane as their evidence.
 *
 * Nothing here is invented. The daemon builds the timeline from exactly these
 * events at read time (`internal/loop/timeline.go`), so this mirrors its two
 * rules: `timelineTitle` for the sentence, and newest-sequence-first for the
 * order. Where the daemon has no title for a kind it errors; here the kind is
 * skipped, so a scenario can stage a frame this mapping does not know without
 * the pane inventing a sentence for it.
 */

/** `Step <node> <state>` — the daemon's phrasing for the node lifecycle kinds. */
const NODE_STATE_KINDS: Record<string, string> = {
  node_running: "running",
  node_succeeded: "succeeded",
  node_failed: "failed",
  node_paused: "paused",
  node_resumed: "resumed",
  node_canceled: "canceled",
  node_killed: "killed",
  node_quarantined: "quarantined",
  node_requeued: "requeued",
};

/** Kinds whose sentence takes no payload at all. */
const FIXED_TITLES: Record<string, string> = {
  channel_msg: "An agent message was recorded",
  token_tick: "Token usage increased",
  runtime_applied: "Runtime settings were applied",
  predicate_diagnostic: "A route condition was evaluated",
  route_taken: "The run chose a route",
  node_wait_started: "A step started waiting",
  node_wait_resumed: "A waiting step resumed",
  node_attention_flagged: "A step needs attention",
  node_attention_cleared: "A step no longer needs attention",
  effect_results: "Run effects finished",
  custom_event: "Loop activity was recorded",
  duplicate_suppressed: "A duplicate update was ignored",
  request_opened: "A request is waiting",
  request_answered: "A request was answered",
  request_expired: "A request expired",
  request_canceled: "A request was canceled",
  node_amended: "A step result was amended",
  branch_pruned: "An unused branch was skipped",
};

interface FramePayload {
  node_id?: string;
  gate_id?: string;
  generation?: number;
  attempt?: number;
  status?: string;
  verdict?: string;
  to?: string;
}

function payloadOf(frame: LoopRunEventFrame): FramePayload {
  const payload: unknown = (frame as { payload?: unknown }).payload;
  return payload && typeof payload === "object" ? (payload as FramePayload) : {};
}

/** `timelineTitle`, for the kinds the story fixtures stage. */
function titleFor(kind: string, payload: FramePayload): string | null {
  const state = NODE_STATE_KINDS[kind];
  if (state) {
    return payload.node_id ? `Step ${payload.node_id} ${state}` : "A step changed state";
  }
  if (kind === "node_retry_scheduled") {
    return payload.node_id ? `Step ${payload.node_id} will retry` : "A step will retry";
  }
  if (kind === "needs_approval") {
    return payload.gate_id ? `Approval "${payload.gate_id}" is waiting` : "An approval is waiting";
  }
  if (kind === "gate_verdict") {
    return payload.gate_id
      ? `Approval "${payload.gate_id}": ${(payload.verdict ?? "").trim()}`
      : "An approval was decided";
  }
  if (kind === "status_changed") {
    // The daemon reads `status`; the fixtures also carry the same value as `to`.
    const status = payload.status ?? payload.to;
    return status ? `Run is now ${status}` : "Run status changed";
  }
  if (kind === "generation_started") {
    const round = payload.generation;
    return typeof round === "number" && round > 0
      ? `Round ${round} started`
      : "A new round started";
  }
  return FIXED_TITLES[kind] ?? null;
}

/**
 * Newest first, exactly as the timeline read serves it.
 *
 * `seq` comes from the frame, so a scenario's own ordering carries through and
 * the story's de-duplication by sequence keeps working.
 */
export function timelineFromFrames(frames: readonly LoopRunEventFrame[]): LoopTimelineEntry[] {
  const entries: LoopTimelineEntry[] = [];
  for (const frame of frames) {
    const payload = payloadOf(frame);
    const title = titleFor(frame.kind, payload);
    if (title === null) continue;
    entries.push({
      seq: frame.seq,
      first_seq: frame.seq,
      kind: frame.kind,
      title,
      at: frame.at,
      ...(typeof payload.generation === "number" ? { generation: payload.generation } : {}),
      ...(payload.node_id ? { node_id: payload.node_id } : {}),
      ...(typeof payload.attempt === "number" ? { attempt: payload.attempt } : {}),
    });
  }
  return entries.sort((left, right) => right.seq - left.seq);
}
