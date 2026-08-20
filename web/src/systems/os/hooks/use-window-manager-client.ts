import { useEffect, useState } from "react";
import { useSelector } from "@xstate/store-react";

import { useStoreBinding } from "@/hooks/use-store-binding";
import { useDocumentVisible } from "@/hooks/use-document-visible";

import { registerWindowManagerClient } from "../adapters/window-manager-api";
import type { WindowManagerClientView } from "../lib/window-manager-types";
import {
  windowManagerClientRegistrationLogic,
  windowManagerRetryDelay,
} from "./window-manager-client-registration-store";

const CLIENT_ID_STORAGE_KEY = "compozy.window-manager.client-id";

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

export function useWindowManagerClient(
  workspaceId: string | null
): WindowManagerClientRegistrationState {
  const [clientId] = useState(stableWindowManagerClientId);
  const documentVisible = useDocumentVisible();
  const { store } = useStoreBinding(workspaceId, () =>
    windowManagerClientRegistrationLogic.createStore({
      clientId,
      documentVisible,
      workspaceId,
    })
  );
  const client = useSelector(store, snapshot => snapshot.context.client);
  const epoch = useSelector(store, snapshot => snapshot.context.epoch);
  const error = useSelector(store, snapshot => snapshot.context.error);
  const phase = useSelector(store, snapshot => snapshot.context.phase);
  const retryCount = useSelector(store, snapshot => snapshot.context.retryCount);
  const selectedWorkspaceId = useSelector(store, snapshot => snapshot.context.workspaceId);

  useEffect(() => {
    store.trigger.documentVisibilityChanged({ visible: documentVisible });
  }, [documentVisible, store]);

  useEffect(() => {
    if (phase !== "registering" || selectedWorkspaceId === null) return undefined;
    const controller = new AbortController();
    void registerWindowManagerClient(selectedWorkspaceId, clientId, undefined, controller.signal)
      .then(view => {
        if (controller.signal.aborted) return;
        if (view.workspaceId !== selectedWorkspaceId || view.clientId !== clientId) {
          throw new Error("CompozyOS registered a different window-manager client.");
        }
        store.trigger.registrationSucceeded({ client: view, epoch });
      })
      .catch(cause => {
        if (controller.signal.aborted) return;
        store.trigger.registrationFailed({
          epoch,
          error:
            cause instanceof Error ? cause : new Error("Unable to register this browser client."),
        });
      });
    return () => controller.abort();
  }, [clientId, epoch, phase, selectedWorkspaceId, store]);

  useEffect(() => {
    if (phase !== "waiting-retry") return undefined;
    const timer = setTimeout(
      () => store.trigger.retryElapsed(),
      windowManagerRetryDelay(retryCount)
    );
    return () => clearTimeout(timer);
  }, [phase, retryCount, store]);

  const status =
    phase === "idle" || phase === "registering" || phase === "registered" ? phase : "error";
  return {
    clientId,
    registrationEpoch: epoch,
    client,
    status,
    error,
    reregister: () => store.trigger.recoveryRequested(),
  };
}
