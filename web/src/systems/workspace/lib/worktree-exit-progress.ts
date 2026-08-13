import type {
  WorktreeExitCTA,
  WorktreeExitEventName,
  WorktreeExitEventPayload,
  WorktreeExitPhase,
  WorktreeExitStepResult,
  WorktreeExitStream,
} from "./worktree-events";

/** One redacted chunk of hook output, as it arrived. */
export interface WorktreeExitHookChunk {
  stream: WorktreeExitStream;
  text: string;
}

export type WorktreeExitPhaseState = "pending" | "running" | "completed" | "skipped" | "failed";

export interface WorktreeExitProgressPhase {
  phase: WorktreeExitPhase;
  state: WorktreeExitPhaseState;
  /** Why a phase did no work ("nothing to commit"). Daemon literal. */
  reason?: string;
  /**
   * Captured command output, delivered with the phase's terminal frame.
   */
  output?: string;
  /**
   * Hook output as it streams, before the phase finishes. Chunks are already
   * redacted by the daemon; they are appended in arrival order and never
   * reordered, so what is on screen is what the hook actually printed.
   */
  chunks?: WorktreeExitHookChunk[];
  sha?: string;
  upstream?: string;
  prStatus?: string;
  prNumber?: number;
  url?: string;
}

export type WorktreeExitProgressStatus = "running" | "completed" | "failed" | "canceled";

export interface WorktreeExitProgress {
  operationID: string;
  action: string;
  /** Announced up front on the started frame, so every phase is visible from the first render. */
  phases: WorktreeExitProgressPhase[];
  status: WorktreeExitProgressStatus;
  /** Redacted failure message from the daemon. */
  message?: string;
  cause?: string;
  failedPhase?: WorktreeExitPhase;
  cta?: WorktreeExitCTA;
}

function toPhaseState(state: string | undefined): WorktreeExitPhaseState {
  switch (state) {
    case "running":
      return "running";
    case "skipped":
      return "skipped";
    case "failed":
      return "failed";
    case "completed":
      return "completed";
    default:
      return "pending";
  }
}

function applyStep(
  phases: WorktreeExitProgressPhase[],
  step: WorktreeExitStepResult
): WorktreeExitProgressPhase[] {
  const existing = phases.find(entry => entry.phase === step.phase);
  const applied: WorktreeExitProgressPhase = {
    phase: step.phase,
    state: toPhaseState(step.state),
    reason: step.reason,
    output: step.output,
    // Streamed chunks survive the terminal frame: they are the only record of
    // what the hook printed while it ran.
    ...(existing?.chunks ? { chunks: existing.chunks } : {}),
    sha: step.sha,
    upstream: step.upstream,
    prStatus: step.pr_status,
    prNumber: step.pr_number,
    url: step.url,
  };
  const index = phases.findIndex(entry => entry.phase === step.phase);
  if (index === -1) return [...phases, applied];
  const next = [...phases];
  next[index] = applied;
  return next;
}

/**
 * Appends one streamed chunk to its phase.
 *
 * A chunk for a phase the started frame never announced still lands, marked
 * running: dropping it would hide output the hook genuinely produced.
 */
function appendChunk(
  phases: WorktreeExitProgressPhase[],
  phase: WorktreeExitPhase,
  chunk: WorktreeExitHookChunk
): WorktreeExitProgressPhase[] {
  const index = phases.findIndex(entry => entry.phase === phase);
  if (index === -1) return [...phases, { phase, state: "running", chunks: [chunk] }];
  const next = [...phases];
  const current = next[index];
  next[index] = { ...current, chunks: [...(current.chunks ?? []), chunk] };
  return next;
}

function markRunning(
  phases: WorktreeExitProgressPhase[],
  phase: WorktreeExitPhase | undefined
): WorktreeExitProgressPhase[] {
  if (!phase) return phases;
  const index = phases.findIndex(entry => entry.phase === phase);
  if (index === -1) return [...phases, { phase, state: "running" }];
  const next = [...phases];
  next[index] = { ...next[index], phase, state: "running" };
  return next;
}

function applyResultSteps(
  phases: WorktreeExitProgressPhase[],
  payload: WorktreeExitEventPayload
): WorktreeExitProgressPhase[] {
  let next = phases;
  for (const step of payload.result?.steps ?? []) next = applyStep(next, step);
  return next;
}

function startProgress(payload: WorktreeExitEventPayload): WorktreeExitProgress {
  return {
    operationID: payload.op_id,
    action: payload.action,
    phases: (payload.phases ?? []).map(phase => ({ phase, state: "pending" as const })),
    status: "running",
  };
}

/**
 * Folds one stream frame into the progress model.
 *
 * Frames belonging to another operation are ignored, so a stale toast can never
 * be advanced by a newer action's events. A `started` frame always begins a new
 * model — that is the only frame that announces the phase list.
 */
export function reduceWorktreeExitEvent(
  current: WorktreeExitProgress | undefined,
  name: WorktreeExitEventName,
  payload: WorktreeExitEventPayload
): WorktreeExitProgress | undefined {
  if (name === "worktree.exit_action_started") return startProgress(payload);
  if (!current || current.operationID !== payload.op_id) return current;

  switch (name) {
    case "worktree.exit_hook_output": {
      if (!payload.phase || !payload.stream || !payload.chunk) return current;
      return {
        ...current,
        phases: appendChunk(current.phases, payload.phase, {
          stream: payload.stream,
          text: payload.chunk,
        }),
      };
    }
    case "worktree.exit_action_step": {
      const steps = payload.result?.steps ?? [];
      if (steps.length === 0) {
        return { ...current, phases: markRunning(current.phases, payload.phase) };
      }
      return { ...current, phases: applyResultSteps(current.phases, payload) };
    }
    case "worktree.exit_action_completed":
      return {
        ...current,
        phases: applyResultSteps(current.phases, payload),
        status: "completed",
        cta: payload.result?.cta,
      };
    case "worktree.exit_action_failed":
      return {
        ...current,
        phases: applyResultSteps(current.phases, payload),
        status: "failed",
        message: payload.message,
        cause: payload.cause,
        failedPhase: payload.phase,
        cta: payload.result?.cta,
      };
    case "worktree.exit_action_canceled":
      return {
        ...current,
        phases: applyResultSteps(current.phases, payload),
        status: "canceled",
        message: payload.message,
      };
    default:
      return current;
  }
}

export function isWorktreeExitProgressTerminal(progress: WorktreeExitProgress): boolean {
  return progress.status !== "running";
}
