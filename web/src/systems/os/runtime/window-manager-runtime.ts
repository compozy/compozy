import type { QueryClient } from "@tanstack/react-query";

import { focusWindowCommand, openWindowCommand } from "../lib/window-manager-command-builders";
import { mruWindowInstance, randomOsWindowId } from "../lib/window-instance-lookup";
import {
  activeTabCandidates,
  directionalFocusTarget,
  directionalSwapTarget,
  type ShownWindow,
} from "../lib/window-manager-navigation";
import type {
  OsDesktopBounds,
  OsDesktopRuntime,
  OsDesktopRuntimeStore,
  OsOpenTarget,
  OsWallpaper,
  WindowManagerCommandOutcome,
  WindowManagerOpenOutcome,
} from "../lib/os-types";
import type {
  FocusDirection,
  FocusWindowOptions,
  WindowManagerCommandId,
  WindowManagerCommandInput,
} from "../lib/window-manager-types";
import { sameOsWindowRoute } from "../lib/window-manager-route";
import { buildOsDesktopRuntimeView } from "../lib/window-manager-view";
import { windowManagerStore } from "../stores/window-manager-store";
import type { WindowManagerRuntimeBinding } from "./window-manager-runtime-core";
import { WindowManagerDesktopRuntime } from "./window-manager-desktop-commands";
import { PendingSwaps } from "./window-manager-pending-swaps";
import { WindowManagerSemanticOpenCoordinator } from "./window-manager-semantic-open";

/** Commands that never move a window within the shown tab: the pending-swap overlay survives. */
const LAYOUT_PRESERVING_COMMANDS: ReadonlySet<WindowManagerCommandId> = new Set([
  "window.swap",
  "window.focus",
  "window.navigate",
  "window.pin",
  "window.tab.reorder",
  "desktop.create",
  "desktop.update",
  "desktop.reorder",
]);

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

  /**
   * Moves focus between the windows on screen in the active tab, by projected geometry with this
   * client's unanswered swaps applied, from the newest focus this client asked for.
   */
  focusDirection(direction: FocusDirection): void {
    const config = this.view.windowManagerConfig;
    if (config === null) return;
    const candidates = this.directionalCandidates();
    if (candidates.length === 0) return;
    const from = this.directionalFocusId();
    const target = directionalFocusTarget({
      candidates,
      focusedId: from,
      direction,
      wrap: config.focusWrap,
    });
    if (target === null || target === from) return;
    this.dispatch(focusWindowCommand(target, { replaceable: true }));
  }

  /**
   * Trades the focused window's place with its on-screen neighbour on that side of the active tab,
   * from the newest focus this client asked for. Never wraps; nothing on that side is a no-op. The
   * moved window keeps focus: `window.swap` exchanges places and leaves focus alone. The swap is
   * sent at once, in order with every other command; the neighbour is read from the layout with
   * this client's unanswered swaps applied, so two quick presses move the window two places.
   */
  swapDirection(direction: FocusDirection): void {
    const from = this.directionalFocusId();
    const target = directionalSwapTarget(this.directionalCandidates(), from, direction);
    if (from === null || target === null) return;
    this.swapWindows(from, target);
  }

  private readonly pendingSwaps = new PendingSwaps();

  /**
   * Every accepted swap, a directional one or a Shift-drop, joins the pending-swap overlay until it
   * settles, and the view republishes so menus re-read their sides. A command that can change the
   * shown layout (an undo, a move, a tile, an arrange, a close, a tab or desktop change) runs after
   * the unanswered swaps in ways this client cannot predict, so it drops the overlay and
   * directional input reads the shown layout again. Commands that leave window places alone keep
   * it.
   */
  protected override dispatch(command: WindowManagerCommandInput): WindowManagerCommandOutcome {
    if (command.commandId !== "window.swap") {
      if (!LAYOUT_PRESERVING_COMMANDS.has(command.commandId)) this.pendingSwaps.clear();
      return super.dispatch(command);
    }
    const shown = this.directionalCandidates();
    const outcome = super.dispatch(command);
    const { first_window_id: first, second_window_id: second } = command.payload;
    if (!outcome.accepted || typeof first !== "string" || typeof second !== "string") {
      return outcome;
    }
    const release = this.pendingSwaps.record(first, second, shown);
    if (release === null) return outcome;
    this.publish();
    // A refused swap publishes before it settles; republish so menus drop the predicted places.
    const settle = () => {
      release();
      this.publish();
    };
    void outcome.completion.then(settle, settle);
    return outcome;
  }

  /**
   * The active tab's on-screen windows with this client's unanswered swaps applied, so directional
   * focus, swaps and the Window menu read where the windows are going.
   */
  directionalCandidates(): ShownWindow[] {
    return this.pendingSwaps.apply(activeTabCandidates(this.view));
  }

  /**
   * The window directional focus and swaps start from. A focus still waiting for its answer is
   * where the operator already is: the view names the focus before it until that answer shows,
   * and a press meanwhile must not start from there.
   */
  directionalFocusId(): string | null {
    const pending = this.focusTurns.pendingFocus();
    if (pending === null) return this.view.focusedId;
    const onScreen = activeTabCandidates(this.view).some(candidate => candidate.id === pending);
    return onScreen ? pending : this.view.focusedId;
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
    this.balanceTab(focused.tabId);
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
   * Calls `listener` after the runtime releases an authoritative client view it held behind this
   * client's focuses, adopted or ignored as older. Views adopted on arrival (`setClient`) are not
   * reported here: their caller already acts on them. A focus answer is not reported either; the
   * stream view its daemon published first was held, so its release reports for both.
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
