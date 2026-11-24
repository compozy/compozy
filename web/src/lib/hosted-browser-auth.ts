import { resetHostedBrowserCrypto } from "./hosted-browser-crypto";

export {
  buildBrowserAuthorizeUrl,
  buildBrowserDPoP,
  buildUpgradeProUrl,
  computeJwkThumbprint,
  deriveCodeChallenge,
  encodePublicJwkParam,
  ensureBrowserKeyPair,
  exportPublicJwk,
  generateAuthState,
  generateCodeVerifier,
  hostedCloudOrigin,
  normalizeDpopHtu,
  replaceBrowserKeyPair,
  setBrowserKeyStore,
  setHostedCloudOrigin,
  type BrowserKeyStore,
  type BrowserPublicJwk,
  type HostedBrowserProvider,
} from "./hosted-browser-crypto";
export {
  clearHostedSessionMeta,
  clearPendingApproval,
  clearPendingPkceState,
  prepareBrowserAuthorizeRedirect,
  readHostedSessionMeta,
  readPendingApproval,
  readPendingPkceState,
  saveHostedSessionMeta,
  savePendingApproval,
  savePendingPkceState,
  validateBrowserCallback,
  type BrowserCallbackValidation,
  type StoredHostedSession,
  type StoredPendingApproval,
  type StoredPkceState,
} from "./hosted-browser-storage";

const HOSTED_DOMAIN_SUFFIX = ".compozy.run";
const CALLBACK_PATH = "/remote/v1/browser/callback";

export type HostedBrowserSignal =
  | { kind: "device_removed" }
  | { kind: "session_expired" }
  | { kind: "key_rebind_required" }
  | { kind: "sign_in_required"; code?: string }
  | { kind: "needs_pro" }
  | { kind: "waiting_approval"; approvalId: string }
  | { kind: "not_approved" }
  | { kind: "another_account"; signedInEmail?: string };

export type HostedBrowserAccessListener = (signal: HostedBrowserSignal) => void;

const listeners = new Set<HostedBrowserAccessListener>();
const reportedResponses = new WeakSet<Response>();

let modeOverride: boolean | undefined;

function currentLocation(): {
  hostname: string;
  protocol: string;
  pathname: string;
  origin: string;
} | null {
  if (typeof window === "undefined" || !window.location) return null;
  return {
    hostname: window.location.hostname,
    protocol: window.location.protocol,
    pathname: window.location.pathname,
    origin: window.location.origin,
  };
}

/**
 * True when the current origin is a Compozy hosted computer address
 * (`*.compozy.run`) or is landing on the hosted browser callback path.
 */
export function isHostedOriginCandidate(
  loc?: { hostname: string; protocol?: string; pathname?: string } | null
): boolean {
  if (modeOverride !== undefined) return modeOverride;
  const resolved = loc ?? currentLocation();
  if (!resolved) return false;
  if (resolved.pathname === CALLBACK_PATH) return true;
  const host = resolved.hostname
    .trim()
    .toLowerCase()
    .replace(/^\[|\]$/g, "");
  return host.endsWith(HOSTED_DOMAIN_SUFFIX) && host.length > HOSTED_DOMAIN_SUFFIX.length;
}

/** True when this page is operating against a hosted computer listener. */
export function isHostedBrowserMode(): boolean {
  return isHostedOriginCandidate();
}

/** Latches hosted browser mode explicitly (used by boundary bootstrap and tests). */
export function setHostedBrowserMode(enabled: boolean | undefined): void {
  modeOverride = enabled;
}

/** Resets all in-memory hosted-browser state (tests only). */
export function resetHostedBrowserAuth(): void {
  modeOverride = undefined;
  listeners.clear();
  resetHostedBrowserCrypto();
}

/** Derives a human-readable fallback computer name from a hosted hostname. */
export function hostedComputerNameFromHostname(hostname: string): string {
  const normalized = hostname.trim().toLowerCase();
  if (!normalized) return "This computer";
  if (normalized.endsWith(HOSTED_DOMAIN_SUFFIX)) {
    const slug = normalized.slice(0, -HOSTED_DOMAIN_SUFFIX.length);
    if (slug) return slug;
  }
  return normalized.split(".")[0] || normalized;
}

/** Derives a friendly browser device label for the approval screen. */
export function describeBrowserDevice(): string {
  if (typeof navigator === "undefined") return "This browser";
  const ua = navigator.userAgent ?? "";
  let device = "This browser";
  if (/iPhone/i.test(ua)) device = "iPhone";
  else if (/iPad/i.test(ua)) device = "iPad";
  else if (/Android/i.test(ua)) device = "Android";
  else if (/Macintosh|Mac OS X/i.test(ua)) device = "Mac";
  else if (/Windows/i.test(ua)) device = "Windows";
  else if (/Linux/i.test(ua)) device = "Linux";

  let browser = "";
  if (/Edg\//i.test(ua)) browser = "Edge";
  else if (/Firefox\//i.test(ua)) browser = "Firefox";
  else if (/Chrome\//i.test(ua) && !/Edg\//i.test(ua)) browser = "Chrome";
  else if (/Safari\//i.test(ua) && !/Chrome\//i.test(ua)) browser = "Safari";

  if (device === "This browser") return browser || device;
  return browser ? `${device} (${browser})` : device;
}

/** True when a same-origin fetch should carry a browser DPoP proof header. */
export function shouldAttachBrowserDPoP(url: string, baseOrigin?: string): boolean {
  if (!isHostedBrowserMode()) return false;
  const origin = baseOrigin ?? currentLocation()?.origin;
  if (!origin) return false;
  try {
    const parsed = new URL(url, origin);
    if (parsed.origin !== origin) return false;
    if (parsed.pathname === "/remote/v1/browser/status") return false;
    return parsed.pathname.startsWith("/api/") || parsed.pathname.startsWith("/remote/v1/");
  } catch {
    return false;
  }
}

function readStringField(obj: unknown, key: string): string | undefined {
  if (!obj || typeof obj !== "object") return undefined;
  const value = Reflect.get(obj, key);
  if (typeof value !== "string") return undefined;
  const trimmed = value.trim();
  return trimmed === "" ? undefined : trimmed;
}

/** Classifies a hosted listener HTTP error response into a U8 gate signal, if applicable. */
export function classifyHostedBrowserError(
  status: number,
  body: unknown
): HostedBrowserSignal | null {
  const code = readStringField(body, "code");
  const details = body && typeof body === "object" ? Reflect.get(body, "details") : undefined;

  if (status === 402 || code === "plan_required" || code === "hosted_route_plan_required") {
    return { kind: "needs_pro" };
  }

  if (status === 401) {
    if (code === "device_revoked") return { kind: "device_removed" };
    if (code === "remote_session_expired") return { kind: "session_expired" };
    if (code === "browser_key_rebind_required") return { kind: "key_rebind_required" };
    return { kind: "sign_in_required", code };
  }

  if (status === 403) {
    if (code === "device_pending_approval") {
      const approvalId =
        readStringField(details, "approval_id") ?? readStringField(body, "approval_id") ?? "";
      return { kind: "waiting_approval", approvalId };
    }
    if (code === "device_not_approved") return { kind: "not_approved" };
    if (code === "account_mismatch" || code === "wrong_account") {
      return {
        kind: "another_account",
        signedInEmail: readStringField(details, "email") ?? readStringField(body, "email"),
      };
    }
  }

  return null;
}

/** Subscribes to hosted browser access signals (revocation, session expiry, plan gating). */
export function observeHostedBrowserAccess(listener: HostedBrowserAccessListener): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

/** Broadcasts a hosted browser access signal to active boundary listeners. */
export function reportHostedBrowserAccess(signal: HostedBrowserSignal): void {
  for (const listener of Array.from(listeners)) {
    listener(signal);
  }
}

/** Inspects a hosted listener HTTP response once and reports gate-relevant errors. */
export async function reportHostedBrowserResponse(response: Response): Promise<void> {
  if (!isHostedBrowserMode()) return;
  if (response.status !== 401 && response.status !== 402 && response.status !== 403) return;
  if (reportedResponses.has(response)) return;
  reportedResponses.add(response);

  let body: unknown;
  try {
    body = await response.clone().json();
  } catch {
    body = undefined;
  }

  const signal = classifyHostedBrowserError(response.status, body);
  if (signal) {
    reportHostedBrowserAccess(signal);
  }
}
