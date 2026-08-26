import type { QueryClient } from "@tanstack/react-query";
import { shallowEqual } from "@xstate/store";

import { executeWindowManagerCommand } from "../adapters/window-manager-api";
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
  WindowManagerRegisteredClientView,
  WindowManagerCommandInput,
  WindowManagerConfig,
  WindowManagerConnectionStatus,
  WindowManagerSnapshot,
} from "../lib/window-manager-types";
import { DEFAULT_WINDOW_MANAGER_WORK_AREA } from "../lib/window-manager-view";
import { windowManagerStore, type DesktopTransitionIntent } from "../stores/window-manager-store";
import { beginWindowManagerCommand } from "../stores/window-manager-store-commands";
import {
  clearSwitchTransition,
  reportClientUnavailable,
  reportCommandCompleted,
  reportCommandRefused,
} from "./window-manager-command-outcome";
import {
  queryCacheEventChangesData,
  randomWindowManagerId,
  WINDOW_MANAGER_DIAGNOSTIC_TTL_MS,
} from "./window-manager-runtime-helpers";
import { WindowManagerSnapshotRefresher } from "./window-manager-snapshot-refresher";

export interface WindowManagerRuntimeBinding {
  workspaceId: string;
  /** The profile whose desks this runtime presents; a switch rebinds (US-026). */
  profileId: string;
  clientId: string;
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
  protected clientAttachmentToken: string | null = null;
  protected wallpaper: OsWallpaper = "ember";
  protected reduceMotion = false;
  protected dockMagnify = true;
  protected loadError: Error | null = null;
  private diagnosticTimer: ReturnType<typeof setTimeout> | null = null;
  private conflictRecovery: Promise<boolean> | null = null;

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
        key[3] === this.binding.workspaceId &&
        key[4] === this.binding.profileId
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
    this.cancelDiagnosticExpiry();
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
      this.binding.profileId === binding.profileId &&
      this.binding.clientId === binding.clientId
    ) {
      return;
    }
    this.snapshotRefresher.reset();
    this.binding = { ...binding };
    this.client = null;
    this.clientAttachmentToken = null;
    this.loadError = null;
    windowManagerStore.trigger.bindingBound({ binding });
    this.publish();
  }

  unbind(): void {
    this.snapshotRefresher.reset();
    this.binding = null;
    this.client = null;
    this.clientAttachmentToken = null;
    this.loadError = null;
    windowManagerStore.trigger.bindingUnbound();
    this.publish();
  }

  setClient(client: WindowManagerClientView | WindowManagerRegisteredClientView | null): void {
    if (client === null) {
      if (this.client === null && this.clientAttachmentToken === null) return;
      this.client = null;
      this.clientAttachmentToken = null;
      this.publish();
      return;
    }
    const binding = this.binding;
    if (
      binding === null ||
      client.workspaceId !== binding.workspaceId ||
      client.clientId !== binding.clientId
    ) {
      return;
    }
    const previousToken = this.clientAttachmentToken;
    if ("attachmentToken" in client) {
      this.clientAttachmentToken = client.attachmentToken;
    }
    if (this.client !== null && client.presentationRevision <= this.client.presentationRevision) {
      if (previousToken !== this.clientAttachmentToken) this.publish();
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

  /**
   * A revision conflict means another client or agent moved the layout first.
   * The surface re-reads the snapshot and reopens for commands on its own; the
   * refused action rolls back and the notice explains what happened.
   */
  private recoverFromConflict(): Promise<boolean> {
    if (this.conflictRecovery !== null) return this.conflictRecovery;
    const commandState = windowManagerStore.getSnapshot().context.commandState;
    // The daemon reporting a revision behind ours means it restarted from a
    // discarded or replaced arrangement; its re-read replaces the cache.
    const authoritative =
      commandState.status === "conflict" &&
      commandState.conflict.currentRevision < (this.view.snapshot?.revision ?? 0);
    this.conflictRecovery = this.refreshSnapshot({ authoritative })
      .then(refreshed => {
        if (!refreshed) return false;
        const commandState = windowManagerStore.getSnapshot().context.commandState;
        if (commandState.status === "conflict") {
          const diagnostic = commandState.diagnostic;
          this.clearConflict();
          windowManagerStore.trigger.diagnosticReported({ diagnostic });
          this.scheduleDiagnosticExpiry();
        }
        return refreshed;
      })
      .finally(() => {
        this.conflictRecovery = null;
        this.publish();
      });
    return this.conflictRecovery;
  }

  /** Resolves once the surface has re-read the layout after a revision conflict. */
  protected awaitConflictRecovery(): Promise<boolean> {
    if (windowManagerStore.getSnapshot().context.commandState.status !== "conflict") {
      return Promise.resolve(true);
    }
    return this.recoverFromConflict();
  }

  private scheduleDiagnosticExpiry(): void {
    this.cancelDiagnosticExpiry();
    this.diagnosticTimer = setTimeout(() => {
      this.diagnosticTimer = null;
      windowManagerStore.trigger.diagnosticCleared();
      this.publish();
    }, WINDOW_MANAGER_DIAGNOSTIC_TTL_MS);
  }

  private cancelDiagnosticExpiry(): void {
    if (this.diagnosticTimer === null) return;
    clearTimeout(this.diagnosticTimer);
    this.diagnosticTimer = null;
  }

  async refreshSnapshot(options: { authoritative?: boolean } = {}): Promise<boolean> {
    const binding = this.binding;
    if (binding === null) return false;
    const result = await this.snapshotRefresher.refresh(
      binding.workspaceId,
      binding.profileId,
      () => this.binding === binding
    );
    if (result.status === "stale") return false;
    if (result.status === "failed") {
      this.setLoadError(result.error);
      return false;
    }
    this.queryClient.setQueryData<WindowManagerSnapshot>(
      windowManagerKeys.snapshot(binding.workspaceId, binding.profileId),
      current =>
        options.authoritative
          ? result.snapshot
          : reconcileWindowManagerSnapshot(current, result.snapshot)
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
        windowManagerKeys.snapshot(this.binding.workspaceId, this.binding.profileId)
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
      ? this.queryClient.getQueryState(
          windowManagerKeys.snapshot(this.binding.workspaceId, this.binding.profileId)
        )?.error
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
    reportClientUnavailable();
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
      binding.profileId !== enqueuedBinding.profileId ||
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
      clearSwitchTransition(command, binding);
      return Promise.resolve(false);
    }

    return executeWindowManagerCommand(
      binding.workspaceId,
      binding.profileId,
      binding.clientId,
      command.expectedRevision ?? snapshot.revision,
      command
    )
      .then(result => {
        this.projectionDeferred = true;
        try {
          this.queryClient.setQueryData<WindowManagerSnapshot>(
            windowManagerKeys.snapshot(binding.workspaceId, binding.profileId),
            current => reconcileWindowManagerSnapshot(current, result.snapshot)
          );
          if (result.client !== null) this.setClient(result.client);
        } finally {
          this.projectionDeferred = false;
          this.publish();
        }
        reportCommandCompleted(requestId, result, binding);
        this.publish();
        return result.applied;
      })
      .catch(error => {
        clearSwitchTransition(command, binding);
        reportCommandRefused(
          requestId,
          error,
          command.expectedRevision ?? snapshot.revision,
          binding
        );
        if (windowManagerStore.getSnapshot().context.commandState.status === "conflict") {
          void this.recoverFromConflict();
        } else {
          this.scheduleDiagnosticExpiry();
        }
        this.publish();
        return false;
      });
  }

  protected dispatch(command: WindowManagerCommandInput): WindowManagerCommandOutcome {
    const pending = this.startDispatch(command);
    return pending === null
      ? { accepted: false, completion: Promise.resolve(false) }
      : { accepted: true, completion: pending };
  }
}
