import type { QueryClient } from "@tanstack/react-query";

import { frameForWindow } from "../lib/group-projection";
import { clampFloatingRect } from "../lib/layout-projection";
import type { GestureRebase, MoveWindowInput, OsFloatingDrop, OsRect } from "../lib/os-types";
import type { WindowManagerCommandOutcome } from "../lib/os-types";
import {
  createTileSnapTarget,
  type SnapCorner,
  type SnapSide,
  type SnapTarget,
} from "../lib/snap-targets";
import { moveWindowCommand, swapWindowsCommand } from "../lib/window-manager-command-builders";
import { windowManagerLayoutArea } from "../lib/window-manager-layout-area";
import type { WindowManagerCommandInput } from "../lib/window-manager-types";
import { normalizedRectToWire, pixelRectToNormalized } from "../lib/window-manager-view";
import { windowManagerStore } from "../stores/window-manager-store";
import { advanceWindowManagerPlacementCycle } from "../stores/window-manager-store-commands";
import { randomWindowManagerId } from "./window-manager-runtime-helpers";
import { WindowManagerTabRuntime } from "./window-manager-tab-commands";

function rejectedCommandOutcome(): WindowManagerCommandOutcome {
  return { accepted: false, completion: Promise.resolve(false) };
}

/**
 * Geometry half of the command surface: zoom, float, move, snap, and tile.
 * Every method dispatches one semantic daemon command; pixels never persist here.
 */
export abstract class WindowManagerSnapRuntime extends WindowManagerTabRuntime {
  constructor(queryClient: QueryClient) {
    super(queryClient);
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
    moveGroup = false,
    rebase?: GestureRebase
  ): WindowManagerCommandOutcome {
    const window = this.view.windows[windowId];
    if (!window) return rejectedCommandOutcome();
    const guard = (
      targetNodeId?: string
    ): Pick<WindowManagerCommandInput, "rebase" | "expectedRevision"> =>
      rebase === undefined
        ? {}
        : {
            expectedRevision: rebase.expectedRevision,
            rebase: {
              windowId,
              ...(rebase.sourceNodeId ? { sourceNodeId: rebase.sourceNodeId } : {}),
              ...(targetNodeId ? { targetNodeId } : {}),
            },
          };
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
      return this.dispatch({
        ...swapWindowsCommand(windowId, target.targetWindowId),
        ...guard(target.targetNodeId),
      });
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
        ...guard(),
      });
      return groupMembers === null
        ? outcome
        : this.restoreStackActive(outcome, groupMembers, windowId);
    }
    const placement = target.kind === "insert" ? target.relation : target.side;
    return this.dispatch({
      ...moveWindowCommand(
        windowId,
        {
          destinationDesktopId: window.desktopId,
          targetWindowId: target.targetWindowId,
          placement,
          moveGroup,
        },
        this.workArea()
      ),
      ...guard(target.targetNodeId),
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

  commitFloatingRect = (
    id: string,
    rect: OsRect,
    drop?: OsFloatingDrop,
    moveGroup = false,
    rebase?: GestureRebase
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
    const command = moveWindowCommand(
      id,
      {
        destinationDesktopId: window.desktopId,
        placement: "floating",
        floatingRect: clamped,
        moveGroup,
      },
      area
    );
    if (rebase === undefined) return this.dispatch(command);
    return this.dispatch({
      ...command,
      expectedRevision: rebase.expectedRevision,
      rebase: {
        windowId: id,
        ...(rebase.sourceNodeId ? { sourceNodeId: rebase.sourceNodeId } : {}),
      },
    });
  };
}
