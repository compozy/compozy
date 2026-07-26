import { defaultApiErrorMessage } from "@/lib/api-client";

export class SessionApiError extends Error {
  constructor(
    message: string,
    public readonly status: number,
    public readonly sessionId?: string
  ) {
    super(message);
    this.name = "SessionApiError";
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
    sessionId
  );
}
