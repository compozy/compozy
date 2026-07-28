import type { QueryClient } from "@tanstack/react-query";
import { createStore, type StoreApi } from "zustand/vanilla";

import {
  executeWindowManagerCommand,
  fetchWindowManagerSnapshot,
  WindowManagerApiError,
} from "../adapters/window-manager-api";
import type { OsDesktopRuntimeStore, OsWallpaper } from "../lib/os-types";
import type { WindowManagerCommandOutcome } from "../lib/os-types";
import { effectiveWindowManagerConfig } from "../lib/window-manager-config";
import { reconcileWindowManagerSnapshot, windowManagerKeys } from "../lib/window-manager-query";
import type {
  PixelRect,
  WindowManagerClientView,
  WindowManagerCommandInput,
  WindowManagerConfig,
  WindowManagerConnectionStatus,
  WindowManagerDiagnosticPayload,
  WindowManagerSnapshot,
} from "../lib/window-manager-types";
import { DEFAULT_WINDOW_MANAGER_WORK_AREA } from "../lib/window-manager-view";
import { windowManagerStore } from "../stores/window-manager-store";

export interface WindowManagerRuntimeBinding {
  workspaceId: string;
  clientId: string;
}

export function randomWindowManagerId(prefix: string): string {
  if (typeof globalThis.crypto?.randomUUID === "function") {
    return `${prefix}-${globalThis.crypto.randomUUID()}`;
  }
  return `${prefix}-${Date.now()}-${Math.random().toString(36).slice(2)}`;
}

function commandDiagnostic(error: unknown): WindowManagerDiagnosticPayload {
  if (error instanceof WindowManagerApiError && error.payload?.diagnostics[0]) {
    return error.payload.diagnostics[0];
  }
  return {
    code:
      error instanceof WindowManagerApiError
        ? (error.payload?.code ?? "command_failed")
        : "command_failed",
    path: null,
    message: error instanceof Error ? error.message : "The window command failed.",
  };
}

/** Query/client lifecycle and semantic command transport shared by the OS runtime. */
export abstract class WindowManagerRuntimeCore {
  private unsubscribeQuery: (() => void) | null = null;
  private unsubscribePresentation: (() => void) | null = null;
  private runtimeStore: StoreApi<OsDesktopRuntimeStore> | null = null;
  protected readonly queryClient: QueryClient;
  protected binding: WindowManagerRuntimeBinding | null = null;
  protected client: WindowManagerClientView | null = null;
  protected wallpaper: OsWallpaper = "ember";
  protected reduceMotion = false;
  protected dockMagnify = true;
  protected railCollapsedAgentIds: readonly string[] = [];
  protected loadError: Error | null = null;

  constructor(queryClient: QueryClient) {
    this.queryClient = queryClient;
  }

  start(): void {
    if (this.unsubscribeQuery !== null || this.unsubscribePresentation !== null) return;
    this.unsubscribeQuery = this.queryClient.getQueryCache().subscribe(event => {
      const key = event.query.queryKey;
      const configKey = windowManagerKeys.config();
      if (
        key.length === configKey.length &&
        key.every((part: unknown, index: number) => part === configKey[index])
      ) {
        this.publish();
        return;
      }
      if (!this.binding) return;
      if (
        key[0] === windowManagerKeys.all[0] &&
        key[1] === windowManagerKeys.all[1] &&
        key[2] === "snapshot" &&
        key[3] === this.binding.workspaceId
      ) {
        this.publish();
      }
    });
    this.unsubscribePresentation = windowManagerStore.subscribe((state, previous) => {
      if (
        state.connectionStatus !== previous.connectionStatus ||
        state.workArea !== previous.workArea ||
        state.seamPreview !== previous.seamPreview
      ) {
        this.publish();
      }
    });
    this.publish();
  }

  stop(): void {
    this.unsubscribeQuery?.();
    this.unsubscribePresentation?.();
    this.unsubscribeQuery = null;
    this.unsubscribePresentation = null;
  }

  destroy(): void {
    this.stop();
  }

  protected abstract buildView(): OsDesktopRuntimeStore;

  protected initializeView(): void {
    this.runtimeStore = createStore<OsDesktopRuntimeStore>()(() => this.buildView());
  }

  private store(): StoreApi<OsDesktopRuntimeStore> {
    if (this.runtimeStore === null) {
      throw new Error("Window-manager runtime store is not initialized.");
    }
    return this.runtimeStore;
  }

  protected get view(): OsDesktopRuntimeStore {
    return this.store().getState();
  }

  getState = (): OsDesktopRuntimeStore => this.store().getState();

  getInitialState = (): OsDesktopRuntimeStore => this.store().getInitialState();

  subscribe = (listener: (state: OsDesktopRuntimeStore, previous: OsDesktopRuntimeStore) => void) =>
    this.store().subscribe(listener);

  bind(binding: WindowManagerRuntimeBinding): void {
    if (
      this.binding?.workspaceId === binding.workspaceId &&
      this.binding.clientId === binding.clientId
    ) {
      return;
    }
    this.binding = { ...binding };
    this.client = null;
    this.loadError = null;
    windowManagerStore.getState().actions.bindClient(binding);
    this.publish();
  }

  unbind(): void {
    this.binding = null;
    this.client = null;
    this.loadError = null;
    windowManagerStore.getState().actions.unbindClient();
    this.publish();
  }

  setClient(client: WindowManagerClientView | null): void {
    if (client === null) {
      if (this.client === null) return;
      this.client = null;
      this.publish();
      return;
    }
    const binding = this.binding;
    if (
      binding === null ||
      client.workspaceId !== binding.workspaceId ||
      client.clientId !== binding.clientId ||
      (this.client !== null && client.presentationRevision <= this.client.presentationRevision)
    ) {
      return;
    }
    const previous = this.client;
    this.client = client;
    const transition = windowManagerStore.getState().transitionIntent;
    if (transition?.mode === "instant" && client.activeDesktopId === transition.toDesktopId) {
      windowManagerStore.getState().actions.setTransitionIntent(null);
    } else if (
      previous !== null &&
      previous.activeDesktopId !== client.activeDesktopId &&
      transition === null
    ) {
      // Reconciled desktop changes — zoom, restore, cross-desktop focus, remote
      // switches — synthesize a transition only when no optimistic intent is
      // outstanding; a queued desktop.switch already staged its target, and an
      // intermediate result must not overwrite it.
      this.synthesizeDesktopTransition(previous.activeDesktopId, client.activeDesktopId);
    }
    this.publish();
  }

  private synthesizeDesktopTransition(fromDesktopId: string, toDesktopId: string): void {
    const snapshot = this.snapshot();
    const globalConfig = this.config();
    if (snapshot === null || globalConfig === null) return;
    const config = effectiveWindowManagerConfig(globalConfig, snapshot.overrides);
    if (this.reduceMotion || config.desktopTransition === "instant") return;
    windowManagerStore.getState().actions.setTransitionIntent({
      fromDesktopId,
      toDesktopId,
      direction: this.desktopTransitionDirection(snapshot.desktops, fromDesktopId, toDesktopId),
      mode: config.desktopTransition,
    });
  }

  protected desktopTransitionDirection(
    desktops: readonly { id: string; order: number }[],
    fromDesktopId: string,
    toDesktopId: string
  ): "earlier" | "later" {
    const fromOrder = desktops.find(desktop => desktop.id === fromDesktopId)?.order ?? 0;
    const toOrder = desktops.find(desktop => desktop.id === toDesktopId)?.order ?? 0;
    return toOrder >= fromOrder ? "later" : "earlier";
  }

  setConnectionStatus(status: WindowManagerConnectionStatus): void {
    windowManagerStore.getState().actions.setConnectionStatus(status);
  }

  setLoadError(error: Error | null): void {
    this.loadError = error;
    this.publish();
  }

  clearConflict(): void {
    windowManagerStore.getState().actions.clearConflict();
  }

  refreshSnapshot(): void {
    const binding = this.binding;
    if (binding === null) return;
    void fetchWindowManagerSnapshot(binding.workspaceId)
      .then(snapshot => {
        this.queryClient.setQueryData<WindowManagerSnapshot>(
          windowManagerKeys.snapshot(binding.workspaceId),
          current => reconcileWindowManagerSnapshot(current, snapshot)
        );
        this.setLoadError(null);
      })
      .catch(error => {
        this.setLoadError(
          error instanceof Error ? error : new Error("Unable to reload the window layout.")
        );
      });
  }

  protected publish(): void {
    this.runtimeStore?.setState(this.buildView(), true);
  }

  protected snapshot(): WindowManagerSnapshot | null {
    if (!this.binding) return null;
    return (
      this.queryClient.getQueryData<WindowManagerSnapshot>(
        windowManagerKeys.snapshot(this.binding.workspaceId)
      ) ?? null
    );
  }

  protected config(): WindowManagerConfig | null {
    return this.queryClient.getQueryData<WindowManagerConfig>(windowManagerKeys.config()) ?? null;
  }

  protected currentLoadError(): Error | null {
    const snapshotError = this.binding
      ? this.queryClient.getQueryState(windowManagerKeys.snapshot(this.binding.workspaceId))?.error
      : null;
    const configError = this.queryClient.getQueryState(windowManagerKeys.config())?.error;
    if (snapshotError instanceof Error) return snapshotError;
    if (configError instanceof Error) return configError;
    return this.loadError;
  }

  protected workArea(): PixelRect {
    return windowManagerStore.getState().workArea?.rect ?? DEFAULT_WINDOW_MANAGER_WORK_AREA;
  }

  private reportClientUnavailable(): void {
    windowManagerStore.getState().actions.reportDiagnostic({
      code: "client_unavailable",
      message: "Window commands are unavailable until this browser reconnects.",
      severity: "warning",
      field: null,
    });
    this.publish();
  }

  private commandChain: Promise<unknown> = Promise.resolve();

  private startDispatch(command: WindowManagerCommandInput): Promise<boolean> | null {
    const binding = this.binding;
    if (binding === null || this.snapshot() === null || this.client === null) {
      this.reportClientUnavailable();
      return null;
    }
    // Rapid interactions (zoom toggle, dock activations, seam arrows) queue
    // behind the in-flight command instead of being silently dropped; each
    // queued command reads a fresh snapshot revision when it runs.
    const run = () => this.runCommand(command, { ...binding });
    const chained = this.commandChain.then(run, run);
    this.commandChain = chained;
    return chained;
  }

  private runCommand(
    command: WindowManagerCommandInput,
    enqueuedBinding: WindowManagerRuntimeBinding
  ): Promise<boolean> {
    const binding = this.binding;
    if (binding === null) {
      this.reportClientUnavailable();
      return Promise.resolve(false);
    }
    if (
      binding.workspaceId !== enqueuedBinding.workspaceId ||
      binding.clientId !== enqueuedBinding.clientId
    ) {
      return Promise.resolve(false);
    }
    const snapshot = this.snapshot();
    if (snapshot === null || this.client === null) {
      this.reportClientUnavailable();
      return Promise.resolve(false);
    }

    const requestId = randomWindowManagerId("wm-command");
    const actions = windowManagerStore.getState().actions;
    if (
      !actions.beginCommand({
        id: requestId,
        kind: command.commandId,
        expectedRevision: snapshot.revision,
      })
    ) {
      // A recorded revision conflict keeps the surface read-only until the
      // user resolves it; queued commands resolve unapplied instead of racing.
      this.clearSwitchTransition(command);
      return Promise.resolve(false);
    }

    return executeWindowManagerCommand(
      binding.workspaceId,
      binding.clientId,
      snapshot.revision,
      command
    )
      .then(result => {
        this.queryClient.setQueryData<WindowManagerSnapshot>(
          windowManagerKeys.snapshot(binding.workspaceId),
          current => reconcileWindowManagerSnapshot(current, result.snapshot)
        );
        if (result.client !== null) this.setClient(result.client);
        const firstDiagnostic = result.diagnostics[0];
        actions.completeCommand(
          requestId,
          firstDiagnostic
            ? {
                code: firstDiagnostic.code,
                message: firstDiagnostic.message,
                severity: "warning",
                field: firstDiagnostic.path,
              }
            : undefined
        );
        this.publish();
        return result.applied;
      })
      .catch(error => {
        const currentBinding = windowManagerStore.getState().binding;
        if (
          currentBinding?.workspaceId === binding.workspaceId &&
          currentBinding.clientId === binding.clientId
        ) {
          this.clearSwitchTransition(command);
        }
        const diagnostic = commandDiagnostic(error);
        const storeDiagnostic = {
          code: diagnostic.code,
          message: diagnostic.message,
          severity: "error" as const,
          field: diagnostic.path,
        };
        if (
          error instanceof WindowManagerApiError &&
          error.status === 409 &&
          error.payload?.currentRevision !== null
        ) {
          actions.recordConflict(
            {
              commandId: requestId,
              expectedRevision: snapshot.revision,
              currentRevision: error.payload?.currentRevision ?? snapshot.revision,
            },
            storeDiagnostic
          );
        } else {
          actions.failCommand(requestId, storeDiagnostic);
        }
        this.publish();
        return false;
      });
  }

  /**
   * Drops the optimistic transition of a failed desktop.switch, but only when
   * the live intent still targets this command's desktop — a newer queued
   * switch may have replaced it.
   */
  private clearSwitchTransition(command: WindowManagerCommandInput): void {
    if (command.commandId !== "desktop.switch") return;
    const intent = windowManagerStore.getState().transitionIntent;
    const target = command.payload.desktop_id;
    if (intent !== null && typeof target === "string" && intent.toDesktopId === target) {
      windowManagerStore.getState().actions.setTransitionIntent(null);
    }
  }

  protected dispatch(command: WindowManagerCommandInput): WindowManagerCommandOutcome {
    const pending = this.startDispatch(command);
    return pending === null
      ? { accepted: false, completion: Promise.resolve(false) }
      : { accepted: true, completion: pending };
  }
}
