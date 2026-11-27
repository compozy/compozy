import {
  buildBrowserAuthorizeUrl,
  deriveCodeChallenge,
  ensureBrowserKeyPair,
  exportPublicJwk,
  generateAuthState,
  generateCodeVerifier,
  replaceBrowserKeyPair,
  type BrowserPublicJwk,
  type HostedBrowserProvider,
} from "./hosted-browser-crypto";

const PKCE_STORAGE_KEY = "compozy.hosted.pkce.v1";
const APPROVAL_STORAGE_KEY = "compozy.hosted.approval.v1";
const SESSION_STORAGE_KEY = "compozy.hosted.session.v1";

export interface StoredPkceState {
  state: string;
  codeVerifier: string;
  provider?: HostedBrowserProvider;
  createdAt: number;
}

export interface StoredPendingApproval {
  approvalId: string;
  provider?: HostedBrowserProvider;
  createdAt: number;
}

export interface StoredHostedSession {
  sessionId: string;
  expiresAt: number;
  scopes: string[];
}

export type BrowserCallbackValidation =
  | {
      ok: true;
      code: string;
      codeVerifier: string;
      provider?: HostedBrowserProvider;
    }
  | {
      ok: false;
      reason:
        | "state_mismatch"
        | "missing_code"
        | "access_denied"
        | "another_account"
        | "needs_pro"
        | "not_approved"
        | "approval_expired"
        | "provider_error";
      message?: string;
    };

function readSessionStorageJson<T>(key: string): T | null {
  if (typeof sessionStorage === "undefined") return null;
  try {
    const raw = sessionStorage.getItem(key);
    return raw ? (JSON.parse(raw) as T) : null;
  } catch {
    return null;
  }
}

function writeSessionStorageJson(key: string, value: unknown): void {
  if (typeof sessionStorage === "undefined") return;
  try {
    sessionStorage.setItem(key, JSON.stringify(value));
  } catch {
    // Ignore storage quota or private-mode restrictions.
  }
}

function removeSessionStorageKey(key: string): void {
  if (typeof sessionStorage === "undefined") return;
  try {
    sessionStorage.removeItem(key);
  } catch {
    // Ignore storage errors.
  }
}

export function savePendingPkceState(record: StoredPkceState): void {
  writeSessionStorageJson(PKCE_STORAGE_KEY, record);
}

export function readPendingPkceState(): StoredPkceState | null {
  return readSessionStorageJson<StoredPkceState>(PKCE_STORAGE_KEY);
}

export function clearPendingPkceState(): void {
  removeSessionStorageKey(PKCE_STORAGE_KEY);
}

export function savePendingApproval(record: StoredPendingApproval): void {
  writeSessionStorageJson(APPROVAL_STORAGE_KEY, record);
}

export function readPendingApproval(): StoredPendingApproval | null {
  return readSessionStorageJson<StoredPendingApproval>(APPROVAL_STORAGE_KEY);
}

export function clearPendingApproval(): void {
  removeSessionStorageKey(APPROVAL_STORAGE_KEY);
}

export function saveHostedSessionMeta(meta: StoredHostedSession): void {
  writeSessionStorageJson(SESSION_STORAGE_KEY, meta);
}

export function readHostedSessionMeta(): StoredHostedSession | null {
  return readSessionStorageJson<StoredHostedSession>(SESSION_STORAGE_KEY);
}

export function clearHostedSessionMeta(): void {
  removeSessionStorageKey(SESSION_STORAGE_KEY);
}

/**
 * Prepares a fresh PKCE + `state` authorize redirect using the persisted
 * non-extractable P-256 browser key (reusing the same key unless `replaceKey` is set).
 */
export async function prepareBrowserAuthorizeRedirect(options: {
  hostname: string;
  provider?: HostedBrowserProvider;
  replaceKey?: boolean;
  cloudOrigin?: string;
}): Promise<{ url: string; state: string; codeVerifier: string; publicJwk: BrowserPublicJwk }> {
  const keyPair = options.replaceKey ? await replaceBrowserKeyPair() : await ensureBrowserKeyPair();
  const publicJwk = await exportPublicJwk(keyPair.publicKey);
  const codeVerifier = generateCodeVerifier();
  const codeChallenge = await deriveCodeChallenge(codeVerifier);
  const state = generateAuthState();
  savePendingPkceState({
    state,
    codeVerifier,
    provider: options.provider,
    createdAt: Date.now(),
  });
  const url = buildBrowserAuthorizeUrl({
    hostname: options.hostname,
    publicJwk,
    codeChallenge,
    state,
    provider: options.provider,
    cloudOrigin: options.cloudOrigin,
  });
  return { url, state, codeVerifier, publicJwk };
}

/**
 * Validates the `/remote/v1/browser/callback` query parameters against the
 * stored PKCE `state` in `sessionStorage` and consumes the pending PKCE record.
 */
export function validateBrowserCallback(search: string): BrowserCallbackValidation {
  const params = new URLSearchParams(search.startsWith("?") ? search.slice(1) : search);
  const returnedState = params.get("state")?.trim() ?? "";
  const returnedError = params.get("error")?.trim() ?? "";
  const returnedDescription = params.get("error_description")?.trim() || undefined;
  const code = params.get("code")?.trim() ?? "";

  const stored = readPendingPkceState();
  clearPendingPkceState();

  if (!stored || !returnedState || stored.state !== returnedState) {
    return { ok: false, reason: "state_mismatch" };
  }

  if (returnedError) {
    if (returnedError === "account_mismatch" || returnedError === "wrong_account") {
      return { ok: false, reason: "another_account", message: returnedDescription };
    }
    if (returnedError === "plan_required" || returnedError === "hosted_route_plan_required") {
      return { ok: false, reason: "needs_pro", message: returnedDescription };
    }
    if (returnedError === "device_not_approved" || returnedError === "approval_denied") {
      return { ok: false, reason: "not_approved", message: returnedDescription };
    }
    if (returnedError === "approval_expired") {
      return { ok: false, reason: "approval_expired", message: returnedDescription };
    }
    if (returnedError === "access_denied") {
      return { ok: false, reason: "access_denied", message: returnedDescription };
    }
    return { ok: false, reason: "provider_error", message: returnedDescription ?? returnedError };
  }

  if (!code) {
    return { ok: false, reason: "missing_code" };
  }

  return {
    ok: true,
    code,
    codeVerifier: stored.codeVerifier,
    provider: stored.provider,
  };
}
