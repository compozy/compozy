import {
  classifyHostedBrowserError,
  clearPendingApproval,
  computeJwkThumbprint,
  ensureBrowserKeyPair,
  exportPublicJwk,
  readHostedSessionMeta,
  readPendingApproval,
  saveHostedSessionMeta,
  savePendingApproval,
  validateBrowserCallback,
  type HostedBrowserProvider,
  type HostedBrowserSignal,
} from "@/lib/hosted-browser-auth";

import {
  fetchHostedBrowserApproval,
  fetchHostedBrowserPeers,
  fetchHostedBrowserStatus,
  HostedBrowserApiError,
  startHostedBrowserSession,
  type HostedPeerPayload,
} from "../adapters/hosted-browser-api";
import { normalizeHostedScopes } from "./hosted-browser-view";
import type { RemoteScope } from "../mocks/remote-computers-types";

const CALLBACK_PATH = "/remote/v1/browser/callback";

export type HostedBrowserGateState =
  | { kind: "checking" }
  | {
      kind: "ready";
      scopes: RemoteScope[];
      expiresAt: number | null;
      peers: HostedPeerPayload[];
    }
  | { kind: "sign_in"; errorMessage?: string }
  | {
      kind: "waiting_approval";
      approvalId: string;
      provider?: HostedBrowserProvider;
    }
  | { kind: "approval_expired"; provider?: HostedBrowserProvider }
  | { kind: "not_approved"; provider?: HostedBrowserProvider }
  | { kind: "another_account"; signedInEmail?: string }
  | { kind: "needs_pro"; graceDaysLeft?: number | null }
  | { kind: "device_removed" };

export type HostedBootstrapOutcome =
  | { kind: "state"; state: HostedBrowserGateState; fingerprint: string }
  | {
      kind: "authorize";
      provider?: HostedBrowserProvider;
      replaceKey: boolean;
      fingerprint: string;
    };

export function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}

export function gateStateForHostedSignal(
  signal: HostedBrowserSignal
): HostedBrowserGateState | null {
  switch (signal.kind) {
    case "device_removed":
      return { kind: "device_removed" };
    case "needs_pro":
      return { kind: "needs_pro" };
    case "another_account":
      return { kind: "another_account", signedInEmail: signal.signedInEmail };
    case "not_approved":
      return { kind: "not_approved" };
    case "waiting_approval":
      return { kind: "waiting_approval", approvalId: signal.approvalId };
    case "sign_in_required":
      return { kind: "sign_in" };
    case "session_expired":
    case "key_rebind_required":
      return null;
  }
}

/**
 * Resolves the initial hosted-browser gate state on page load:
 * key initialization, `/remote/v1/browser/status`, OAuth callback redemption,
 * pending approval resume, and existing cookie+DPoP session probe.
 */
export async function bootstrapHostedBrowserSession(
  signal: AbortSignal
): Promise<HostedBootstrapOutcome> {
  const keyPair = await ensureBrowserKeyPair();
  const publicJwk = await exportPublicJwk(keyPair.publicKey);
  const fingerprint = await computeJwkThumbprint(publicJwk);

  try {
    const status = await fetchHostedBrowserStatus(signal);
    if (status.state === "plan_required") {
      return { kind: "state", state: { kind: "needs_pro" }, fingerprint };
    }
  } catch (err) {
    if (isAbortError(err)) throw err;
    if (err instanceof HostedBrowserApiError) {
      const classified = classifyHostedBrowserError(err.status, err.body);
      if (classified?.kind === "needs_pro") {
        return { kind: "state", state: { kind: "needs_pro" }, fingerprint };
      }
    }
  }

  // 1. OAuth callback return from cloud.compozy.com.
  if (typeof window !== "undefined" && window.location.pathname === CALLBACK_PATH) {
    const search = window.location.search;
    window.history.replaceState(null, "", "/");
    const validation = validateBrowserCallback(search);
    if (!validation.ok) {
      if (validation.reason === "another_account") {
        return { kind: "state", state: { kind: "another_account" }, fingerprint };
      }
      if (validation.reason === "needs_pro") {
        return { kind: "state", state: { kind: "needs_pro" }, fingerprint };
      }
      if (validation.reason === "not_approved") {
        return { kind: "state", state: { kind: "not_approved" }, fingerprint };
      }
      if (validation.reason === "approval_expired") {
        return { kind: "state", state: { kind: "approval_expired" }, fingerprint };
      }
      return {
        kind: "state",
        state: {
          kind: "sign_in",
          errorMessage:
            validation.reason === "state_mismatch"
              ? "Sign-in verification failed. Try signing in again."
              : validation.message,
        },
        fingerprint,
      };
    }

    try {
      const session = await startHostedBrowserSession(
        { code: validation.code, code_verifier: validation.codeVerifier },
        signal
      );
      clearPendingApproval();
      const scopes = normalizeHostedScopes(session.scopes);
      saveHostedSessionMeta({
        sessionId: session.session_id,
        expiresAt: session.expires_at,
        scopes,
      });
      let peers: HostedPeerPayload[] = [];
      try {
        const peerRes = await fetchHostedBrowserPeers(signal);
        peers = peerRes.peers ?? [];
      } catch {
        peers = [];
      }
      return {
        kind: "state",
        state: { kind: "ready", scopes, expiresAt: session.expires_at, peers },
        fingerprint,
      };
    } catch (err) {
      if (isAbortError(err)) throw err;
      if (err instanceof HostedBrowserApiError) {
        const classified = classifyHostedBrowserError(err.status, err.body);
        if (classified?.kind === "key_rebind_required") {
          return {
            kind: "authorize",
            provider: validation.provider,
            replaceKey: true,
            fingerprint,
          };
        }
        if (classified?.kind === "waiting_approval" && classified.approvalId) {
          savePendingApproval({
            approvalId: classified.approvalId,
            provider: validation.provider,
            createdAt: Date.now(),
          });
          return {
            kind: "state",
            state: {
              kind: "waiting_approval",
              approvalId: classified.approvalId,
              provider: validation.provider,
            },
            fingerprint,
          };
        }
        const mapped = classified ? gateStateForHostedSignal(classified) : null;
        if (mapped) return { kind: "state", state: mapped, fingerprint };
        return {
          kind: "state",
          state: { kind: "sign_in", errorMessage: err.message },
          fingerprint,
        };
      }
      return {
        kind: "state",
        state: { kind: "sign_in", errorMessage: "Couldn't complete sign-in. Try again." },
        fingerprint,
      };
    }
  }

  // 2. Resume a pending approval if stored in sessionStorage.
  const pendingApproval = readPendingApproval();
  if (pendingApproval?.approvalId) {
    try {
      const approval = await fetchHostedBrowserApproval(pendingApproval.approvalId, signal);
      if (approval.state === "pending") {
        return {
          kind: "state",
          state: {
            kind: "waiting_approval",
            approvalId: pendingApproval.approvalId,
            provider: pendingApproval.provider,
          },
          fingerprint,
        };
      }
      clearPendingApproval();
      if (approval.state === "approved") {
        return {
          kind: "authorize",
          provider: pendingApproval.provider,
          replaceKey: false,
          fingerprint,
        };
      }
      if (approval.state === "denied") {
        return {
          kind: "state",
          state: { kind: "not_approved", provider: pendingApproval.provider },
          fingerprint,
        };
      }
      if (approval.state === "expired") {
        return {
          kind: "state",
          state: { kind: "approval_expired", provider: pendingApproval.provider },
          fingerprint,
        };
      }
    } catch (err) {
      if (isAbortError(err)) throw err;
      clearPendingApproval();
    }
  }

  // 3. Probe existing browser cookie + DPoP key via `/remote/v1/browser/peers`.
  try {
    const peerRes = await fetchHostedBrowserPeers(signal);
    const storedMeta = readHostedSessionMeta();
    const scopes = normalizeHostedScopes(storedMeta?.scopes);
    return {
      kind: "state",
      state: {
        kind: "ready",
        scopes,
        expiresAt: storedMeta?.expiresAt ?? null,
        peers: peerRes.peers ?? [],
      },
      fingerprint,
    };
  } catch (err) {
    if (isAbortError(err)) throw err;
    if (err instanceof HostedBrowserApiError) {
      const classified = classifyHostedBrowserError(err.status, err.body);
      if (classified?.kind === "session_expired") {
        return { kind: "authorize", replaceKey: false, fingerprint };
      }
      if (classified?.kind === "key_rebind_required") {
        return { kind: "authorize", replaceKey: true, fingerprint };
      }
      const mapped = classified ? gateStateForHostedSignal(classified) : null;
      return { kind: "state", state: mapped ?? { kind: "sign_in" }, fingerprint };
    }
    return { kind: "state", state: { kind: "sign_in" }, fingerprint };
  }
}
