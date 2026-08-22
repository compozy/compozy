import type { ValidateLoopResult } from "../types";

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : null;
}

/**
 * The one trim-or-omit rule every Loops adapter builds its query with.
 *
 * A blank filter and an absent filter mean the same thing to the daemon, so they
 * have to serialize the same way here — and they have to do it from a single
 * implementation, because `normalizeText` in the query keys is written to match
 * this behaviour and a private copy per adapter lets the two drift apart.
 */
export function normalizeOptionalText(value?: string | null): string | undefined {
  if (typeof value !== "string") return undefined;
  const normalized = value.trim();
  return normalized === "" ? undefined : normalized;
}

/** Reads the daemon's `{code, details}` reason envelope off a rejected response. */
export function reasonEnvelope(error: unknown): { code: string; details: Record<string, string> } {
  const body = asRecord(error);
  const code = typeof body?.code === "string" ? body.code : "";
  const rawDetails = asRecord(body?.details);
  const details: Record<string, string> = {};
  for (const [key, value] of Object.entries(rawDetails ?? {})) {
    if (typeof value === "string") details[key] = value;
  }
  return { code, details };
}

export class LoopsApiError extends Error {
  constructor(
    message: string,
    public readonly status: number
  ) {
    super(message);
    this.name = "LoopsApiError";
  }
}

export interface LoopInputValidationPayload {
  field: string;
  kind?: string;
  loop: string;
  origin: string;
  reason: string;
  value?: string;
}

function inputValidationMessage(payload: LoopInputValidationPayload): string {
  const label = payload.field || "Input";
  switch (payload.reason) {
    case "required":
      return `${label} is required.`;
    case "enum_mismatch":
      return `${label} must use one of the allowed values.`;
    case "unknown_reference":
      return payload.kind
        ? `${label} references an unavailable ${payload.kind}.`
        : `${label} references an unavailable value.`;
    case "invalid_runtime":
      return `${label} uses an unavailable runtime.`;
    case "type_mismatch":
    case "invalid_kind_payload":
      return `${label} has an invalid value.`;
    case "unknown_input":
      return `${label} is not declared by this Loop.`;
    default:
      return `${label} could not be validated.`;
  }
}

/** A run rejection tied to one declared input field. */
export class LoopInputValidationError extends LoopsApiError {
  constructor(public readonly validation: LoopInputValidationPayload) {
    super(inputValidationMessage(validation), 422);
    this.name = "LoopInputValidationError";
  }

  get fieldErrors(): Readonly<Record<string, string>> {
    return this.validation.field === "" ? {} : { [this.validation.field]: this.message };
  }
}

export function inputValidationPayload(error: unknown): LoopInputValidationPayload | null {
  const body = asRecord(error);
  const validation = asRecord(body?.input_validation);
  if (
    typeof validation?.field !== "string" ||
    typeof validation.loop !== "string" ||
    typeof validation.origin !== "string" ||
    typeof validation.reason !== "string"
  ) {
    return null;
  }
  return {
    field: validation.field,
    loop: validation.loop,
    origin: validation.origin,
    reason: validation.reason,
    ...(typeof validation.kind === "string" ? { kind: validation.kind } : {}),
    ...(typeof validation.value === "string" ? { value: validation.value } : {}),
  };
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

export class LoopRequestError extends LoopsApiError {
  constructor(
    message: string,
    status: number,
    public readonly code: string,
    public readonly details: Readonly<Record<string, string>>
  ) {
    super(message, status);
    this.name = "LoopRequestError";
  }

  get fieldErrors(): Readonly<Record<string, string>> {
    return this.code === "request_validation_failed" ? this.details : {};
  }

  get isAnswerable(): boolean {
    return this.status === 422;
  }

  get recordedDecision(): string {
    return this.details.decision ?? this.details.answered_decision ?? "";
  }
}

/**
 * A rejection from the run read layer (`/briefing`, `/nodes`, `/timeline`).
 *
 * These carry structure the UI acts on rather than prose it prints: an invalid
 * roster state names the `allowed` set, and a cursor the daemon will not honour
 * says whether the page set moved (`timeline_branch_changed`) or the token was
 * malformed. The story recovers from a stale cursor by re-reading the newest
 * window — it never splices two histories together (Safety Invariant 7).
 */
export class LoopReadError extends LoopsApiError {
  constructor(
    message: string,
    status: number,
    public readonly code: string,
    public readonly details: Readonly<Record<string, string>>
  ) {
    super(message, status);
    this.name = "LoopReadError";
  }

  /** The cursor no longer addresses a readable page set; restart from the head. */
  get isStaleCursor(): boolean {
    return this.code === "timeline_branch_changed" || this.code === "invalid_cursor";
  }

  /** The roster state vocabulary the daemon accepts, already split. */
  get allowedStates(): string[] {
    return (this.details.allowed ?? "")
      .split(",")
      .map(state => state.trim())
      .filter(state => state !== "");
  }
}

export class LoopTimetravelError extends LoopsApiError {
  constructor(
    message: string,
    status: number,
    public readonly code: string,
    public readonly details: Readonly<Record<string, string>>
  ) {
    super(message, status);
    this.name = "LoopTimetravelError";
  }
}
