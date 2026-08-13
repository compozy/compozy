/**
 * Per-worktree stream vocabulary, mirroring `internal/worktree/events.go`.
 *
 * `worktree.pre_create` / `worktree.pre_remove` are hook gates dispatched at the
 * call site — they are never published to the event store, so they are not
 * stream frames and are deliberately absent here.
 */
export const WORKTREE_LIFECYCLE_EVENTS = [
  "worktree.created",
  "worktree.adopted",
  "worktree.removed",
  "worktree.missing",
  "worktree.dismissed",
  "worktree.creation_canceled",
  "worktree.setup_failed",
  "worktree.status_refreshed",
  "worktree.branch_reclaimed",
] as const;

export const WORKTREE_EXIT_EVENTS = [
  "worktree.exit_action_started",
  "worktree.exit_action_step",
  "worktree.exit_hook_output",
  "worktree.exit_action_completed",
  "worktree.exit_action_failed",
  "worktree.exit_action_canceled",
] as const;

export type WorktreeLifecycleEventName = (typeof WORKTREE_LIFECYCLE_EVENTS)[number];
export type WorktreeExitEventName = (typeof WORKTREE_EXIT_EVENTS)[number];
export type WorktreeStreamEventName = WorktreeLifecycleEventName | WorktreeExitEventName;

/** Terminal frames close an operation; the ladder and status reread after them. */
export const WORKTREE_EXIT_TERMINAL_EVENTS: readonly WorktreeExitEventName[] = [
  "worktree.exit_action_completed",
  "worktree.exit_action_failed",
  "worktree.exit_action_canceled",
];

export type WorktreeExitPhase = "commit" | "push" | "pr";

/** Which pipe a hook chunk came from. The daemon redacts every chunk before emitting it. */
export type WorktreeExitStream = "stdout" | "stderr";

/**
 * One executed phase. `output` is the command output captured with the step's
 * terminal frame; live chunks arrive separately on `worktree.exit_hook_output`
 * while the phase is still running.
 */
export interface WorktreeExitStepResult {
  phase: WorktreeExitPhase;
  state: string;
  reason?: string;
  output?: string;
  sha?: string;
  upstream?: string;
  pr_status?: string;
  pr_number?: number;
  url?: string;
}

export interface WorktreeExitCTA {
  action: string;
  label: string;
  url?: string;
}

export interface WorktreeExitActionResult {
  op_id: string;
  action: string;
  steps: WorktreeExitStepResult[];
  cta?: WorktreeExitCTA;
}

/** Frame payload for every `worktree.exit_action_*` event. */
export interface WorktreeExitEventPayload {
  op_id: string;
  action: string;
  /** Present on the started frame only: the full phase list, announced up front. */
  phases?: WorktreeExitPhase[];
  phase?: WorktreeExitPhase;
  state?: string;
  result?: WorktreeExitActionResult;
  message?: string;
  cause?: string;
  /** Set on `worktree.exit_hook_output` only: which pipe the chunk came from. */
  stream?: WorktreeExitStream;
  /** Set on `worktree.exit_hook_output` only: one redacted output chunk. */
  chunk?: string;
}

const EXIT_PHASES = new Set<string>(["commit", "push", "pr"]);
const EXIT_STREAMS = new Set<string>(["stdout", "stderr"]);

function isExitStream(value: unknown): value is WorktreeExitStream {
  return typeof value === "string" && EXIT_STREAMS.has(value);
}

function isExitPhase(value: unknown): value is WorktreeExitPhase {
  return typeof value === "string" && EXIT_PHASES.has(value);
}

function toStepResult(value: unknown): WorktreeExitStepResult | undefined {
  if (typeof value !== "object" || value === null) return undefined;
  const step = value as Partial<WorktreeExitStepResult>;
  if (!isExitPhase(step.phase) || typeof step.state !== "string") return undefined;
  return { ...step, phase: step.phase, state: step.state };
}

function toActionResult(value: unknown): WorktreeExitActionResult | undefined {
  if (typeof value !== "object" || value === null) return undefined;
  const result = value as Partial<WorktreeExitActionResult>;
  if (typeof result.op_id !== "string" || typeof result.action !== "string") return undefined;
  const steps = Array.isArray(result.steps)
    ? result.steps.flatMap(step => {
        const parsed = toStepResult(step);
        return parsed ? [parsed] : [];
      })
    : [];
  return { ...result, op_id: result.op_id, action: result.action, steps };
}

/**
 * Validates a raw stream frame before anything reaches the progress model. An
 * unparseable frame is dropped rather than rendered as a half-known step.
 */
export function parseWorktreeExitEvent(data: string): WorktreeExitEventPayload | undefined {
  try {
    const parsed: unknown = JSON.parse(data);
    if (typeof parsed !== "object" || parsed === null) return undefined;
    const payload = parsed as Partial<WorktreeExitEventPayload>;
    if (typeof payload.op_id !== "string" || payload.op_id.trim() === "") return undefined;
    if (typeof payload.action !== "string" || payload.action.trim() === "") return undefined;
    const phases = Array.isArray(payload.phases) ? payload.phases.filter(isExitPhase) : undefined;
    return {
      ...payload,
      op_id: payload.op_id,
      action: payload.action,
      phases,
      phase: isExitPhase(payload.phase) ? payload.phase : undefined,
      stream: isExitStream(payload.stream) ? payload.stream : undefined,
      chunk: typeof payload.chunk === "string" ? payload.chunk : undefined,
      result: toActionResult(payload.result),
    };
  } catch {
    return undefined;
  }
}
