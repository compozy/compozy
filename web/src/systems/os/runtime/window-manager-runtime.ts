import type { QueryClient } from "@tanstack/react-query";

import { focusWindowCommand, openWindowCommand } from "../lib/window-manager-command-builders";
import { mruWindowInstance, randomOsWindowId } from "../lib/window-instance-lookup";
import { directionalFocusTarget } from "../lib/window-manager-navigation";
import type {
  OsDesktopBounds,
  OsDesktopRuntime,
  OsDesktopRuntimeStore,
  OsOpenTarget,
  OsWallpaper,
  WindowManagerCommandOutcome,
  WindowManagerOpenOutcome,
} from "../lib/os-types";
import type { FocusDirection, FocusWindowOptions } from "../lib/window-manager-types";
import { sameOsWindowRoute } from "../lib/window-manager-route";
import { buildOsDesktopRuntimeView } from "../lib/window-manager-view";
import { windowManagerStore } from "../stores/window-manager-store";
import type { WindowManagerRuntimeBinding } from "./window-manager-runtime-core";
import { WindowManagerDesktopRuntime } from "./window-manager-desktop-commands";
import { WindowManagerSemanticOpenCoordinator } from "./window-manager-semantic-open";

export class WindowManagerRuntime extends WindowManagerDesktopRuntime implements OsDesktopRuntime {
  private readonly semanticOpens: WindowManagerSemanticOpenCoordinator;

  constructor(queryClient: QueryClient) {
    super(queryClient);
    this.initializeView();
    this.semanticOpens = new WindowManagerSemanticOpenCoordinator({
      bindingKey: () => JSON.stringify(this.binding) ?? "unbound",
      getState: () => this.view,
      navigate: (id, route, mode) => this.navigateWindow(id, route, mode),
      open: target => this.openOrFocusAttempt(target, true),
    });
  }

  override bind(binding: WindowManagerRuntimeBinding): void {
    const changed =
      this.binding?.workspaceId !== binding.workspaceId ||
      this.binding.profileId !== binding.profileId ||
      this.binding.clientId !== binding.clientId;
    super.bind(binding);
    if (changed) this.semanticOpens.reset();
  }

  override unbind(): void {
    const changed = this.binding !== null;
    super.unbind();
    if (changed) this.semanticOpens.reset();
  }

  /** Moves focus between the windows on screen in the active tab, by projected geometry. */
  focusDirection(direction: FocusDirection): void {
    const config = this.view.windowManagerConfig;
    const activeDesktopId = this.view.activeDesktopId;
    if (config === null || activeDesktopId === null) return;
    const tabId = this.view.activeTabIds[activeDesktopId];
    const projection = tabId === undefined ? undefined : this.view.projections[tabId];
    if (projection === undefined) return;
    const target = directionalFocusTarget({
      candidates: projection.windows.flatMap(projected =>
        projected.shown ? [{ id: projected.windowId, rect: projected.rect }] : []
      ),
      focusedId: this.view.focusedId,
      direction,
      wrap: config.focusWrap,
    });
    if (target === null || target === this.view.focusedId) return;
    this.dispatch(focusWindowCommand(target, { replaceable: true }));
  }

  undoLayout(): void {
    this.dispatch({ commandId: "layout.undo", payload: {} });
  }

  redoLayout(): void {
    this.dispatch({ commandId: "layout.redo", payload: {} });
  }

  /** Evens every split in the focused window's tab; a single-window tab has nothing to even. */
  balanceFocusedLayout(): void {
    const focused = this.view.focusedId ? this.view.windows[this.view.focusedId] : undefined;
    if (!focused?.inSplit) return;
    this.balanceLayout({ tabId: focused.tabId });
  }

  protected buildView(): OsDesktopRuntimeStore {
    const { seamPreview, connectionStatus, workArea, routeIntents } =
      windowManagerStore.getSnapshot().context;
    return {
      ...buildOsDesktopRuntimeView({
        snapshot: this.snapshot(),
        globalConfig: this.config(),
        client: this.client,
        workArea: this.workArea(),
        workAreaOrigin: workArea?.origin ?? { x: 0, y: 0 },
        viewportWidth: workArea?.viewportWidth,
        seamPreview,
        routeIntents,
        connectionStatus,
        loadError: this.currentLoadError(),
        wallpaper: this.wallpaper,
        reduceMotion: this.reduceMotion,
      }),
      clientAttachmentToken: this.clientAttachmentToken,
    };
  }

  openOrFocus = (target: OsOpenTarget): WindowManagerOpenOutcome =>
    this.semanticOpens.openOrFocus(target);

  /**
   * Focus-first launch (ADR-010): resolve the live instance semantically by
   * `(app, instanceKey)` — most-recently-focused wins — and only open when
   * none exists. "Open new instance" and "open after this tab" skip the lookup,
   * which is what makes the same gesture explicit instead of ambiguous. A live
   * instance in an inactive tab is focused: focus activates its tab.
   */
  private openOrFocusAttempt(
    target: OsOpenTarget,
    recoverTopologyConflict: boolean
  ): WindowManagerOpenOutcome {
    // Semantic identity and the command revision must come from the same current snapshot.
    const state = this.buildView();
    const existing =
      target.forceNewInstance || target.afterWindowId
        ? null
        : mruWindowInstance(state.windows, state.client?.focusOrder ?? [], {
            app: target.app,
            instanceKey: target.instanceKey ?? null,
          });
    if (existing) {
      const id = existing.id;
      let outcome: WindowManagerCommandOutcome;
      if (target.route && !sameOsWindowRoute(existing.route, target.route)) {
        outcome = this.navigateWindow(id, target.route, target.navigateMode);
      } else {
        outcome = this.focusWindow(id);
      }
      return this.recoverOpenOrFocus(target, id, outcome, recoverTopologyConflict);
    }

    const desktopId =
      target.desktopId ??
      (target.afterWindowId ? state.windows[target.afterWindowId]?.desktopId : undefined) ??
      state.activeDesktopId;
    const id = randomOsWindowId();
    if (desktopId === null || desktopId === undefined) {
      return { windowId: id, accepted: false, completion: Promise.resolve(false) };
    }
    const outcome = this.dispatch({
      ...openWindowCommand(target, id, desktopId),
      // Identity lookups keep their revision fence. Deliberate new windows
      // skip that lookup and bind to the serializer's execution revision.
      ...(target.forceNewInstance || target.afterWindowId
        ? {}
        : { expectedRevision: state.snapshot?.revision }),
    });
    this.publish();
    return this.recoverOpenOrFocus(target, id, outcome, recoverTopologyConflict);
  }

  private recoverOpenOrFocus(
    target: OsOpenTarget,
    id: string,
    outcome: WindowManagerCommandOutcome,
    enabled: boolean
  ): WindowManagerOpenOutcome {
    const startedFromIdle = windowManagerStore.getSnapshot().context.commandState.status === "idle";
    if (!enabled || !outcome.accepted || !startedFromIdle) return { windowId: id, ...outcome };
    return {
      windowId: id,
      accepted: true,
      completion: outcome.completion.then(async applied => {
        if (applied) return true;
        if (windowManagerStore.getSnapshot().context.commandState.status !== "conflict")
          return false;
        if (!(await this.awaitConflictRecovery())) return false;
        const retry = this.openOrFocusAttempt(target, false);
        return retry.accepted && (await retry.completion);
      }),
    };
  }

  closeWindow = (id: string, viewOnly = false): Promise<boolean> =>
    viewOnly ? this.closeWindowViewOnly(id) : this.closeWindowScoped(id, "tab");

  /** Focus also makes the window's tab this client's active tab (daemon client projection). */
  focusWindow = (id: string, options?: FocusWindowOptions): WindowManagerCommandOutcome =>
    this.dispatch(focusWindowCommand(id, options));

  /**
   * Calls `listener` after the runtime adopts an authoritative client view it held behind this
   * client's focuses. Views adopted on arrival (`setClient`) and command answers are not reported
   * here: their callers already act on them.
   */
  onHeldClientAdopted = (listener: () => void): (() => void) =>
    this.focusTurns.onHeldViewAdopted(listener);

  setWallpaper = (wallpaper: OsWallpaper): void => {
    this.wallpaper = wallpaper;
    this.publish();
  };

  setReduceMotion = (on: boolean): void => {
    this.reduceMotion = on;
    this.publish();
  };

  setDesktopBounds = (bounds: OsDesktopBounds): void => {
    if (bounds.width <= 0 || bounds.height <= 0) return;
    windowManagerStore.trigger.workAreaMeasured({
      workArea: {
        viewportWidth: bounds.viewportWidth,
        rect: { x: 0, y: 0, w: bounds.width, h: bounds.height },
        origin: { x: bounds.origin.x, y: bounds.origin.y },
      },
    });
  };
}
