import { useEffect, useState } from "react";
import { useSelector } from "@xstate/store-react";

import { useStoreBinding } from "@/hooks/use-store-binding";
import { useDocumentVisible } from "@/hooks/use-document-visible";

import { registerWindowManagerClient } from "../adapters/window-manager-api";
import { isDesktopShell } from "../lib/desktop-shell-bridge";
import { stableWindowManagerClientId } from "../lib/window-manager-client-identity";
import type { WindowManagerAttachedClientView } from "../lib/window-manager-types";
import {
  windowManagerClientRegistrationLogic,
  windowManagerRetryDelay,
} from "./window-manager-client-registration-store";

export interface WindowManagerClientRegistrationState {
  clientId: string;
  registrationEpoch: number;
  client: WindowManagerAttachedClientView | null;
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
    const kind = isDesktopShell() ? "shell" : "browser";
    void registerWindowManagerClient(
      selectedWorkspaceId,
      clientId,
      undefined,
      kind,
      controller.signal
    )
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
