/**
 * Transport-level classification of the daemon's mandatory sign-in gate signals
 * (HTTP 401 `sign_in_required` / `reauth_required`, WebSocket close `4401`, and
 * the SSE `sign_in_required` frame).
 *
 * Lives in `lib/` so shared transports (`api-client`, `ticketed-event-source`,
 * `ticketed-web-socket`, `web-socket-event-source`, terminal sockets) can report
 * gate closure without importing a domain module.
 */

import { isHostedBrowserMode } from "./hosted-browser-auth";

export const ACCOUNT_SIGN_IN_CLOSE_CODE = 4401;

export interface AccountGateSignal {
  readonly kind: "sign_in_required";
  readonly code?: string;
}

type AccountGateListener = (signal: AccountGateSignal) => void;

const listeners = new Set<AccountGateListener>();
const reportedResponses = new WeakSet<Response>();

function readErrorCode(payload: unknown): string | undefined {
  if (payload == null || typeof payload !== "object") return undefined;
  const code = Reflect.get(payload, "code");
  if (typeof code !== "string") return undefined;
  const normalized = code.trim();
  return normalized === "" ? undefined : normalized;
}

export function isAccountSignInCloseCode(code?: number): boolean {
  return code === ACCOUNT_SIGN_IN_CLOSE_CODE;
}

export function classifyAccountGateResponse(
  status: number,
  payload: unknown
): AccountGateSignal | undefined {
  if (status !== 401) return undefined;
  const code = readErrorCode(payload);
  if (code === "sign_in_required" || code === "reauth_required") {
    return { kind: "sign_in_required", code };
  }
  return undefined;
}

export function reportAccountGateSignal(signal: AccountGateSignal): void {
  if (isHostedBrowserMode()) return;
  for (const listener of Array.from(listeners)) {
    listener(signal);
  }
}

export async function reportAccountGateResponse(response: Response): Promise<void> {
  if (isHostedBrowserMode()) return;
  if (response.status !== 401) return;
  if (reportedResponses.has(response)) return;
  reportedResponses.add(response);
  try {
    const signal = classifyAccountGateResponse(response.status, await response.clone().json());
    if (signal) {
      reportAccountGateSignal(signal);
    }
  } catch {
    // Non-JSON 401 carries no sign-in gate decision.
  }
}

export function observeAccountGateSignal(listener: AccountGateListener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** Resets all listeners (tests only). */
export function resetAccountGateSignalForTest(): void {
  listeners.clear();
}
