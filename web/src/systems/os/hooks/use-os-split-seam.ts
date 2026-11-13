import { useSelector, useStore } from "@xstate/store-react";
import { useEffect, useRef, type KeyboardEvent, type PointerEvent } from "react";

import {
  bindLayoutGestureCancellation,
  type GestureCancelReason,
} from "../lib/layout-gesture-cancellation";
import { seamWeightDelta } from "../lib/seam-preview";
import type { WindowManagerCommandFence, WindowManagerCommandOutcome } from "../lib/os-types";
import type { LayoutNode, ProjectedSeam } from "../lib/window-manager-types";
import { windowManagerStore } from "../stores/window-manager-store";
import {
  SEAM_WEIGHT_EPSILON,
  seamDragAtLimit,
  seamDragCommit,
  seamDragLogic,
  seamKeyStep,
} from "./seam-drag-store";
import { useDesktop } from "./use-desktop";
import { useOsShell } from "./use-os-shell";

function keyDirection(key: string): -1 | 1 | null {
  if (key === "ArrowLeft" || key === "ArrowUp") return -1;
  if (key === "ArrowRight" || key === "ArrowDown") return 1;
  return null;
}

function clearPreview(): void {
  windowManagerStore.trigger.seamPreviewCleared();
}

/** The committed weights of one split in the authoritative snapshot (never the live preview). */
function committedWeights(nodes: readonly LayoutNode[], splitId: string): string | null {
  for (const node of nodes) {
    if (node.kind !== "split") continue;
    if (node.id === splitId) return node.weights.join(",");
    const nested = committedWeights(node.children, splitId);
    if (nested !== null) return nested;
  }
  return null;
}

/**
 * The shared seam preview belongs to the newest seam drag: a commit settling late clears it only
 * when no later drag has taken it over since.
 */
let previewGeneration = 0;

/**
 * What a seam drag is drawn against: the client binding, the shown desktop and tab and the work
 * area (`frame`), and the split's committed weights (`weights`). Hidden tabs stay mounted, so a
 * tab or desktop switch must cancel a drag explicitly; weights changed by someone else would
 * rebase the drag's delta onto shares its preview never showed.
 */
function useSeamGestureContext(splitId: string | undefined): { frame: string; weights: string } {
  const binding = useSelector(windowManagerStore, snapshot => {
    const value = snapshot.context.binding;
    return value === null ? "" : `${value.workspaceId}/${value.profileId}/${value.clientId}`;
  });
  const area = useSelector(windowManagerStore, snapshot => {
    const rect = snapshot.context.workArea?.rect;
    return rect === undefined ? "" : `${rect.x},${rect.y},${rect.w},${rect.h}`;
  });
  const shown = useDesktop(state => {
    const desktopId = state.activeDesktopId ?? "";
    return `${desktopId}/${state.activeTabIds[desktopId] ?? ""}`;
  });
  const weights = useDesktop(state => {
    if (splitId === undefined) return "";
    const roots = (state.snapshot?.desktops ?? []).flatMap(desktop =>
      desktop.tabs.map(tab => tab.root)
    );
    return committedWeights(roots, splitId) ?? "";
  });
  return { frame: `${binding}|${shown}|${area}`, weights };
}

export type OsSplitSeamState = "drag" | "limit" | undefined;

/**
 * One seam's gesture. A pointer drag previews the two sides' shares through the shared seam
 * preview, stops at either side's pixel minimum, and commits one `layout.resize` fenced to the
 * revision it started from. Escape, a cancelled or lost pointer, a context change (another tab,
 * desktop, binding or work area, or the split's weights changed by someone else) and unmounting
 * cancel it and restore the shares. Arrow keys resize in steps and a double-click balances the
 * two sides; every command is fenced to the revision it was authored at.
 */
export function useOsSplitSeam(seam: ProjectedSeam | undefined, memberCount: number) {
  const { manager } = useOsShell();
  const store = useStore(seamDragLogic);
  const state = useSelector(store, (snapshot): OsSplitSeamState => {
    if (snapshot.context.phase !== "dragging") return undefined;
    return seamDragAtLimit(snapshot.context) ? "limit" : "drag";
  });
  const { frame: contextKey, weights: weightsKey } = useSeamGestureContext(seam?.splitId);
  const cancelRef = useRef<((reason: GestureCancelReason) => void) | null>(null);
  /**
   * This seam's own resizes and balances: how many are in flight, and the newest revision one of
   * them was acknowledged at. The runtime publishes a command's snapshot before its completion
   * settles, so a weight change at or below that revision is this client's own echo.
   */
  const own = useRef({ pending: 0, revision: Number.NEGATIVE_INFINITY });

  // A drag outlives neither its seam nor the context it was drawn in; the split's weights may
  // change under it only through this seam's own earlier commands.
  const seamId = seam?.id;
  useEffect(() => {
    const drag = store.getSnapshot().context;
    if (drag.phase !== "dragging") return;
    if (drag.seam?.id !== seamId || drag.contextKey !== contextKey) {
      cancelRef.current?.("context-changed");
      return;
    }
    if (drag.weightsKey === weightsKey) return;
    const revision = manager.getState().snapshot?.revision ?? Number.POSITIVE_INFINITY;
    if (own.current.pending > 0 || revision <= own.current.revision) {
      store.trigger.weightsAcknowledged({ weightsKey });
      return;
    }
    cancelRef.current?.("context-changed");
  }, [store, manager, seamId, contextKey, weightsKey]);
  useEffect(() => () => cancelRef.current?.("disposed"), []);

  /** Keyboard and balance commands are fenced like a drag: to this revision and this seam. */
  const fenceFor = (target: ProjectedSeam): WindowManagerCommandFence | undefined => {
    const revision = manager.getState().snapshot?.revision;
    if (revision === undefined) return undefined;
    const { splitId, boundaryIndex } = target;
    return { expectedRevision: revision, rebase: { splitId, boundaryIndex } };
  };

  /** Tracks one of this seam's commands until it settles, so its echo never cancels a drag. */
  const sent = (outcome: WindowManagerCommandOutcome): Promise<boolean> => {
    if (!outcome.accepted) return outcome.completion;
    own.current.pending += 1;
    return outcome.completion.finally(() => {
      own.current.pending -= 1;
      const revision = manager.getState().snapshot?.revision;
      if (revision !== undefined) own.current.revision = Math.max(own.current.revision, revision);
    });
  };

  const onPointerDown = (event: PointerEvent<HTMLElement>) => {
    const revision = manager.getState().snapshot?.revision;
    if (seam === undefined || revision === undefined || event.button !== 0) return;
    if (store.getSnapshot().context.phase !== "idle") return;
    // Suppresses the browser's text selection under the moving pointer.
    event.preventDefault();
    const element = event.currentTarget;
    const pointerId = event.pointerId;
    const vertical = seam.orientation === "vertical";
    const coordinate = (pointer: { clientX: number; clientY: number }) =>
      vertical ? pointer.clientX : pointer.clientY;
    let frame: number | null = null;

    const teardown = () => {
      cancelRef.current = null;
      if (frame !== null) cancelAnimationFrame(frame);
      frame = null;
      unbindCancellation();
      element.removeEventListener("pointermove", onMove);
      element.removeEventListener("pointerup", onUp);
      if (element.hasPointerCapture(pointerId)) element.releasePointerCapture(pointerId);
    };
    const cancel = (reason: GestureCancelReason) => {
      teardown();
      store.trigger.dragCancelled({ reason });
      if (generation === previewGeneration) clearPreview();
    };
    const onMove = (move: globalThis.PointerEvent) => {
      if (move.pointerId !== pointerId) return;
      store.trigger.pointerMoved({ pointerId, coordinate: coordinate(move) });
      if (frame !== null) return;
      frame = requestAnimationFrame(() => {
        frame = null;
        const drag = store.getSnapshot().context;
        if (drag.phase !== "dragging" || drag.seam === null) return;
        windowManagerStore.trigger.seamPreviewSet({
          preview: {
            splitId: drag.seam.splitId,
            boundaryIndex: drag.seam.boundaryIndex,
            deltaPx: drag.deltaPx,
          },
        });
      });
    };
    const onUp = (up: globalThis.PointerEvent) => {
      if (up.pointerId !== pointerId) return;
      store.trigger.pointerMoved({ pointerId, coordinate: coordinate(up) });
      const commit = seamDragCommit(store.getSnapshot().context);
      teardown();
      store.trigger.dragEnded({ pointerId });
      if (commit === null) {
        if (generation === previewGeneration) clearPreview();
        return;
      }
      // The preview stays until the command settles, so the panes never flash back; a later
      // drag that has taken the preview over keeps it.
      void sent(
        manager.resizeLayout(commit.splitId, commit.boundaryIndex, commit.delta, commit.fence)
      ).finally(() => {
        if (generation === previewGeneration) clearPreview();
      });
    };

    element.setPointerCapture(pointerId);
    previewGeneration += 1;
    const generation = previewGeneration;
    store.trigger.dragStarted({
      pointerId,
      coordinate: coordinate(event),
      seam,
      revision,
      contextKey,
      weightsKey,
    });
    const unbindCancellation = bindLayoutGestureCancellation(window, cancel);
    cancelRef.current = cancel;
    element.addEventListener("pointermove", onMove);
    element.addEventListener("pointerup", onUp);
  };

  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (seam === undefined) return;
    const direction = keyDirection(event.key);
    if (direction === null) return;
    event.preventDefault();
    const delta = seamWeightDelta(seam, direction * seamKeyStep(event) * seam.axisSpan);
    if (Math.abs(delta) > SEAM_WEIGHT_EPSILON) {
      void sent(manager.resizeLayout(seam.splitId, seam.boundaryIndex, delta, fenceFor(seam)));
    }
  };

  /**
   * Balances the two sides this seam separates. With two members that is the whole split, which
   * `layout.balance` evens; in a wider split only this pair is evened, so the others keep theirs.
   */
  const onDoubleClick = () => {
    if (seam === undefined) return;
    if (memberCount <= 2) {
      void sent(manager.balanceLayout({ splitId: seam.splitId }, fenceFor(seam)));
      return;
    }
    const delta = (seam.trailingWeight - seam.leadingWeight) / 2;
    if (Math.abs(delta) > SEAM_WEIGHT_EPSILON) {
      void sent(manager.resizeLayout(seam.splitId, seam.boundaryIndex, delta, fenceFor(seam)));
    }
  };

  return { state, onPointerDown, onKeyDown, onDoubleClick };
}
