import { useSelector, useStore } from "@xstate/store-react";
import { useEffect, useRef, type KeyboardEvent, type PointerEvent } from "react";

import {
  bindLayoutGestureCancellation,
  type GestureCancelReason,
} from "../lib/layout-gesture-cancellation";
import { seamWeightDelta } from "../lib/seam-preview";
import type { WindowManagerCommandFence, WindowManagerCommandOutcome } from "../lib/os-types";
import { authoritativeSeam, DEFAULT_WINDOW_MANAGER_WORK_AREA } from "../lib/window-manager-view";
import type {
  LayoutNode,
  PixelRect,
  ProjectedSeam,
  WindowManagerSnapshot,
} from "../lib/window-manager-types";
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
 * This client's own resizes and balances on one split: how many are in flight, the newest
 * revision one of them was acknowledged at, and the drag releases waiting for them to settle.
 * The runtime publishes a command's snapshot before its completion settles. Kept per manager (one
 * per client) and split, so a sibling seam's commit counts too.
 */
interface OwnSplitCommands {
  pending: number;
  revision: number;
  settled: Array<() => void>;
}

const ownCommands = new WeakMap<object, Map<string, OwnSplitCommands>>();

function ownSplitCommands(manager: object, splitId: string): OwnSplitCommands {
  let splits = ownCommands.get(manager);
  if (splits === undefined) {
    splits = new Map();
    ownCommands.set(manager, splits);
  }
  let own = splits.get(splitId);
  if (own === undefined) {
    own = { pending: 0, revision: Number.NEGATIVE_INFINITY, settled: [] };
    splits.set(splitId, own);
  }
  return own;
}

/** The split's committed weights in a snapshot, or "" when it has no such split. */
function splitWeightsKey(snapshot: WindowManagerSnapshot | null, splitId: string): string {
  const roots = (snapshot?.desktops ?? []).flatMap(desktop => desktop.tabs.map(tab => tab.root));
  return committedWeights(roots, splitId) ?? "";
}

/** The work area the runtime projects every tab against, preview included. */
function projectedWorkArea(): PixelRect {
  return (
    windowManagerStore.getSnapshot().context.workArea?.rect ?? DEFAULT_WINDOW_MANAGER_WORK_AREA
  );
}

/**
 * What a seam drag is drawn against: the client binding, the shown desktop and tab and the work
 * area (`frame`), and the split's committed weights (`weights`). Hidden tabs stay mounted, so a
 * tab or desktop switch must cancel a drag explicitly.
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
  const weights = useDesktop(state =>
    splitId === undefined ? "" : splitWeightsKey(state.snapshot, splitId)
  );
  return { frame: `${binding}|${shown}|${area}`, weights };
}

export type OsSplitSeamState = "drag" | "limit" | undefined;

/**
 * One seam's gesture. A pointer drag previews the two sides' shares through the shared seam
 * preview, stops at either side's pixel minimum, and commits one `layout.resize` fenced to the
 * revision its seam was laid out at. Escape, a cancelled or lost pointer, a context change
 * (another tab, desktop, binding or work area, the seam gone, or the split's weights changed by
 * someone else while this client has nothing in flight on it) and unmounting cancel it and
 * restore the shares. While this client has a command in flight on the split, a weights change
 * re-bases the drag on the latest layout instead, and a release waits for that command to settle
 * so it is sent against the layout its preview shows. Arrow keys resize in steps and a
 * double-click balances the two sides; every command is fenced to the revision it was authored at.
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
  const rebaseRef = useRef<(() => boolean) | null>(null);

  // A drag outlives neither its seam nor the context it was drawn in. The split's weights may
  // change under it only while this client has a command in flight on that split (or just had
  // one acknowledged); the drag is then re-based on the latest layout.
  const seamId = seam?.id;
  useEffect(() => {
    const drag = store.getSnapshot().context;
    if (drag.phase !== "dragging") return;
    if (drag.seam === null || drag.seam.id !== seamId || drag.contextKey !== contextKey) {
      cancelRef.current?.("context-changed");
      return;
    }
    if (drag.weightsKey === weightsKey) return;
    const revision = manager.getState().snapshot?.revision ?? Number.POSITIVE_INFINITY;
    const own = ownSplitCommands(manager, drag.seam.splitId);
    if (own.pending > 0 || revision <= own.revision) {
      rebaseRef.current?.();
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

  /** Tracks one of this client's commands on a split until it settles. */
  const sent = (splitId: string, outcome: WindowManagerCommandOutcome): Promise<boolean> => {
    if (!outcome.accepted) return outcome.completion;
    const own = ownSplitCommands(manager, splitId);
    own.pending += 1;
    return outcome.completion.finally(() => {
      own.pending -= 1;
      const revision = manager.getState().snapshot?.revision;
      if (revision !== undefined) own.revision = Math.max(own.revision, revision);
      if (own.pending === 0) for (const resume of own.settled.splice(0)) resume();
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
    const own = ownSplitCommands(manager, seam.splitId);
    let frame: number | null = null;
    let live = true;
    let unbindCancellation = () => {};

    /** Lets go of the pointer: its listeners, capture, pending frame and pointer cancellations. */
    const detach = () => {
      if (frame !== null) cancelAnimationFrame(frame);
      frame = null;
      unbindCancellation();
      element.removeEventListener("pointermove", onMove);
      element.removeEventListener("pointerup", onUp);
      if (element.hasPointerCapture(pointerId)) element.releasePointerCapture(pointerId);
    };
    const teardown = () => {
      live = false;
      cancelRef.current = null;
      rebaseRef.current = null;
      detach();
    };
    const cancel = (reason: GestureCancelReason) => {
      teardown();
      store.trigger.dragCancelled({ reason });
      if (generation === previewGeneration) clearPreview();
    };
    const showPreview = () => {
      const drag = store.getSnapshot().context;
      if (drag.phase !== "dragging" || drag.seam === null) return;
      if (generation !== previewGeneration) return;
      windowManagerStore.trigger.seamPreviewSet({
        preview: {
          splitId: drag.seam.splitId,
          boundaryIndex: drag.seam.boundaryIndex,
          deltaPx: drag.deltaPx,
        },
      });
    };
    /**
     * Re-bases the gesture on the split's latest unpreviewed layout — the seam the preview is
     * clamped against — keeping the seam on screen under the pointer. Cancels when it is gone.
     */
    const rebase = (): boolean => {
      const drag = store.getSnapshot().context;
      if (drag.phase !== "dragging" || drag.seam === null) return false;
      const view = manager.getState();
      const latest = authoritativeSeam(view, projectedWorkArea(), drag.seam);
      if (latest === null || view.snapshot === null) {
        cancel("context-changed");
        return false;
      }
      store.trigger.rebased({
        seam: latest,
        revision: view.snapshot.revision,
        weightsKey: splitWeightsKey(view.snapshot, latest.splitId),
      });
      showPreview();
      return true;
    };
    const commit = () => {
      const result = seamDragCommit(store.getSnapshot().context);
      teardown();
      store.trigger.dragEnded({ pointerId });
      if (result === null) {
        if (generation === previewGeneration) clearPreview();
        return;
      }
      // The preview stays until the command settles, so the panes never flash back; a later
      // drag that has taken the preview over keeps it.
      void sent(
        result.splitId,
        manager.resizeLayout(result.splitId, result.boundaryIndex, result.delta, result.fence)
      ).finally(() => {
        if (generation === previewGeneration) clearPreview();
      });
    };
    const onMove = (move: globalThis.PointerEvent) => {
      if (move.pointerId !== pointerId) return;
      store.trigger.pointerMoved({ pointerId, coordinate: coordinate(move) });
      if (frame !== null) return;
      frame = requestAnimationFrame(() => {
        frame = null;
        showPreview();
      });
    };
    const onUp = (up: globalThis.PointerEvent) => {
      if (up.pointerId !== pointerId) return;
      store.trigger.pointerMoved({ pointerId, coordinate: coordinate(up) });
      if (own.pending === 0) {
        commit();
        return;
      }
      // A command of this client on the split is still in flight, and its result moves the
      // layout this release would be sent against: hold the release, previewed, until it settles,
      // then re-base on the latest layout and send.
      detach();
      showPreview();
      own.settled.push(() => {
        if (live && rebase()) commit();
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
    unbindCancellation = bindLayoutGestureCancellation(window, cancel);
    cancelRef.current = cancel;
    rebaseRef.current = rebase;
    element.addEventListener("pointermove", onMove);
    element.addEventListener("pointerup", onUp);
    // The seam on screen may still show an earlier command of this client that has not landed:
    // base the drag on the layout its preview is clamped against, anchored where the seam shows.
    if (own.pending > 0) rebase();
  };

  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (seam === undefined) return;
    const direction = keyDirection(event.key);
    if (direction === null) return;
    event.preventDefault();
    const delta = seamWeightDelta(seam, direction * seamKeyStep(event) * seam.axisSpan);
    if (Math.abs(delta) > SEAM_WEIGHT_EPSILON) {
      void sent(
        seam.splitId,
        manager.resizeLayout(seam.splitId, seam.boundaryIndex, delta, fenceFor(seam))
      );
    }
  };

  /**
   * Balances the two sides this seam separates. With two members that is the whole split, which
   * `layout.balance` evens; in a wider split only this pair is evened, so the others keep theirs.
   */
  const onDoubleClick = () => {
    if (seam === undefined) return;
    if (memberCount <= 2) {
      void sent(seam.splitId, manager.balanceLayout({ splitId: seam.splitId }, fenceFor(seam)));
      return;
    }
    const delta = (seam.trailingWeight - seam.leadingWeight) / 2;
    if (Math.abs(delta) > SEAM_WEIGHT_EPSILON) {
      void sent(
        seam.splitId,
        manager.resizeLayout(seam.splitId, seam.boundaryIndex, delta, fenceFor(seam))
      );
    }
  };

  return { state, onPointerDown, onKeyDown, onDoubleClick };
}
