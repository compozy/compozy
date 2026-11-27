import { useEffect, useEffectEvent, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";

import {
  buildUpgradeProUrl,
  classifyHostedBrowserError,
  clearHostedSessionMeta,
  clearPendingApproval,
  clearPendingPkceState,
  describeBrowserDevice,
  hostedComputerNameFromHostname,
  observeHostedBrowserAccess,
  prepareBrowserAuthorizeRedirect,
  savePendingApproval,
  type HostedBrowserProvider,
} from "@/lib/hosted-browser-auth";

import {
  endHostedBrowserSession,
  fetchHostedBrowserApproval,
  fetchHostedBrowserPeers,
  HostedBrowserApiError,
} from "../adapters/hosted-browser-api";
import type { HostedBrowserContextValue } from "../contexts/hosted-browser-context";
import {
  bootstrapHostedBrowserSession,
  isAbortError,
  type HostedBrowserGateState,
} from "../lib/hosted-browser-bootstrap";
import {
  hasHostedScope,
  toHostedComputerFixture,
  toHostedPeerViews,
} from "../lib/hosted-browser-view";
import type { HostedPeerFixture } from "../mocks/remote-computers-types";

export type { HostedBrowserGateState } from "../lib/hosted-browser-bootstrap";

const APPROVAL_POLL_INTERVAL_MS = 5_000;
const PROACTIVE_REFRESH_LEAD_MS = 120_000;
const MIN_PROACTIVE_REFRESH_MS = 5_000;

export interface UseHostedBrowserAccessOptions {
  enabled: boolean;
  /** Override navigation (for tests). Defaults to `window.location.assign`. */
  navigate?: (url: string) => void;
  /** Override opening external billing/docs URLs (for tests). */
  openExternal?: (url: string) => void;
  /** Override approval polling interval in ms (for tests). */
  pollIntervalMs?: number;
}

export interface HostedBrowserAccessModel {
  state: HostedBrowserGateState;
  hostname: string;
  computerName: string;
  deviceName: string;
  browserFingerprint: string;
  contextValue: HostedBrowserContextValue | null;
  signInWithProvider: (provider: HostedBrowserProvider) => void;
  primaryGateAction: () => void;
  secondaryGateAction: () => void;
}

function defaultNavigate(url: string): void {
  if (typeof window !== "undefined") {
    window.location.assign(url);
  }
}

function defaultOpenExternal(url: string): void {
  if (typeof window !== "undefined") {
    window.open(url, "_blank", "noopener,noreferrer");
  }
}

function currentHostname(): string {
  if (typeof window === "undefined") return "";
  return window.location.hostname || "";
}

/**
 * Owns the hosted browser sign-in lifecycle (`*.compozy.run`, U8):
 * WebCrypto P-256 non-extractable key + PKCE authorize redirect, callback
 * redemption, 5 s approval polling with automatic re-authorization, live
 * revocation / entitlement gating, proactive silent re-mint, peers, and Sign out.
 */
export function useHostedBrowserAccess({
  enabled,
  navigate = defaultNavigate,
  openExternal = defaultOpenExternal,
  pollIntervalMs = APPROVAL_POLL_INTERVAL_MS,
}: UseHostedBrowserAccessOptions): HostedBrowserAccessModel {
  const queryClient = useQueryClient();
  const [state, setState] = useState<HostedBrowserGateState>({ kind: "checking" });
  const [browserFingerprint, setBrowserFingerprint] = useState("");
  const [reconnecting, setReconnecting] = useState(false);
  const redirectingRef = useRef(false);

  const hostname = currentHostname();
  const deviceName = describeBrowserDevice();

  const beginAuthorize = (options: { provider?: HostedBrowserProvider; replaceKey?: boolean }) => {
    if (redirectingRef.current) return;
    redirectingRef.current = true;
    void prepareBrowserAuthorizeRedirect({
      hostname: currentHostname(),
      provider: options.provider,
      replaceKey: options.replaceKey,
    })
      .then(({ url }) => {
        navigate(url);
      })
      .catch(() => {
        redirectingRef.current = false;
        setState({
          kind: "sign_in",
          errorMessage: "Couldn't start sign-in in this browser. Try again.",
        });
      });
  };
  const authorizeFromEffect = useEffectEvent(beginAuthorize);

  // Bootstrap hosted session state on mount.
  useEffect(() => {
    if (!enabled) return;
    const controller = new AbortController();
    let cancelled = false;

    void bootstrapHostedBrowserSession(controller.signal)
      .then(outcome => {
        if (cancelled) return;
        setBrowserFingerprint(outcome.fingerprint);
        if (outcome.kind === "authorize") {
          authorizeFromEffect({ provider: outcome.provider, replaceKey: outcome.replaceKey });
        } else {
          setState(outcome.state);
        }
      })
      .catch(err => {
        if (cancelled || isAbortError(err)) return;
        setState({
          kind: "sign_in",
          errorMessage: "This browser couldn't initialize secure sign-in keys.",
        });
      });

    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [enabled]);

  // Poll pending approval every 5 seconds while waiting.
  const waitingApprovalId = state.kind === "waiting_approval" ? state.approvalId : null;
  const waitingProvider = state.kind === "waiting_approval" ? state.provider : undefined;

  useEffect(() => {
    if (!enabled || !waitingApprovalId) return;
    let cancelled = false;
    const controller = new AbortController();

    const pollOnce = async () => {
      try {
        const result = await fetchHostedBrowserApproval(waitingApprovalId, controller.signal);
        if (cancelled) return;
        if (result.state === "approved") {
          clearPendingApproval();
          authorizeFromEffect({ provider: waitingProvider, replaceKey: false });
        } else if (result.state === "denied") {
          clearPendingApproval();
          setState({ kind: "not_approved", provider: waitingProvider });
        } else if (result.state === "expired") {
          clearPendingApproval();
          setState({ kind: "approval_expired", provider: waitingProvider });
        }
      } catch (err) {
        if (cancelled || isAbortError(err)) return;
        if (err instanceof HostedBrowserApiError) {
          const signal = classifyHostedBrowserError(err.status, err.body);
          if (signal?.kind === "device_removed") {
            clearPendingApproval();
            setState({ kind: "device_removed" });
          } else if (signal?.kind === "needs_pro") {
            setState({ kind: "needs_pro" });
          }
        }
      }
    };

    const timer = setInterval(() => {
      void pollOnce();
    }, pollIntervalMs);

    return () => {
      cancelled = true;
      controller.abort();
      clearInterval(timer);
    };
  }, [enabled, pollIntervalMs, waitingApprovalId, waitingProvider]);

  // Subscribe to runtime 401/402/403 hosted access signals.
  useEffect(() => {
    if (!enabled) return;
    return observeHostedBrowserAccess(signal => {
      if (signal.kind === "device_removed") {
        clearHostedSessionMeta();
        clearPendingApproval();
        setState({ kind: "device_removed" });
        return;
      }
      if (signal.kind === "needs_pro") {
        setState({ kind: "needs_pro" });
        return;
      }
      if (signal.kind === "session_expired") {
        authorizeFromEffect({ replaceKey: false });
        return;
      }
      if (signal.kind === "key_rebind_required") {
        authorizeFromEffect({ replaceKey: true });
        return;
      }
      if (signal.kind === "another_account") {
        setState({ kind: "another_account", signedInEmail: signal.signedInEmail });
        return;
      }
      if (signal.kind === "not_approved") {
        setState({ kind: "not_approved" });
        return;
      }
      if (signal.kind === "waiting_approval" && signal.approvalId) {
        savePendingApproval({ approvalId: signal.approvalId, createdAt: Date.now() });
        setState({ kind: "waiting_approval", approvalId: signal.approvalId });
        return;
      }
      if (signal.kind === "sign_in_required") {
        setState(current => (current.kind === "ready" ? { kind: "sign_in" } : current));
      }
    });
  }, [enabled]);

  // Proactive silent re-mint before the 1 h browser session expires.
  const readyExpiresAt = state.kind === "ready" ? state.expiresAt : null;
  useEffect(() => {
    if (!enabled || readyExpiresAt === null) return;
    const targetMs = readyExpiresAt * 1000 - Date.now() - PROACTIVE_REFRESH_LEAD_MS;
    const delayMs = Math.max(MIN_PROACTIVE_REFRESH_MS, targetMs);
    const timer = setTimeout(() => {
      authorizeFromEffect({ replaceKey: false });
    }, delayMs);
    return () => clearTimeout(timer);
  }, [enabled, readyExpiresAt]);

  // Track browser online/offline transitions for the reconnecting indicator.
  useEffect(() => {
    if (!enabled || typeof window === "undefined") return;
    const onOffline = () => setReconnecting(true);
    const onOnline = () => {
      setReconnecting(false);
      void fetchHostedBrowserPeers()
        .then(peerRes => {
          setState(current =>
            current.kind === "ready" ? { ...current, peers: peerRes.peers ?? [] } : current
          );
        })
        .catch(() => undefined);
    };
    window.addEventListener("offline", onOffline);
    window.addEventListener("online", onOnline);
    return () => {
      window.removeEventListener("offline", onOffline);
      window.removeEventListener("online", onOnline);
    };
  }, [enabled]);

  const rawPeers = state.kind === "ready" ? state.peers : [];
  const scopes = state.kind === "ready" ? state.scopes : [];
  const peers = toHostedPeerViews(hostname, rawPeers);
  const computer = toHostedComputerFixture(hostname, peers, scopes);
  const computerName = computer.name || hostedComputerNameFromHostname(hostname);

  const handleSignOut = () => {
    clearHostedSessionMeta();
    clearPendingApproval();
    clearPendingPkceState();
    void endHostedBrowserSession()
      .catch(() => undefined)
      .finally(() => {
        queryClient.clear();
        redirectingRef.current = false;
        setState({ kind: "sign_in" });
      });
  };

  const handleSelectPeer = (peer: HostedPeerFixture) => {
    if (peer.current || !peer.hostedUrl) return;
    navigate(peer.hostedUrl);
  };

  const handleRetryConnection = () => {
    setReconnecting(true);
    void fetchHostedBrowserPeers()
      .then(peerRes => {
        setReconnecting(false);
        setState(current =>
          current.kind === "ready" ? { ...current, peers: peerRes.peers ?? [] } : current
        );
      })
      .catch(() => {
        setReconnecting(false);
      });
  };

  const contextValue: HostedBrowserContextValue | null =
    state.kind === "ready"
      ? {
          computer,
          peers,
          scopes,
          canOperateTerminal: hasHostedScope(scopes, "terminal:operate"),
          reconnecting,
          selectPeer: handleSelectPeer,
          signOut: handleSignOut,
          retryConnection: handleRetryConnection,
        }
      : null;

  const signInWithProvider = (provider: HostedBrowserProvider) => {
    clearPendingApproval();
    redirectingRef.current = false;
    beginAuthorize({ provider, replaceKey: false });
  };

  const primaryGateAction = () => {
    if (state.kind === "needs_pro") {
      openExternal(buildUpgradeProUrl(hostname));
      return;
    }
    if (state.kind === "approval_expired" || state.kind === "not_approved") {
      clearPendingApproval();
      redirectingRef.current = false;
      beginAuthorize({ provider: state.provider, replaceKey: false });
      return;
    }
    if (state.kind === "device_removed") {
      clearPendingApproval();
      clearHostedSessionMeta();
      redirectingRef.current = false;
      setState({ kind: "sign_in" });
      return;
    }
    if (state.kind === "another_account") {
      handleSignOut();
      return;
    }
    redirectingRef.current = false;
    beginAuthorize({ replaceKey: false });
  };

  const secondaryGateAction = () => {
    if (state.kind === "needs_pro") {
      openExternal(buildUpgradeProUrl(hostname));
      return;
    }
    clearPendingApproval();
    clearPendingPkceState();
    redirectingRef.current = false;
    setState({ kind: "sign_in" });
  };

  return {
    state,
    hostname,
    computerName,
    deviceName,
    browserFingerprint,
    contextValue,
    signInWithProvider,
    primaryGateAction,
    secondaryGateAction,
  };
}
