import { useEffect, useState } from "react";
import { useSelector } from "@xstate/store-react";

import { useStoreBinding } from "@/hooks/use-store-binding";
import { useDocumentVisible } from "@/hooks/use-document-visible";

import { registerWindowManagerClient, WindowManagerApiError } from "../adapters/window-manager-api";
import { isDesktopShell } from "../lib/desktop-shell-bridge";
import { stableWindowManagerClientId } from "../lib/window-manager-client-identity";
import type {
  WindowManagerClientView,
  WindowManagerRegisteredClientView,
} from "../lib/window-manager-types";
import {
  type WindowManagerClientRestore,
  windowManagerClientRegistrationLogic,
  windowManagerRetryDelay,
} from "./window-manager-client-registration-store";

interface ClientScope {
  workspaceId: string;
  profileId: string;
}

function clientScope(workspaceId: string | null, profileId: string | null): ClientScope | null {
  if (workspaceId === null || profileId === null) return null;
  return { workspaceId, profileId };
}

function bindingScope(scope: ClientScope | null): string | null {
  return scope === null ? null : `${scope.workspaceId}\u0000${scope.profileId}`;
}

/** What a lost registration restores: the client view this page was showing (#1655). */
export function clientRestore(
  view: Pick<WindowManagerClientView, "activeDesktopId" | "focusedWindowId"> | null
): WindowManagerClientRestore | null {
  if (view === null) return null;
  return { activeDesktopId: view.activeDesktopId, focusedWindowId: view.focusedWindowId };
}

function refusedDesktopHint(cause: unknown): boolean {
  return (
    cause instanceof WindowManagerApiError &&
    cause.payload?.code === "window_manager_desktop_not_found"
  );
}

export interface WindowManagerClientRegistrationState {
  clientId: string;
  registrationEpoch: number;
  client: WindowManagerRegisteredClientView | null;
  status: "idle" | "registering" | "registered" | "error";
  error: Error | null;
  /**
   * The window this page had focused before its registration was lost, while the registration that
   * restores it has not settled it yet (`restoreSettled`); `null` otherwise.
   */
  restoredFocus: string | null;
  /**
   * Registers again. `restore` names the view this page was showing, which the new registration
   * asks for (its desktop) and `restoredFocus` reports (its window).
   */
  reregister: (restore?: WindowManagerClientRestore | null) => void;
  restoreSettled: (windowId: string) => void;
}

export function useWindowManagerClient(
  workspaceId: string | null,
  profileId: string | null
): WindowManagerClientRegistrationState {
  const [clientId] = useState(stableWindowManagerClientId);
  const documentVisible = useDocumentVisible();
  const scope = clientScope(workspaceId, profileId);
  // The registration belongs to one (workspace, profile) pair: switching either
  // rebinds the store, which re-registers this client against the desks it is
  // about to present. Registering is all this side does — the daemon's claim moves
  // the attachment, so no unregister races the switch it follows (US-026).
  const { store } = useStoreBinding(bindingScope(scope), () =>
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
  const restore = useSelector(store, snapshot => snapshot.context.restore);
  const desktopHint = restore?.activeDesktopId ?? undefined;

  useEffect(() => {
    store.trigger.documentVisibilityChanged({ visible: documentVisible });
  }, [documentVisible, store]);

  useEffect(() => {
    if (phase !== "registering" || selectedWorkspaceId === null) return undefined;
    const controller = new AbortController();
    const kind = isDesktopShell() ? "shell" : "browser";
    if (profileId === null) return undefined;
    // Only a re-registration in this page carries a desktop hint; the first sends none.
    void registerWindowManagerClient(
      selectedWorkspaceId,
      profileId,
      clientId,
      desktopHint,
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
        // The desktop is gone: stay on the daemon's default, with no error surface.
        if (desktopHint !== undefined && refusedDesktopHint(cause)) {
          store.trigger.desktopHintRefused({ epoch });
          return;
        }
        store.trigger.registrationFailed({
          epoch,
          error:
            cause instanceof Error ? cause : new Error("Unable to register this browser client."),
        });
      });
    return () => controller.abort();
  }, [clientId, desktopHint, epoch, phase, profileId, selectedWorkspaceId, store]);

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
    restoredFocus: phase === "registered" ? (restore?.focusedWindowId ?? null) : null,
    reregister: next =>
      store.trigger.recoveryRequested(next === undefined ? {} : { restore: next }),
    restoreSettled: windowId => store.trigger.restoreSettled({ windowId }),
  };
}
