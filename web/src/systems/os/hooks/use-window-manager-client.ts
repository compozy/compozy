import { useEffect, useEffectEvent, useRef, useState } from "react";

import { registerWindowManagerClient } from "../adapters/window-manager-api";
import type { WindowManagerClientView } from "../lib/window-manager-types";

const CLIENT_ID_STORAGE_KEY = "agh.window-manager.client-id";

function randomClientId(): string {
  const cryptoApi = globalThis.crypto;
  if (typeof cryptoApi?.randomUUID === "function") {
    return `web-${cryptoApi.randomUUID()}`;
  }
  if (typeof cryptoApi?.getRandomValues === "function") {
    const bytes = cryptoApi.getRandomValues(new Uint8Array(16));
    return `web-${Array.from(bytes, value => value.toString(16).padStart(2, "0")).join("")}`;
  }
  return `web-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function scheduleRetry(callback: () => void, delay: number): () => void {
  let active = true;
  const timer = setTimeout(() => {
    if (!active) return;
    active = false;
    callback();
  }, delay);
  return () => {
    if (!active) return;
    active = false;
    clearTimeout(timer);
  };
}

export function stableWindowManagerClientId(): string {
  if (typeof window === "undefined") return "web-server-render";
  const created = randomClientId();
  try {
    const existing = window.localStorage.getItem(CLIENT_ID_STORAGE_KEY)?.trim();
    if (existing) return existing;
    window.localStorage.setItem(CLIENT_ID_STORAGE_KEY, created);
  } catch {
    return created;
  }
  return created;
}

export interface WindowManagerClientRegistrationState {
  clientId: string;
  registrationEpoch: number;
  client: WindowManagerClientView | null;
  status: "idle" | "registering" | "registered" | "error";
  error: Error | null;
  reregister: () => void;
}

interface RegistrationResult {
  workspaceId: string;
  client: WindowManagerClientView | null;
  status: "registering" | "registered" | "error";
  error: Error | null;
}

export function useWindowManagerClient(
  workspaceId: string | null,
  onClientChange: (client: WindowManagerClientView | null) => void,
  onErrorChange: (error: Error | null) => void = () => {}
): WindowManagerClientRegistrationState {
  const [clientId] = useState(stableWindowManagerClientId);
  const [registration, setRegistration] = useState<RegistrationResult | null>(null);
  const [attempt, setAttempt] = useState(0);
  const publishedWorkspace = useRef<string | null>(null);
  const retryCount = useRef(0);
  const cancelRetry = useRef<(() => void) | null>(null);
  const publishClient = useEffectEvent(onClientChange);
  const publishError = useEffectEvent(onErrorChange);

  useEffect(() => {
    if (workspaceId === null) {
      publishError(null);
      publishClient(null);
      publishedWorkspace.current = null;
      retryCount.current = 0;
      return undefined;
    }

    const controller = new AbortController();
    let ownedCancelRetry: (() => void) | null = null;
    if (publishedWorkspace.current !== workspaceId) {
      publishClient(null);
      publishedWorkspace.current = workspaceId;
      retryCount.current = 0;
    }
    publishError(null);

    void registerWindowManagerClient(workspaceId, clientId, undefined, controller.signal)
      .then(view => {
        if (controller.signal.aborted) return;
        if (view.workspaceId !== workspaceId || view.clientId !== clientId) {
          throw new Error("The daemon registered a different window-manager client.");
        }
        setRegistration({ workspaceId, client: view, status: "registered", error: null });
        publishError(null);
        retryCount.current = 0;
        publishClient(view);
      })
      .catch(cause => {
        if (controller.signal.aborted) return;
        const nextError =
          cause instanceof Error ? cause : new Error("Unable to register this browser client.");
        setRegistration({ workspaceId, client: null, status: "error", error: nextError });
        publishError(nextError);
        publishClient(null);
        const delay = Math.min(8_000, 500 * 2 ** Math.min(retryCount.current, 4));
        const cancel = scheduleRetry(() => {
          if (cancelRetry.current === cancel) cancelRetry.current = null;
          retryCount.current += 1;
          setRegistration({ workspaceId, client: null, status: "registering", error: null });
          setAttempt(current => current + 1);
        }, delay);
        ownedCancelRetry = cancel;
        cancelRetry.current = cancel;
      });

    return () => {
      controller.abort();
      ownedCancelRetry?.();
      if (cancelRetry.current === ownedCancelRetry) {
        cancelRetry.current = null;
      }
    };
  }, [attempt, clientId, workspaceId]);

  const current = registration?.workspaceId === workspaceId ? registration : null;
  const status = workspaceId === null ? "idle" : (current?.status ?? "registering");

  return {
    clientId,
    registrationEpoch: attempt,
    client: current?.client ?? null,
    status,
    error: current?.error ?? null,
    reregister: () => {
      cancelRetry.current?.();
      cancelRetry.current = null;
      retryCount.current = 0;
      if (workspaceId === null) return;
      setRegistration({ workspaceId, client: null, status: "registering", error: null });
      onClientChange(null);
      onErrorChange(null);
      setAttempt(current => current + 1);
    },
  };
}
