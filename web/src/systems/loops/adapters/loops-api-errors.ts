import type { ValidateLoopResult } from "../types";

export class LoopsApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "LoopsApiError";
  }
}

/** A structured 422 lint rejection that the editor can map back onto nodes. */
export class LoopValidationError extends LoopsApiError {
  constructor(
    message: string,
    public readonly result: ValidateLoopResult
  ) {
    super(message, 422);
    this.name = "LoopValidationError";
  }
}

/**
 * The daemon's deterministic answer to a lifecycle verb it will not apply. The
 * contract carries a reason `code` plus `details` naming the state that won:
 * `actual_state`, `allowed_transitions`, and — when a concurrent verb beat this
 * one — the winner's `winner_actor_kind` / `winner_actor_id` / `winner_reason` /
 * `winner_requested_at`.
 *
 * These are not failures. The run page renders them as information, which is
 * only possible because the structure survives the adapter boundary intact
 * instead of being flattened into a message string.
 */
export class LoopLifecycleConflictError extends LoopsApiError {
  constructor(
    message: string,
    status: number,
    public readonly code: string,
    public readonly details: Readonly<Record<string, string>>
  ) {
    super(message, status);
    this.name = "LoopLifecycleConflictError";
  }

  get actualState(): string {
    return this.details.actual_state ?? "";
  }

  /** The verbs the daemon says are valid from `actualState`, already split. */
  get allowedTransitions(): string[] {
    return (this.details.allowed_transitions ?? "")
      .split(",")
      .map(verb => verb.trim())
      .filter(verb => verb !== "");
  }

  get winnerActorKind(): string {
    return this.details.winner_actor_kind ?? "";
  }

  get winnerActorId(): string {
    return this.details.winner_actor_id ?? "";
  }

  get winnerReason(): string {
    return this.details.winner_reason ?? "";
  }

  get winnerRequestedAt(): string {
    return this.details.winner_requested_at ?? "";
  }
}
