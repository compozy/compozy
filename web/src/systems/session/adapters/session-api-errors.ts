import {
  apiErrorCode,
  apiErrorCurrentTurnId,
  apiErrorQueueCap,
  defaultApiErrorMessage,
} from "@/lib/api-client";

export interface SessionApiErrorDetail {
  /** Deterministic daemon error code, e.g. `active_turn_mismatch`. */
  code?: string;
  /** The turn the daemon reports as active when a strict fence is refused. */
  currentTurnId?: string;
  /** The queue cap named by a `queue_full` refusal. */
  queueCap?: number;
}

export class SessionApiError extends Error {
  readonly code: string | null;
  readonly currentTurnId: string | null;
  readonly queueCap: number | null;

  constructor(
    message: string,
    public readonly status: number,
    public readonly sessionId?: string,
    detail: SessionApiErrorDetail = {}
  ) {
    super(message);
    this.name = "SessionApiError";
    this.code = detail.code ?? null;
    this.currentTurnId = detail.currentTurnId ?? null;
    this.queueCap = detail.queueCap ?? null;
  }
}

export class SessionNotFoundError extends SessionApiError {
  constructor(id: string) {
    super(`Session not found: ${id}`, 404, id);
    this.name = "SessionNotFoundError";
  }
}

export function throwSessionRequestError(
  response: Response,
  error: unknown,
  fallback: string,
  sessionId?: string
): never {
  if (response.status === 404 && sessionId) {
    throw new SessionNotFoundError(sessionId);
  }
  throw new SessionApiError(
    defaultApiErrorMessage(fallback, response, error),
    response.status,
    sessionId,
    {
      code: apiErrorCode(error),
      currentTurnId: apiErrorCurrentTurnId(error),
      queueCap: apiErrorQueueCap(error),
    }
  );
}
