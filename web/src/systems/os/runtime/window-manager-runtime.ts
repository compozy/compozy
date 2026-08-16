import type { QueryClient } from "@tanstack/react-query";

import { frameForWindow } from "../lib/group-projection";
import { clampFloatingRect } from "../lib/layout-projection";
import {
  createTileSnapTarget,
  type SnapCorner,
  type SnapSide,
  type SnapTarget,
} from "../lib/snap-targets";
import {
  arrangeLayoutCommand,
  moveWindowCommand,
  restoreWindowCommand,
  swapWindowsCommand,
} from "../lib/window-manager-command-builders";
import { mruWindowInstance, randomOsWindowId } from "../lib/window-instance-lookup";
import { arrangePeerWindows, directionalFocusTarget } from "../lib/window-manager-navigation";
import type {
  MoveWindowInput,
  OsArrangePreset,
  OsDesktopBounds,
  OsDesktopRuntime,
  OsDesktopRuntimeStore,
  OsFloatingDrop,
  OsOpenTarget,
  OsRect,
  OsWallpaper,
  WindowManagerCommandOutcome,
  WindowManagerOpenOutcome,
} from "../lib/os-types";
import type { GroupFrameEditInput } from "../lib/frame-seams";
import type { FocusDirection, NormalizedRect } from "../lib/window-manager-types";
import { sameOsWindowRoute } from "../lib/window-manager-route";
import { orderedDesktops } from "../lib/desktop-order";
import { windowManagerLayoutArea } from "../lib/window-manager-layout-area";
import {
  buildOsDesktopRuntimeView,
  normalizedRectToWire,
  pixelRectToNormalized,
} from "../lib/window-manager-view";
import { windowManagerStore } from "../stores/window-manager-store";
import { advanceWindowManagerPlacementCycle } from "../stores/window-manager-store-commands";
import { randomWindowManagerId } from "./window-manager-runtime-core";
import { openWindowCommand, WindowManagerTabRuntime } from "./window-manager-tab-commands";

function rejectedCommandOutcome(): WindowManagerCommandOutcome {
  return { accepted: false, completion: Promise.resolve(false) };
}

export class WindowManagerRuntime extends WindowManagerTabRuntime implements OsDesktopRuntime {
  constructor(queryClient: QueryClient) {
    super(queryClient);
    this.initializeView();
  }

  createDesktop(): void {
    this.dispatch({
      commandId: "desktop.create",
      payload: { desktop_id: "", name: "", purpose: "standard" },
    });
  }

  renameDesktop(desktopId: string, name: string): void {
    this.dispatch({ commandId: "desktop.update", payload: { desktop_id: desktopId, name } });
  }

  reorderDesktop(desktopId: string, order: number): void {
    this.dispatch({ commandId: "desktop.reorder", payload: { desktop_id: desktopId, order } });
  }

  switchDesktop(desktopId: string): void {
    const current = this.view.activeDesktopId;
    const config = this.view.windowManagerConfig;
    const accepted = this.dispatch({
      commandId: "desktop.switch",
      payload: { desktop_id: desktopId },
    });
    if (accepted.accepted && current !== null && current !== desktopId && config !== null) {
      windowManagerStore.trigger.transitionIntentChanged({
        intent: {
          fromDesktopId: current,
          toDesktopId: desktopId,
          direction: this.desktopTransitionDirection(this.view.desktops, current, desktopId),
          mode: this.reduceMotion ? "instant" : config.desktopTransition,
        },
      });
    }
  }

  switchDesktopDirection(direction: "previous" | "next"): void {
    const active = this.view.activeDesktopId;
    const ordered = orderedDesktops(this.view.desktops);
    const index = ordered.findIndex(desktop => desktop.id === active);
    if (index < 0) return;
    const target = ordered[index + (direction === "next" ? 1 : -1)];
    if (target) this.switchDesktop(target.id);
  }

  deleteDesktop(desktopId: string, destinationId: string | null): void {
    this.dispatch({
      commandId: "desktop.delete",
      payload: {
        desktop_id: desktopId,
        ...(destinationId ? { destination_id: destinationId } : {}),
      },
    });
  }

  moveWindowToDesktop(windowId: string, destinationDesktopId: string): void {
    this.dispatch({
      commandId: "window.move",
      payload: {
        window_id: windowId,
        destination_desktop_id: destinationDesktopId,
        placement: "floating",
        move_group: false,
      },
    });
  }

  tileWindow(windowId: string, edge: SnapSide | SnapCorner): void {
    const config = this.view.windowManagerConfig;
    if (!this.view.windows[windowId] || config === null) return;
    const cycleStep = advanceWindowManagerPlacementCycle(windowManagerStore, windowId, edge);
    this.applySnapTarget(
      windowId,
      createTileSnapTarget(
        windowManagerLayoutArea(this.workArea(), config.gaps),
        edge,
        config.snap.repeatRatios,
        cycleStep,
        config.gaps.inner
      )
    );
  }

  applySnapTarget(
    windowId: string,
    target: SnapTarget,
    moveGroup = false
  ): WindowManagerCommandOutcome {
    const window = this.view.windows[windowId];
    if (!window) return rejectedCommandOutcome();
    windowManagerStore.trigger.placementTargetTracked({
      windowId,
      edge: target.kind === "tile" ? target.edge : null,
    });
    // A group move carries every frame member in deck order; `window.move`
    // has no group placement, so tiles arrange the members as one stack.
    const frameMembers = moveGroup
      ? frameForWindow(this.view.frames, windowId)?.members
      : undefined;
    const groupMembers =
      frameMembers !== undefined && frameMembers.length > 1 ? frameMembers : null;
    if (target.kind === "zoom") {
      return this.zoomWindow(windowId);
    }
    if (target.kind === "swap") {
      return this.dispatch(swapWindowsCommand(windowId, target.targetWindowId));
    }
    if (target.kind === "tile") {
      const config = this.view.windowManagerConfig;
      if (config === null) return rejectedCommandOutcome();
      // The frame stores the whole zone: the inner gap is a pixel quantity the
      // projection re-applies, so baking it into a fraction would compound it
      // and drift with every viewport size.
      const frame = pixelRectToNormalized(
        target.zoneRect,
        windowManagerLayoutArea(this.workArea(), config.gaps)
      );
      const outcome = this.dispatch({
        commandId: "layout.arrange",
        payload: {
          desktop_id: window.desktopId,
          window_ids: groupMembers === null ? [windowId] : [...groupMembers],
          arrangement: groupMembers === null ? "horizontal" : "stack",
          frame: normalizedRectToWire(frame),
          group_id: randomWindowManagerId("group"),
        },
      });
      return groupMembers === null
        ? outcome
        : this.restoreStackActive(outcome, groupMembers, windowId);
    }
    const placement = target.kind === "insert" ? target.relation : target.side;
    return this.moveWindow(windowId, {
      destinationDesktopId: window.desktopId,
      targetWindowId: target.targetWindowId,
      placement,
      moveGroup,
    });
  }

  /**
   * `layout.arrange{stack}` activates the first member by contract; when the
   * dragged frame's active tab sat elsewhere in deck order, one follow-up
   * `window.stack.set_active` restores it after the arrange lands.
   */
  private restoreStackActive(
    outcome: WindowManagerCommandOutcome,
    members: readonly string[],
    activeId: string
  ): WindowManagerCommandOutcome {
    if (!outcome.accepted || members[0] === activeId) return outcome;
    return {
      accepted: true,
      completion: outcome.completion.then(async applied => {
        if (!applied) return false;
        const activation = this.activateStackMember(activeId);
        return activation.accepted ? activation.completion : false;
      }),
    };
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
    return buildOsDesktopRuntimeView({
      snapshot: this.snapshot(),
      globalConfig: this.config(),
      client: this.client,
      workArea: this.workArea(),
      workAreaOrigin: workArea?.origin ?? { x: 0, y: 0 },
      seamPreview,
      routeIntents,
      connectionStatus,
      loadError: this.currentLoadError(),
      railCollapsedAgentIds: this.railCollapsedAgentIds,
      wallpaper: this.wallpaper,
      reduceMotion: this.reduceMotion,
      dockMagnify: this.dockMagnify,
    });
  }

  openOrFocus = (target: OsOpenTarget): WindowManagerOpenOutcome =>
    this.openOrFocusAttempt(target, true);

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
        if (!(await this.refreshSnapshot())) return false;
        this.clearConflict();
        const retry = this.openOrFocusAttempt(target, false);
        return retry.accepted && (await retry.completion);
      }),
    };
  }

  closeWindow = (id: string): Promise<boolean> => this.closeWindowScoped(id, "tab");

  focusWindow = (id: string): WindowManagerCommandOutcome => {
    return this.dispatch({
      commandId: "window.focus",
      payload: { window_id: id, direction: "" },
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

  zoomWindow = (id: string): WindowManagerCommandOutcome => {
    return this.dispatch({ commandId: "window.zoom", payload: { window_id: id } });
  };

  toggleFloating = (id: string, floatingRect?: OsRect): WindowManagerCommandOutcome => {
    const normalized = floatingRect
      ? pixelRectToNormalized(
          clampFloatingRect({ proposedRect: floatingRect, workArea: this.workArea() }),
          this.workArea()
        )
      : undefined;
    return this.dispatch({
      commandId: "window.toggle_floating",
      payload: {
        window_id: id,
        ...(normalized ? { floating_rect: normalizedRectToWire(normalized) } : {}),
      },
    });
  };

  moveWindow = (id: string, input: MoveWindowInput): WindowManagerCommandOutcome => {
    return this.dispatch(moveWindowCommand(id, input, this.workArea()));
  };

  arrangeLayout = (anchorId: string, preset: OsArrangePreset): void => {
    const anchor = this.view.windows[anchorId];
    if (!anchor) return;
    const peers = arrangePeerWindows(this.view.windows, anchorId);
    const command = arrangeLayoutCommand(anchor, peers, preset, randomWindowManagerId("group"));
    if (command !== null) this.dispatch(command);
  };

  commitFloatingRect = (
    id: string,
    rect: OsRect,
    drop?: OsFloatingDrop,
    moveGroup = false
  ): WindowManagerCommandOutcome => {
    const window = this.view.windows[id];
    if (!window) return rejectedCommandOutcome();
    const area = this.workArea();
    const clamped = clampFloatingRect({
      proposedRect: rect,
      workArea: area,
      pointer: drop?.pointer,
      grabOffset: drop?.grabOffset,
    });
    return this.moveWindow(id, {
      destinationDesktopId: window.desktopId,
      placement: "floating",
      floatingRect: clamped,
      moveGroup,
    });
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

  toggleRailGroup = (agentId: string): void => {
    const normalized = agentId.trim();
    if (!normalized) return;
    this.railCollapsedAgentIds = this.railCollapsedAgentIds.includes(normalized)
      ? this.railCollapsedAgentIds.filter(id => id !== normalized)
      : [...this.railCollapsedAgentIds, normalized];
    this.publish();
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
