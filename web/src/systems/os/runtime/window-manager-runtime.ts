import type { QueryClient } from "@tanstack/react-query";

import { frameForWindow, zoomedFrame } from "../lib/group-projection";
import { arrangeLayoutCommand, restoreWindowCommand } from "../lib/window-manager-command-builders";
import { mruWindowInstance, randomOsWindowId } from "../lib/window-instance-lookup";
import { arrangePeerWindows, directionalFocusTarget } from "../lib/window-manager-navigation";
import type {
  OsArrangePreset,
  OsDesktopBounds,
  OsDesktopRuntime,
  OsDesktopRuntimeStore,
  OsOpenTarget,
  OsWallpaper,
  WindowManagerCommandOutcome,
  WindowManagerOpenOutcome,
} from "../lib/os-types";
import type { GroupFrameEditInput } from "../lib/frame-seams";
import type { FocusDirection, NormalizedRect } from "../lib/window-manager-types";
import { sameOsWindowRoute } from "../lib/window-manager-route";
import { buildOsDesktopRuntimeView, normalizedRectToWire } from "../lib/window-manager-view";
import { windowManagerStore } from "../stores/window-manager-store";
import type { WindowManagerRuntimeBinding } from "./window-manager-runtime-core";
import { randomWindowManagerId } from "./window-manager-runtime-helpers";
import { WindowManagerDesktopRuntime } from "./window-manager-desktop-commands";
import { WindowManagerSemanticOpenCoordinator } from "./window-manager-semantic-open";
import { openWindowCommand } from "./window-manager-tab-commands";

function rejectedCommandOutcome(): WindowManagerCommandOutcome {
  return { accepted: false, completion: Promise.resolve(false) };
}

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

  focusDirection(direction: FocusDirection): void {
    const config = this.view.windowManagerConfig;
    const activeDesktopId = this.view.activeDesktopId;
    if (config === null || activeDesktopId === null) return;
    const target = directionalFocusTarget({
      windows: Object.values(this.view.windows).filter(
        window => window.desktopId === activeDesktopId
      ),
      focusedId: this.view.focusedId,
      direction,
      wrap: config.focusWrap,
    });
    if (target === null) return;
    this.dispatch({
      commandId: "window.focus",
      payload: { window_id: target, direction: "" },
      rebase: { windowId: target },
    });
  }

  undoLayout(): void {
    this.dispatch({ commandId: "layout.undo", payload: {} });
  }

  redoLayout(): void {
    this.dispatch({ commandId: "layout.redo", payload: {} });
  }

  balanceFocusedLayout(): void {
    const focused = this.view.focusedId ? this.view.windows[this.view.focusedId] : undefined;
    if (!focused) return;
    this.balanceLayout(focused.groupId ?? undefined, focused.nodeId ?? undefined);
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
        seamPreview,
        routeIntents,
        connectionStatus,
        loadError: this.currentLoadError(),
        wallpaper: this.wallpaper,
        reduceMotion: this.reduceMotion,
        dockMagnify: this.dockMagnify,
      }),
      clientAttachmentToken: this.clientAttachmentToken,
    };
  }

  openOrFocus = (target: OsOpenTarget): WindowManagerOpenOutcome =>
    this.semanticOpens.openOrFocus(target);

  /**
   * Focus-first launch (ADR-010): resolve the live instance semantically by
   * `(app, instanceKey)` — most-recently-focused wins — and only open when
   * none exists. "Open new instance" and "open as tab" skip the lookup, which
   * is what makes the same gesture explicit instead of ambiguous.
   */
  private openOrFocusAttempt(
    target: OsOpenTarget,
    recoverTopologyConflict: boolean
  ): WindowManagerOpenOutcome {
    const existing =
      target.forceNewInstance || target.stackTargetWindowId
        ? null
        : mruWindowInstance(this.view.windows, this.view.client?.focusOrder ?? [], {
            app: target.app,
            instanceKey: target.instanceKey ?? null,
          });
    if (existing) {
      const id = existing.id;
      let outcome: WindowManagerCommandOutcome;
      if (existing.minimized) {
        const authoritative = this.view.snapshot?.windows[id];
        outcome = authoritative
          ? this.dispatch(restoreWindowCommand(authoritative, target.route))
          : rejectedCommandOutcome();
      } else if (target.route && !sameOsWindowRoute(existing.route, target.route)) {
        outcome = this.navigateWindow(id, target.route, target.navigateMode);
      } else {
        outcome = this.focusWindow(id);
      }
      return this.recoverOpenOrFocus(target, id, outcome, recoverTopologyConflict);
    }

    const desktopId =
      (target.stackTargetWindowId
        ? this.view.windows[target.stackTargetWindowId]?.desktopId
        : undefined) ?? this.view.activeDesktopId;
    const id = randomOsWindowId();
    if (desktopId === null || desktopId === undefined) {
      return { windowId: id, accepted: false, completion: Promise.resolve(false) };
    }
    const outcome = this.dispatch(openWindowCommand(target, id, desktopId));
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

  closeWindow = (id: string): Promise<boolean> => this.closeWindowScoped(id, "tab");

  /**
   * Focus reveals: a tiled window hidden under a zoomed unit on its desktop is
   * unzoomed into view first, so activating it from the dock or palette never
   * lands focus on something the user cannot see.
   */
  focusWindow = (id: string): WindowManagerCommandOutcome => {
    const window = this.view.windows[id];
    if (window && !window.minimized && window.placement !== "floating") {
      const zoomed = frameForWindow(this.view.frames, id);
      const covering = zoomedFrame(this.view.frames[window.desktopId]);
      if (covering !== null && (zoomed === null || zoomed.id !== covering.id)) {
        this.zoomWindow(covering.activeWindowId);
      }
    }
    return this.dispatch({
      commandId: "window.focus",
      payload: { window_id: id, direction: "" },
      rebase: { windowId: id },
    });
  };

  minimizeWindow = (id: string): Promise<boolean> => {
    return this.dispatch({
      commandId: "window.close",
      payload: { window_id: id, minimize: true },
    }).completion;
  };

  restoreWindow = (id: string): WindowManagerCommandOutcome => {
    const window = this.view.snapshot?.windows[id];
    if (!window) return rejectedCommandOutcome();
    return this.dispatch(restoreWindowCommand(window));
  };

  arrangeLayout = (anchorId: string, preset: OsArrangePreset): void => {
    const anchor = this.view.windows[anchorId];
    if (!anchor) return;
    const peers = arrangePeerWindows(this.view.windows, anchorId);
    const command = arrangeLayoutCommand(anchor, peers, preset, randomWindowManagerId("group"));
    if (command !== null) this.dispatch(command);
  };

  resizeLayout = (
    splitId: string,
    boundaryIndex: number,
    delta: number
  ): WindowManagerCommandOutcome => {
    return this.dispatch({
      commandId: "layout.resize",
      payload: { split_id: splitId, boundary_index: boundaryIndex, delta },
      rebase: { splitId, boundaryIndex },
    });
  };

  /** Free-edge resize of one tiled unit; the daemon detaches split members. */
  resizeWindowFrame = (windowId: string, frame: NormalizedRect): WindowManagerCommandOutcome => {
    return this.dispatch({
      commandId: "window.resize",
      payload: { window_id: windowId, frame: normalizedRectToWire(frame) },
    });
  };

  /** One atomic multi-island frame rewrite for a shared-boundary drag. */
  resizeGroupFrames = (
    desktopId: string,
    edits: readonly GroupFrameEditInput[]
  ): WindowManagerCommandOutcome => {
    if (edits.length === 0) return rejectedCommandOutcome();
    return this.dispatch({
      commandId: "layout.frame_resize",
      payload: {
        desktop_id: desktopId,
        edits: edits.map(edit => ({
          group_id: edit.groupId,
          frame: normalizedRectToWire(edit.frame),
        })),
      },
    });
  };

  balanceLayout = (groupId?: string, splitId?: string): void => {
    this.dispatch({
      commandId: "layout.balance",
      payload: {
        ...(groupId ? { group_id: groupId } : {}),
        ...(splitId ? { split_id: splitId } : {}),
      },
    });
  };

  setWallpaper = (wallpaper: OsWallpaper): void => {
    this.wallpaper = wallpaper;
    this.publish();
  };

  setDockMagnify = (on: boolean): void => {
    this.dockMagnify = on;
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
        rect: { x: 0, y: 0, w: bounds.width, h: bounds.height },
        origin: { x: bounds.origin.x, y: bounds.origin.y },
      },
    });
  };
}
