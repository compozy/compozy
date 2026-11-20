/**
 * How long a Continue or Fork dialog waits on the daemon before it hands the
 * operator a way on. A large source can keep either request busy for minutes;
 * past these bounds the dialog stops waiting and offers Try again instead.
 */
export const SESSION_DERIVE_PREVIEW_TIMEOUT_MS = 15_000;
export const SESSION_DERIVE_SUBMIT_TIMEOUT_MS = 60_000;

/** `signal` (the caller's own cancel), bounded by `timeoutMs`; the timeout aborts as `TimeoutError`. */
export function sessionDeriveDeadline(timeoutMs: number, signal?: AbortSignal): AbortSignal {
  const deadline = AbortSignal.timeout(timeoutMs);
  return signal ? AbortSignal.any([signal, deadline]) : deadline;
}

function errorName(error: unknown): string {
  return typeof error === "object" && error !== null && "name" in error
    ? String((error as { name: unknown }).name)
    : "";
}

/** The request outlived its deadline: the daemon may still be working on it. */
export function isSessionDeriveTimeout(error: unknown): boolean {
  return errorName(error) === "TimeoutError";
}

/** The operator stopped waiting (closed or canceled the dialog). */
export function isSessionDeriveAbort(error: unknown): boolean {
  return errorName(error) === "AbortError";
}
