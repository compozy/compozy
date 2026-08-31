import type { GoalTurn, LoopRunEventFrame, LoopRunEventKind } from "../types";

export interface LoopGateVerdict {
  nodeId: string;
  gateId: string;
  generation: number;
  verdict: "pass" | "revise";
  score?: number;
  bestGeneration?: number;
  reason?: string;
  route?: string;
  criteria: {
    id: string;
    type: string;
    status: "pass" | "revise";
    score?: number;
    note?: string;
  }[];
  blockingIssues: { id: string; note?: string }[];
}

export interface LoopApprovalFact {
  label: string;
  value: string;
}

export interface LoopApprovalRequest {
  gateId: string;
  title: string;
  prompt?: string;
  facts: LoopApprovalFact[];
}

export type LoopGateDecision = "approve" | "request_changes" | "reject";

export interface LoopGoalTurnLive {
  seq: number;
  generation: number;
  nodeId: string;
  itemIndex: number;
  turn: number;
  promptAttempt: number;
  promptId: string;
  sessionId: string;
  bindingHandle: string;
  bindingEpoch: number;
  actorKind: string;
  actorId: string;
  resultStatus: string | null;
  stopReason: string | null;
  reasonCode: string | null;
  verdictOutcome: string | null;
  blockingIssues: { id: string; note: string }[];
  criteria: GoalTurn["criteria"];
  warnings: GoalTurn["warnings"];
  evidenceRef: string | null;
  tokensUsed: number | null;
  startedAt: string;
  endedAt: string | null;
}

export interface LoopCoordinatorFailure {
  kind: "coordinator_failure";
  code: string;
  cause: string;
  recovery: string;
}

/**
 * The streamed half of node lifecycle truth. The durable half lives in the run
 * detail (`node_controls[]` / `waits[]`); this slice carries what only the stream
 * knows between two polls — the newest retry schedule per node and the effect
 * deliveries, which have no durable projection on the run response at all.
 */
export interface LoopNodeRetrySchedule {
  nodeId: string;
  itemIndex?: number;
  generation: number;
  attempt: number;
  nextAttemptAt: string | null;
  failureClass: string;
}

/** One acknowledged effect delivery (`effect_results`). */
export interface LoopEffectResult {
  deliveryId: string;
  trigger: string;
  nodeId: string;
  outcome: string;
  code: string;
  cause: string;
  durationMs: number | null;
  at: string;
}

export interface LoopRunLiveState {
  /** Highest positive stream sequence applied to any live slice. */
  lastSequence: number;
  /**
   * Retained structural frames in arrival (seq) order — the single source for
   * the story timeline and the Inspect drawer's raw Events section. The
   * high-frequency display kinds (`token_tick`, `channel_msg`) are excluded so
   * they can never evict structural history; their data lands in `tokensUsed`
   * and the network surface respectively.
   */
  frames: LoopRunEventFrame[];
  /** Latest gate verdict keyed by `nodeId` (a re-run overwrites the prior verdict). */
  gateVerdicts: Record<string, LoopGateVerdict>;
  needsApproval: LoopApprovalRequest | null;
  /** Latest token count from `token_tick`, overlaid on the polled run when fresher. */
  tokensUsed: number | null;
  goalTurns: LoopGoalTurnLive[];
  /** Run-level failure projected from a terminal status event before any node exists. */
  failure: LoopCoordinatorFailure | null;
  /**
   * Newest retry schedule per node id. A `node_retry_scheduled` frame lands
   * before the run detail refetch carries `next_attempt_at`, so the run page can
   * name the attempt and its due time without waiting a poll interval.
   */
  retrySchedules: Record<string, LoopNodeRetrySchedule>;
  /** Effect deliveries in arrival order, bounded like the frame ring. */
  effectResults: LoopEffectResult[];
}

export function emptyLoopRunLiveState(): LoopRunLiveState {
  return {
    lastSequence: 0,
    frames: [],
    gateVerdicts: {},
    needsApproval: null,
    tokensUsed: null,
    goalTurns: [],
    failure: null,
    retrySchedules: {},
    effectResults: [],
  };
}

/** Bounds retained structural frames; a run past this keeps its newest history. */
const MAX_STORY_FRAMES = 500;

/**
 * Bounds the live goal-turn replay slice. A goal node can emit an unbounded
 * stream of turns; without this cap the automatic SSE state would grow without
 * limit on a long run. Explicitly paged durable turns live in the query cache
 * and are merged separately, so this only trims the auto-accumulated tail.
 */
const MAX_GOAL_TURNS = 500;

/** Bounds the retained effect-delivery slice on a long-running, chatty run. */
const MAX_EFFECT_RESULTS = 200;

/**
 * Kinds excluded from frame retention: they update a dedicated slice and carry no
 * story row, so they must not consume the bounded window and evict `node_running`
 * (a goal node can emit many turns after it starts). The `Set<LoopRunEventKind>`
 * element type keeps the closed policy honest — a kind outside the union fails at
 * compile time — and the `ReadonlySet<LoopRunEventKind>` annotation carries that
 * closed type through `.has`, so membership is a compile-time closed-kind check.
 *
 * Every one of the 15 new lifecycle kinds IS retained: each one is a story beat
 * an operator has to be able to read back after a reconnect. The two new
 * stream-only kinds (`stale_schedule_dropped`, `late_arrival`) are retained too —
 * they build no story row, but they are rare, and the Inspect drawer's raw Events
 * section is the only place a dropped schedule is ever visible.
 */
const UNRETAINED_KINDS: ReadonlySet<LoopRunEventKind> = new Set<LoopRunEventKind>([
  "token_tick",
  "channel_msg",
  "goal_turn_started",
  "goal_turn_completed",
]);

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : null;
}

function str(value: unknown, fallback = ""): string {
  return typeof value === "string" ? value : fallback;
}

function num(value: unknown): number | undefined {
  return typeof value === "number" && Number.isFinite(value) ? value : undefined;
}

function parseCoordinatorFailure(value: unknown): LoopCoordinatorFailure | null {
  const failure = asRecord(value);
  if (!failure || failure.kind !== "coordinator_failure") return null;
  const code = str(failure.code).trim();
  const cause = str(failure.cause).trim();
  const recovery = str(failure.recovery).trim();
  if (!code || !cause || !recovery) return null;
  return { kind: "coordinator_failure", code, cause, recovery };
}

function applyGateVerdict(
  verdicts: Record<string, LoopGateVerdict>,
  payload: Record<string, unknown>
): Record<string, LoopGateVerdict> {
  const nodeId = str(payload.node_id);
  if (nodeId === "") return verdicts;
  const rawCriteria = Array.isArray(payload.criteria) ? payload.criteria : [];
  const rawIssues = Array.isArray(payload.blocking_issues) ? payload.blocking_issues : [];
  return {
    ...verdicts,
    [nodeId]: {
      nodeId,
      gateId: str(payload.gate_id),
      generation: num(payload.generation) ?? 0,
      verdict: str(payload.verdict) === "pass" ? "pass" : "revise",
      score: num(payload.score),
      bestGeneration: num(payload.best_generation),
      reason: str(payload.reason) || undefined,
      route: str(payload.route) || undefined,
      criteria: rawCriteria.map(item => {
        const record = asRecord(item);
        return {
          id: str(record?.id),
          type: str(record?.type),
          status: str(record?.status) === "pass" ? "pass" : "revise",
          score: num(record?.score),
          note: str(record?.note) || undefined,
        };
      }),
      blockingIssues: rawIssues.map(item => {
        const record = asRecord(item);
        return { id: str(record?.id), note: str(record?.note) || undefined };
      }),
    },
  };
}

function parseApproval(payload: Record<string, unknown>): LoopApprovalRequest {
  const rawFacts = Array.isArray(payload.facts) ? payload.facts : [];
  return {
    gateId: str(payload.gate_id, "approve"),
    title: str(payload.title, "Approve to resume"),
    prompt: str(payload.prompt) || undefined,
    facts: rawFacts.map(item => {
      const record = asRecord(item);
      return { label: str(record?.label), value: str(record?.value) };
    }),
  };
}

function blockingIssues(value: unknown): { id: string; note: string }[] {
  if (!Array.isArray(value)) return [];
  return value.map(item => {
    const record = asRecord(item);
    return { id: str(record?.id), note: str(record?.note) };
  });
}

function goalCriterionOutcome(value: unknown): GoalTurn["criteria"][number]["outcome"] | null {
  switch (value) {
    case "approved":
    case "rejected":
    case "awaiting_approval":
    case "blocked":
    case "error":
    case "timeout":
    case "invalid_output":
      return value;
    default:
      return null;
  }
}

function diagnosticWarnings(value: unknown): GoalTurn["warnings"] {
  if (!Array.isArray(value)) return [];
  return value.flatMap(item => {
    const record = asRecord(item);
    const code = str(record?.code).trim();
    const message = str(record?.message).trim();
    return code && message ? [{ code, message }] : [];
  });
}

function goalCriteria(value: unknown): GoalTurn["criteria"] {
  if (!Array.isArray(value)) return [];
  return value.flatMap(item => {
    const record = asRecord(item);
    const id = str(record?.id).trim();
    const type = str(record?.type).trim();
    const outcome = goalCriterionOutcome(record?.outcome);
    const passed = record?.passed;
    if (!id || !type || !outcome || typeof passed !== "boolean") return [];

    const criterion: GoalTurn["criteria"][number] = {
      id,
      type,
      outcome,
      passed,
    };
    if (typeof record?.exit_code === "number" || record?.exit_code === null) {
      criterion.exit_code = record.exit_code;
    }
    if (typeof record?.score === "number" || record?.score === null) {
      criterion.score = record.score;
    }
    if (typeof record?.stdout === "string") criterion.stdout = record.stdout;
    if (typeof record?.stderr === "string") criterion.stderr = record.stderr;
    if (typeof record?.broken === "boolean") criterion.broken = record.broken;
    if (record && Object.prototype.hasOwnProperty.call(record, "evidence")) {
      criterion.evidence = record.evidence;
    }
    if (record && Object.prototype.hasOwnProperty.call(record, "payload")) {
      criterion.payload = record.payload;
    }
    const issues = blockingIssues(record?.blocking_issues);
    if (issues.length > 0) criterion.blocking_issues = issues;
    const warnings = diagnosticWarnings(record?.warnings);
    if (warnings.length > 0) criterion.warnings = warnings;
    return [criterion];
  });
}

function applyGoalTurn(
  turns: LoopGoalTurnLive[],
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>,
  completed: boolean
): LoopGoalTurnLive[] {
  const promptId = str(payload.prompt_id);
  if (!promptId) return turns;
  const index = turns.findIndex(turn => turn.promptId === promptId);
  const previous = index >= 0 ? turns[index] : undefined;
  const next: LoopGoalTurnLive = {
    seq: num(payload.seq) ?? previous?.seq ?? num(frame.seq) ?? 0,
    generation: num(payload.generation) ?? previous?.generation ?? 0,
    nodeId: str(payload.node_id, previous?.nodeId),
    itemIndex: num(payload.item_index) ?? previous?.itemIndex ?? 0,
    turn: num(payload.turn) ?? previous?.turn ?? 0,
    promptAttempt: num(payload.prompt_attempt) ?? previous?.promptAttempt ?? 0,
    promptId,
    sessionId: str(payload.session_id, previous?.sessionId),
    bindingHandle: str(payload.binding_handle, previous?.bindingHandle),
    bindingEpoch: num(payload.binding_epoch) ?? previous?.bindingEpoch ?? 0,
    actorKind: str(payload.actor_kind, previous?.actorKind),
    actorId: str(payload.actor_id, previous?.actorId),
    resultStatus: completed ? str(payload.result_status) || null : null,
    stopReason: completed ? str(payload.stop_reason) || null : null,
    reasonCode: completed ? str(payload.reason_code) || null : null,
    verdictOutcome: completed ? str(payload.verdict_outcome) || null : null,
    blockingIssues: completed ? blockingIssues(payload.blocking_issues) : [],
    criteria: completed ? goalCriteria(payload.criteria) : [],
    warnings: completed ? diagnosticWarnings(payload.warnings) : [],
    evidenceRef: completed ? str(payload.evidence_ref) || null : null,
    tokensUsed: completed ? (num(payload.tokens_used) ?? null) : null,
    startedAt: previous?.startedAt || str(frame.at),
    endedAt: completed ? str(frame.at) || null : null,
  };
  // Append trims to the newest window; an in-place update keeps the length, so it
  // never grows and needs no trim.
  if (index < 0) return [...turns, next].slice(-MAX_GOAL_TURNS);
  return turns.map((turn, turnIndex) => (turnIndex === index ? next : turn));
}

function parseRetrySchedule(payload: Record<string, unknown>): LoopNodeRetrySchedule | null {
  const nodeId = str(payload.node_id);
  if (nodeId === "") return null;
  return {
    nodeId,
    itemIndex: num(payload.item_index),
    generation: num(payload.generation) ?? 0,
    attempt: num(payload.attempt) ?? 0,
    nextAttemptAt: str(payload.next_attempt_at) || null,
    failureClass: str(payload.failure_class),
  };
}

function parseEffectResult(
  frame: LoopRunEventFrame,
  payload: Record<string, unknown>
): LoopEffectResult | null {
  const deliveryId = str(payload.delivery_id);
  if (deliveryId === "") return null;
  return {
    deliveryId,
    trigger: str(payload.trigger),
    nodeId: str(payload.node_id),
    outcome: str(payload.outcome),
    code: str(payload.code),
    cause: str(payload.cause),
    durationMs: num(payload.duration_ms) ?? null,
    at: str(frame.at),
  };
}

/** Retains a structural frame in seq order after the global sequence fence accepts it. */
function retainFrame(frames: LoopRunEventFrame[], frame: LoopRunEventFrame): LoopRunEventFrame[] {
  return [...frames, frame].slice(-MAX_STORY_FRAMES);
}

/**
 * Folds one SSE frame into the run-page live state: structural frames are
 * retained in order (bounded), and the structured kinds also update their
 * dedicated slice. Malformed payloads degrade to the retained frame only.
 */
export function applyLoopEventFrame(
  state: LoopRunLiveState,
  frame: LoopRunEventFrame
): LoopRunLiveState {
  const sequence = num(frame.seq) ?? 0;
  if (sequence > 0 && sequence <= state.lastSequence) return state;
  const kind = str(frame.kind);
  const payload = asRecord(frame.payload);
  const next: LoopRunLiveState = {
    ...state,
    lastSequence: sequence > 0 ? sequence : state.lastSequence,
    // kind is a runtime-guarded SSE string; membership is tested against the
    // closed LoopRunEventKind policy set.
    frames: UNRETAINED_KINDS.has(kind as LoopRunEventKind)
      ? state.frames
      : retainFrame(state.frames, frame),
  };
  if (!payload) return next;
  switch (kind) {
    case "gate_verdict":
      next.gateVerdicts = applyGateVerdict(state.gateVerdicts, payload);
      break;
    case "needs_approval":
      next.needsApproval = parseApproval(payload);
      break;
    case "token_tick": {
      const total = num(payload.tokens_used);
      if (total !== undefined) next.tokensUsed = total;
      break;
    }
    case "goal_turn_started":
      next.goalTurns = applyGoalTurn(state.goalTurns, frame, payload, false);
      break;
    case "goal_turn_completed":
      next.goalTurns = applyGoalTurn(state.goalTurns, frame, payload, true);
      break;
    case "status_changed": {
      const failure = parseCoordinatorFailure(payload.failure);
      if (failure) next.failure = failure;
      break;
    }
    case "node_retry_scheduled": {
      const schedule = parseRetrySchedule(payload);
      if (schedule) {
        next.retrySchedules = { ...state.retrySchedules, [schedule.nodeId]: schedule };
      }
      break;
    }
    // A node that resumes, gets canceled, quarantines, or requeues is no
    // longer waiting on a backoff — drop the stale schedule so the run page stops
    // counting down to an attempt that will never run.
    case "node_resumed":
    case "node_canceled":
    case "node_quarantined":
    case "node_requeued":
    case "node_succeeded": {
      const nodeId = str(payload.node_id);
      if (nodeId !== "" && state.retrySchedules[nodeId] !== undefined) {
        next.retrySchedules = Object.fromEntries(
          Object.entries(state.retrySchedules).filter(([key]) => key !== nodeId)
        );
      }
      break;
    }
    case "effect_results": {
      const result = parseEffectResult(frame, payload);
      if (result && !state.effectResults.some(item => item.deliveryId === result.deliveryId)) {
        next.effectResults = [...state.effectResults, result].slice(-MAX_EFFECT_RESULTS);
      }
      break;
    }
    // The remaining lifecycle kinds — node_paused, node_wait_started,
    // node_wait_resumed, node_attention_flagged, node_attention_cleared,
    // custom_event, duplicate_suppressed, target_breaker_transition — carry no
    // slice of their own: they are pure story beats, already retained above, and
    // their durable state arrives on the run detail's controls/waits.
    default:
      break;
  }
  return next;
}
