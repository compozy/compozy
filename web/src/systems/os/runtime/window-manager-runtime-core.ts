import type { QueryCacheNotifyEvent, QueryClient } from "@tanstack/react-query";
import { shallowEqual } from "@xstate/store";

import { executeWindowManagerCommand, WindowManagerApiError } from "../adapters/window-manager-api";
import type { OsDesktopRuntimeStore, OsWallpaper } from "../lib/os-types";
import type { WindowManagerCommandOutcome } from "../lib/os-types";
import { effectiveWindowManagerConfig } from "../lib/window-manager-config";
import { reconcileWindowManagerSnapshot, windowManagerKeys } from "../lib/window-manager-query";
import type { WindowManagerSettingsSection } from "../lib/window-manager-settings-section";
import {
  createWindowManagerProjectionAtom,
  type WindowManagerProjectionAtom,
} from "../lib/window-manager-projection";
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
import { windowManagerStore, type DesktopTransitionIntent } from "../stores/window-manager-store";
import { beginWindowManagerCommand } from "../stores/window-manager-store-commands";
import { WindowManagerSnapshotRefresher } from "./window-manager-snapshot-refresher";

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

function queryCacheEventChangesData(event: QueryCacheNotifyEvent): boolean {
  switch (event.type) {
    case "added":
      return event.query.state.data !== undefined;
    case "removed":
      return true;
    case "updated":
      return (
        event.action.type === "success" ||
        (event.action.type === "setState" && Object.hasOwn(event.action.state, "data"))
      );
    case "observerAdded":
    case "observerRemoved":
    case "observerResultsUpdated":
    case "observerOptionsUpdated":
      return false;
  }
}

/** Query/client lifecycle and semantic command transport shared by the OS runtime. */
export abstract class WindowManagerRuntimeCore {
  private unsubscribeQuery: (() => void) | null = null;
  private unsubscribePresentation: (() => void) | null = null;
  private runtimeProjection: WindowManagerProjectionAtom | null = null;
  private initialView: OsDesktopRuntimeStore | null = null;
  private projectionDeferred = false;
  private readonly snapshotRefresher = new WindowManagerSnapshotRefresher();
  protected readonly queryClient: QueryClient;
  protected binding: WindowManagerRuntimeBinding | null = null;
  protected client: WindowManagerClientView | null = null;
  protected wallpaper: OsWallpaper = "ember";
  protected reduceMotion = false;
  protected dockMagnify = true;
  protected loadError: Error | null = null;

  constructor(queryClient: QueryClient) {
    this.queryClient = queryClient;
  }

  start(): void {
    if (this.unsubscribeQuery !== null || this.unsubscribePresentation !== null) return;
    this.unsubscribeQuery = this.queryClient.getQueryCache().subscribe(event => {
      if (!queryCacheEventChangesData(event)) return;
      const key = event.query.queryKey;
      const configKey = this.configKey();
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
    const presentation = windowManagerStore.select(
      state => ({
        connectionStatus: state.connectionStatus,
        workArea: state.workArea,
        seamPreview: state.seamPreview,
      }),
      shallowEqual
    );
    const subscription = presentation.subscribe(() => this.publish());
    this.unsubscribePresentation = () => subscription.unsubscribe();
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
    const initial = this.buildView();
    this.initialView = initial;
    this.runtimeProjection = createWindowManagerProjectionAtom(initial);
  }

  private projection(): WindowManagerProjectionAtom {
    if (this.runtimeProjection === null) {
      throw new Error("Window-manager runtime projection is not initialized.");
    }
    return this.runtimeProjection;
  }

  protected get view(): OsDesktopRuntimeStore {
    return this.projection().get();
  }

  get projectionAtom(): WindowManagerProjectionAtom {
    return this.projection();
  }

  getState = (): OsDesktopRuntimeStore => this.projection().get();

  getInitialState = (): OsDesktopRuntimeStore => {
    if (this.initialView === null) {
      throw new Error("Window-manager runtime projection is not initialized.");
    }
    return this.initialView;
  };

  subscribe = (
    listener: (state: OsDesktopRuntimeStore, previous: OsDesktopRuntimeStore) => void
  ) => {
    let previous = this.getState();
    const subscription = this.projection().subscribe(current => {
      listener(current, previous);
      previous = current;
    });
    return () => subscription.unsubscribe();
  };

  bind(binding: WindowManagerRuntimeBinding): void {
    if (
      this.binding?.workspaceId === binding.workspaceId &&
      this.binding.clientId === binding.clientId
    ) {
      return;
    }
    this.snapshotRefresher.reset();
    this.binding = { ...binding };
    this.client = null;
    this.loadError = null;
    windowManagerStore.trigger.bindingBound({ binding });
    this.publish();
  }

  unbind(): void {
    this.snapshotRefresher.reset();
    this.binding = null;
    this.client = null;
    this.loadError = null;
    windowManagerStore.trigger.bindingUnbound();
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
    windowManagerStore.trigger.desktopStateObserved({
      activeDesktopId: client.activeDesktopId,
      reconciledIntent:
        previous !== null && previous.activeDesktopId !== client.activeDesktopId
          ? this.desktopTransitionIntent(previous.activeDesktopId, client.activeDesktopId)
          : null,
    });
    this.publish();
  }

  private desktopTransitionIntent(
    fromDesktopId: string,
    toDesktopId: string
  ): DesktopTransitionIntent | null {
    const snapshot = this.snapshot();
    const globalConfig = this.config();
    if (snapshot === null || globalConfig === null) return null;
    const config = effectiveWindowManagerConfig(globalConfig, snapshot.overrides);
    if (this.reduceMotion || config.desktopTransition === "instant") return null;
    return {
      fromDesktopId,
      toDesktopId,
      direction: this.desktopTransitionDirection(snapshot.desktops, fromDesktopId, toDesktopId),
      mode: config.desktopTransition,
    };
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
    windowManagerStore.trigger.connectionStatusChanged({ status });
  }

  setLoadError(error: Error | null): void {
    this.loadError = error;
    this.publish();
  }

  clearConflict(): void {
    windowManagerStore.trigger.conflictCleared();
  }

  async refreshSnapshot(): Promise<boolean> {
    const binding = this.binding;
    if (binding === null) return false;
    const result = await this.snapshotRefresher.refresh(
      binding.workspaceId,
      () => this.binding === binding
    );
    if (result.status === "stale") return false;
    if (result.status === "failed") {
      this.setLoadError(result.error);
      return false;
    }
    this.queryClient.setQueryData<WindowManagerSnapshot>(
      windowManagerKeys.snapshot(binding.workspaceId),
      current => reconcileWindowManagerSnapshot(current, result.snapshot)
    );
    this.setLoadError(null);
    return true;
  }

  protected publish(): void {
    if (this.projectionDeferred) return;
    this.runtimeProjection?.set(this.buildView());
  }

  protected snapshot(): WindowManagerSnapshot | null {
    if (!this.binding) return null;
    return (
      this.queryClient.getQueryData<WindowManagerSnapshot>(
        windowManagerKeys.snapshot(this.binding.workspaceId)
      ) ?? null
    );
  }

  private configKey() {
    const binding = this.binding;
    return binding === null
      ? windowManagerKeys.config()
      : windowManagerKeys.config(binding.workspaceId, binding.clientId);
  }

  protected config(): WindowManagerConfig | null {
    return (
      this.queryClient.getQueryData<WindowManagerSettingsSection>(this.configKey())?.config ?? null
    );
  }

  protected currentLoadError(): Error | null {
    const snapshotError = this.binding
      ? this.queryClient.getQueryState(windowManagerKeys.snapshot(this.binding.workspaceId))?.error
      : null;
    const configError = this.queryClient.getQueryState(this.configKey())?.error;
    if (snapshotError instanceof Error) return snapshotError;
    if (configError instanceof Error) return configError;
    return this.loadError;
  }

  protected workArea(): PixelRect {
    return (
      windowManagerStore.getSnapshot().context.workArea?.rect ?? DEFAULT_WINDOW_MANAGER_WORK_AREA
    );
  }

  private reportClientUnavailable(): void {
    windowManagerStore.trigger.diagnosticReported({
      diagnostic: {
        code: "client_unavailable",
        message: "Window commands are unavailable until this browser reconnects.",
        severity: "warning",
        field: null,
      },
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
    if (
      !beginWindowManagerCommand(windowManagerStore, {
        id: requestId,
        kind: command.commandId,
        expectedRevision: snapshot.revision,
      })
    ) {
      // A recorded revision conflict keeps the surface read-only until the
      // user resolves it; queued commands resolve unapplied instead of racing.
      this.clearSwitchTransition(command, binding);
      return Promise.resolve(false);
    }

    return executeWindowManagerCommand(
      binding.workspaceId,
      binding.clientId,
      snapshot.revision,
      command
    )
      .then(result => {
        this.projectionDeferred = true;
        try {
          this.queryClient.setQueryData<WindowManagerSnapshot>(
            windowManagerKeys.snapshot(binding.workspaceId),
            current => reconcileWindowManagerSnapshot(current, result.snapshot)
          );
          if (result.client !== null) this.setClient(result.client);
        } finally {
          this.projectionDeferred = false;
          this.publish();
        }
        const firstDiagnostic = result.diagnostics[0];
        windowManagerStore.trigger.commandCompleted({
          commandId: requestId,
          ...(firstDiagnostic
            ? {
                diagnostic: {
                  code: firstDiagnostic.code,
                  message: firstDiagnostic.message,
                  severity: "warning" as const,
                  field: firstDiagnostic.path,
                },
              }
            : {}),
          binding,
        });
        this.publish();
        return result.applied;
      })
      .catch(error => {
        this.clearSwitchTransition(command, binding);
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
          windowManagerStore.trigger.conflictRecorded({
            conflict: {
              commandId: requestId,
              expectedRevision: snapshot.revision,
              currentRevision: error.payload?.currentRevision ?? snapshot.revision,
            },
            diagnostic: storeDiagnostic,
            binding,
          });
        } else {
          windowManagerStore.trigger.commandFailed({
            commandId: requestId,
            diagnostic: storeDiagnostic,
            binding,
          });
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
  private clearSwitchTransition(
    command: WindowManagerCommandInput,
    binding: WindowManagerRuntimeBinding
  ): void {
    if (command.commandId !== "desktop.switch") return;
    const target = command.payload.desktop_id;
    if (typeof target !== "string") return;
    windowManagerStore.trigger.transitionIntentRejected({ binding, toDesktopId: target });
  }

  protected dispatch(command: WindowManagerCommandInput): WindowManagerCommandOutcome {
    const pending = this.startDispatch(command);
    return pending === null
      ? { accepted: false, completion: Promise.resolve(false) }
      : { accepted: true, completion: pending };
  }
}
