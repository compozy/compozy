/**
 * Transport-level classification of the two gateway responses that end a
 * device's access, plus the observer seam the gateway system subscribes to.
 *
 * This lives in `lib/` rather than in the gateway system so the shared stream
 * transport can report an ended session without importing a domain module
 * (which would close an import cycle through `@/lib/api-client`).
 *
 * Deliberately narrow: only an HTTP 401 carrying one of two explicit daemon
 * codes counts. Network failures, aborts, timeouts, 5xx, and even a rejected
 * stream ticket are *not* access decisions — a flaky link must never eject the
 * operator from the app, and a spent single-use ticket is resolved by minting a
 * fresh one, not by tearing the session down.
 */
export type GatewayAccessSignal = "unauthenticated" | "revoked";

const UNAUTHENTICATED_CODE = "gateway_device_unauthenticated";
const REVOKED_CODE = "gateway_device_revoked";

type AccessSignalListener = (signal: GatewayAccessSignal) => void;

const listeners = new Set<AccessSignalListener>();

/** Reads the daemon's stable machine code out of an error envelope. */
export function gatewayErrorCode(payload: unknown): string | undefined {
  if (payload == null || typeof payload !== "object") return undefined;
  const code = Reflect.get(payload, "code");
  if (typeof code !== "string") return undefined;
  const normalized = code.trim();
  return normalized === "" ? undefined : normalized;
}

/**
 * Returns the access signal a response represents, or `undefined` when the
 * failure says nothing about this device's standing.
 */
export function classifyGatewayAccess(
  status: number,
  payload: unknown
): GatewayAccessSignal | undefined {
  if (status !== 401) return undefined;
  switch (gatewayErrorCode(payload)) {
    case REVOKED_CODE:
      return "revoked";
    case UNAUTHENTICATED_CODE:
      return "unauthenticated";
    default:
      return undefined;
  }
}

/** Publishes a classified signal. A non-signal response is a no-op. */
export function reportGatewayAccess(status: number, payload: unknown): void {
  const signal = classifyGatewayAccess(status, payload);
  if (!signal) return;
  for (const listener of listeners) listener(signal);
}

/** Subscribes to access signals. Returns the unsubscribe function. */
export function observeGatewayAccess(listener: AccessSignalListener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}
