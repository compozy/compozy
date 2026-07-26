import type { LoopRunEventFrame, LoopRunEventKind } from "../types";

/**
 * The SSE contract the run page consumes (techspec §observability). The stream
 * replays every retained event from `after_sequence` (default 0), so the reducer
 * below rebuilds the full run history on connect: raw structural frames feed the
 * story timeline + the Inspect drawer's Events section, while the structured
 * kinds (`gate_verdict`, `needs_approval`, `token_tick`, `goal_turn_*`) also
 * update their dedicated surface. Payloads are read through runtime guards so an
 * unknown or malformed frame degrades to a bare retained frame instead of
 * throwing.
 */

export interface LoopGateVerdict {
  nodeId: string;
  generation: number;
  verdict: "pass" | "revise";
  confidence?: number;
  reason?: string;
  route?: string;
  criteria: { id: string; type: string; status: "pass" | "revise"; note?: string }[];
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

/** The closed HITL verdict vocabulary (ADR-017 §9.8): resume / revise / halt. */
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

export interface LoopRunLiveState {
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
}

export function emptyLoopRunLiveState(): LoopRunLiveState {
  return {
    frames: [],
    gateVerdicts: {},
    needsApproval: null,
    tokensUsed: null,
    goalTurns: [],
    failure: null,
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

/**
 * Kinds excluded from frame retention: they update a dedicated slice and carry no
 * story row, so they must not consume the bounded window and evict `node_running`
 * (a goal node can emit many turns after it starts). The `Set<LoopRunEventKind>`
 * element type keeps the closed policy honest — a kind outside the union fails at
 * compile time — and the `ReadonlySet<LoopRunEventKind>` annotation carries that
 * closed type through `.has`, so membership is a compile-time closed-kind check.
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
      generation: num(payload.generation) ?? 0,
      verdict: str(payload.verdict) === "pass" ? "pass" : "revise",
      confidence: num(payload.confidence),
      reason: str(payload.reason) || undefined,
      route: str(payload.route) || undefined,
      criteria: rawCriteria.map(item => {
        const record = asRecord(item);
        return {
          id: str(record?.id),
          type: str(record?.type),
          status: str(record?.status) === "pass" ? "pass" : "revise",
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

/** Retains a structural frame in seq order, skipping reconnect-replay duplicates. */
function retainFrame(frames: LoopRunEventFrame[], frame: LoopRunEventFrame): LoopRunEventFrame[] {
  const seq = num(frame.seq) ?? 0;
  const last = frames.length > 0 ? (num(frames[frames.length - 1].seq) ?? 0) : 0;
  if (seq > 0 && seq <= last) return frames;
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
  const kind = str(frame.kind);
  const payload = asRecord(frame.payload);
  const next: LoopRunLiveState = {
    ...state,
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
    default:
      break;
  }
  return next;
}
