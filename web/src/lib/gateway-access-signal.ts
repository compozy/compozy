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
 *
 * The tier latch is similarly narrow: a parsable `X-Compozy-Gateway-Tier`
 * publishes on any response, and only the authoritative `/api/status` response
 * may publish *unknown* (a missing or unparsable header there resets to the
 * default-hidden set). Other responses never unlatch a latched tier.
 */
export type GatewayAccessSignal = "unauthenticated" | "revoked";
export type GatewayListenerTier = "local" | "private" | "public";

const UNAUTHENTICATED_CODE = "gateway_device_unauthenticated";
const REVOKED_CODE = "gateway_device_revoked";
const GATEWAY_TIER_HEADER = "X-Compozy-Gateway-Tier";

type AccessSignalListener = (signal: GatewayAccessSignal) => void;
/**
 * Tier observations carry a parsable `GatewayListenerTier`, or `undefined`
 * when the authoritative `/api/status` response proves the tier unknown
 * (US-001.EC-2) — the default-hidden capability set applies (BR-2).
 */
type TierListener = (tier: GatewayListenerTier | undefined) => void;
/**
 * Forbidden envelopes are handed over raw: this transport knows only that the
 * daemon refused, never which refusals mean what. Deciding that a 403 code is
 * a domain signal (e.g. loopback-only) belongs to the gateway system, which
 * subscribes here and classifies.
 */
type ForbiddenResponseListener = (status: number, payload: unknown) => void;

const listeners = new Set<AccessSignalListener>();
const tierListeners = new Set<TierListener>();
const forbiddenListeners = new Set<ForbiddenResponseListener>();

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

export function reportGatewayListenerTier(rawTier: string | null): void {
  const tier = parseGatewayListenerTier(rawTier);
  if (!tier) return;
  for (const listener of tierListeners) listener(tier);
}

export function parseGatewayListenerTier(rawTier: string | null): GatewayListenerTier | undefined {
  const tier = rawTier?.trim().toLowerCase();
  if (tier !== "local" && tier !== "private" && tier !== "public") return undefined;
  return tier;
}

export function readGatewayListenerTier(response: Response): GatewayListenerTier | undefined {
  return parseGatewayListenerTier(response.headers.get(GATEWAY_TIER_HEADER));
}

/**
 * Tier latch for one response. A parsable tier header publishes on any
 * response. A missing or unparsable one keeps the last latched tier — except
 * on `/api/status` itself, the tier's authoritative source, where it publishes
 * unknown instead (US-001.EC-2): a proxy that strips the header must not leave
 * a stale tier latched. Random non-status responses never unlatch.
 */
function reportResponseGatewayTier(response: Response): void {
  const tier = parseGatewayListenerTier(response.headers.get(GATEWAY_TIER_HEADER));
  if (!tier && !isGatewayStatusResponse(response)) return;
  for (const listener of tierListeners) listener(tier);
}

/**
 * True only for the daemon status endpoint — the one response allowed to
 * publish an unknown tier. `Response.url` is empty for synthesized responses,
 * which therefore never count as the status endpoint.
 */
function isGatewayStatusResponse(response: Response): boolean {
  try {
    return new URL(response.url).pathname === "/api/status";
  } catch {
    return false;
  }
}

export async function reportGatewayResponse(response: Response): Promise<void> {
  reportResponseGatewayTier(response);
  if (response.status === 403) {
    await reportForbiddenResponse(response);
    return;
  }
  if (response.status !== 401) return;
  try {
    reportGatewayAccess(response.status, await response.clone().json());
  } catch {
    // A 401 without a JSON envelope carries no access decision.
  }
}

/**
 * Publishes a 403 envelope to forbidden-response observers. Fired only for
 * 403s (other statuses say nothing a listener classified) and skipped entirely
 * when nobody observes, so unobserved refusals cost no body parse.
 */
async function reportForbiddenResponse(response: Response): Promise<void> {
  if (forbiddenListeners.size === 0) return;
  try {
    const payload: unknown = await response.clone().json();
    for (const listener of forbiddenListeners) listener(response.status, payload);
  } catch {
    // A 403 without a JSON envelope carries nothing to classify.
  }
}

/** Subscribes to access signals. Returns the unsubscribe function. */
export function observeGatewayAccess(listener: AccessSignalListener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** Subscribes to 403 envelopes. Returns the unsubscribe function. */
export function observeGatewayForbiddenResponse(listener: ForbiddenResponseListener): () => void {
  forbiddenListeners.add(listener);
  return () => {
    forbiddenListeners.delete(listener);
  };
}

export function observeGatewayListenerTier(listener: TierListener): () => void {
  tierListeners.add(listener);
  return () => {
    tierListeners.delete(listener);
  };
}
