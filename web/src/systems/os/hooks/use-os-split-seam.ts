import { useSelector, useStore } from "@xstate/store-react";
import { useEffect, useRef, type KeyboardEvent, type PointerEvent } from "react";

import {
  bindLayoutGestureCancellation,
  type GestureCancelReason,
} from "../lib/layout-gesture-cancellation";
import { seamWeightDelta } from "../lib/seam-preview";
import type {
  OsDesktopRuntimeStore,
  WindowManagerCommandFence,
  WindowManagerCommandOutcome,
} from "../lib/os-types";
import { authoritativeSeam, DEFAULT_WINDOW_MANAGER_WORK_AREA } from "../lib/window-manager-view";
import type { PixelRect, ProjectedSeam } from "../lib/window-manager-types";
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

/**
 * The shared seam preview belongs to the newest seam drag: a commit settling late clears it only
 * when no later drag has taken it over since.
 */
let previewGeneration = 0;

/**
 * This client's own resizes and balances in flight on one split, and the drag releases waiting
 * for them to settle. Kept per manager (one per client) and split, so a sibling seam's command
 * counts too, and only while something is in flight: the map never outgrows the splits this
 * client is resizing right now. The runtime publishes a command's snapshot before its completion
 * settles, so its result always lands while it is still counted here.
 */
interface OwnSplitCommands {
  pending: number;
  settled: Array<() => void>;
}

const ownCommands = new WeakMap<object, Map<string, OwnSplitCommands>>();

function ownPending(manager: object, splitId: string): number {
  return ownCommands.get(manager)?.get(splitId)?.pending ?? 0;
}

/** Counts one of this client's commands on a split until it settles. */
function trackOwnCommand(
  manager: object,
  splitId: string,
  completion: Promise<boolean>
): Promise<boolean> {
  let splits = ownCommands.get(manager);
  if (splits === undefined) {
    splits = new Map();
    ownCommands.set(manager, splits);
  }
  let own = splits.get(splitId);
  if (own === undefined) {
    own = { pending: 0, settled: [] };
    splits.set(splitId, own);
  }
  const tracked = own;
  const owner = splits;
  tracked.pending += 1;
  return completion.finally(() => {
    tracked.pending -= 1;
    if (tracked.pending > 0) return;
    owner.delete(splitId);
    for (const resume of tracked.settled.splice(0)) resume();
  });
}

/** Runs `resume` once nothing of this client is in flight on the split. */
function afterOwnCommands(manager: object, splitId: string, resume: () => void): void {
  const own = ownCommands.get(manager)?.get(splitId);
  if (own === undefined || own.pending === 0) resume();
  else own.settled.push(resume);
}

/** The work area the runtime projects every tab against, preview included. */
function projectedWorkArea(): PixelRect {
  return (
    windowManagerStore.getSnapshot().context.workArea?.rect ?? DEFAULT_WINDOW_MANAGER_WORK_AREA
  );
}

const GEOMETRY_EPSILON = 0.000001;

/**
 * Whether two layouts of a seam drive a drag the same way: everything the preview, the limit
 * state and the release read (orientation, position, bounds, pixel span and the two weights).
 * The hit rect and window ids are not part of it, so a re-projection that lands the seam in the
 * same place never disturbs a drag.
 */
function sameSeamGeometry(left: ProjectedSeam, right: ProjectedSeam): boolean {
  const close = (a: number, b: number) => Math.abs(a - b) <= GEOMETRY_EPSILON;
  return (
    left.orientation === right.orientation &&
    close(left.value, right.value) &&
    close(left.minValue, right.minValue) &&
    close(left.maxValue, right.maxValue) &&
    close(left.axisSpan, right.axisSpan) &&
    close(left.leadingWeight, right.leadingWeight) &&
    close(left.trailingWeight, right.trailingWeight)
  );
}

/**
 * What a seam drag is drawn against: the client binding, the shown desktop and tab and the work
 * area. Hidden tabs stay mounted, so a tab or desktop switch must cancel a drag explicitly.
 */
function useSeamGestureFrame(): string {
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
  return `${binding}|${shown}|${area}`;
}

export type OsSplitSeamState = "drag" | "limit" | undefined;

/**
 * One seam's gesture. A pointer drag previews the two sides' shares through the shared seam
 * preview, stops at either side's pixel minimum, and commits one `layout.resize` fenced to the
 * revision its seam was laid out at. Escape, a cancelled or lost pointer, a context change
 * (another tab, desktop, binding or work area, the seam gone, or the seam's layout changed by
 * someone else while this client has nothing in flight on its split) and unmounting cancel it and
 * restore the shares. While this client has a command in flight on the split, a layout change
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
  const contextKey = useSeamGestureFrame();
  const cancelRef = useRef<((reason: GestureCancelReason) => void) | null>(null);

  // A drag outlives neither its seam nor the frame it was drawn in; the seam's own layout is
  // watched by the drag itself, on every snapshot.
  const seamId = seam?.id;
  useEffect(() => {
    const drag = store.getSnapshot().context;
    if (drag.phase !== "dragging") return;
    if (drag.seam === null || drag.seam.id !== seamId || drag.contextKey !== contextKey) {
      cancelRef.current?.("context-changed");
    }
  }, [store, seamId, contextKey]);
  useEffect(() => () => cancelRef.current?.("disposed"), []);

  /** Keyboard and balance commands are fenced like a drag: to this revision and this seam. */
  const fenceFor = (target: ProjectedSeam): WindowManagerCommandFence | undefined => {
    const revision = manager.getState().snapshot?.revision;
    if (revision === undefined) return undefined;
    const { splitId, boundaryIndex } = target;
    return { expectedRevision: revision, rebase: { splitId, boundaryIndex } };
  };

  const sent = (splitId: string, outcome: WindowManagerCommandOutcome): Promise<boolean> =>
    outcome.accepted ? trackOwnCommand(manager, splitId, outcome.completion) : outcome.completion;

  const onPointerDown = (event: PointerEvent<HTMLElement>) => {
    const revision = manager.getState().snapshot?.revision;
    if (seam === undefined || revision === undefined || event.button !== 0) return;
    if (store.getSnapshot().context.phase !== "idle") return;
    // Suppresses the browser's text selection under the moving pointer.
    event.preventDefault();
    const element = event.currentTarget;
    const pointerId = event.pointerId;
    const splitId = seam.splitId;
    const vertical = seam.orientation === "vertical";
    const coordinate = (pointer: { clientX: number; clientY: number }) =>
      vertical ? pointer.clientX : pointer.clientY;
    let frame: number | null = null;
    let live = true;
    let unbindCancellation = () => {};
    let unwatchLayout = () => {};

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
      unwatchLayout();
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
     * Re-bases the gesture on the seam's latest unpreviewed layout — the seam the preview is
     * clamped against — keeping the seam on screen under the pointer, and shows it at once. When
     * the layout changed and nothing of this client is in flight on the split, the change is
     * someone else's: the drag is cancelled, as it is when the seam is gone.
     */
    const follow = (view: OsDesktopRuntimeStore, owned: boolean): boolean => {
      const drag = store.getSnapshot().context;
      if (drag.phase !== "dragging" || drag.seam === null) return false;
      const latest = authoritativeSeam(view, projectedWorkArea(), drag.seam);
      if (latest === null || view.snapshot === null) {
        cancel("context-changed");
        return false;
      }
      if (sameSeamGeometry(latest, drag.seam)) return true;
      if (!owned) {
        cancel("context-changed");
        return false;
      }
      store.trigger.rebased({ seam: latest, revision: view.snapshot.revision });
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
      if (ownPending(manager, splitId) === 0) {
        commit();
        return;
      }
      // A command of this client on the split is still in flight, and its result moves the
      // layout this release would be sent against: hold the release, previewed, until it settles,
      // then re-base on the latest layout and send.
      detach();
      showPreview();
      afterOwnCommands(manager, splitId, () => {
        if (live && follow(manager.getState(), true)) commit();
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
    });
    unbindCancellation = bindLayoutGestureCancellation(window, cancel);
    cancelRef.current = cancel;
    element.addEventListener("pointermove", onMove);
    element.addEventListener("pointerup", onUp);
    // Every snapshot under the drag is checked against the seam it is based on: its ancestors'
    // weights and its members' minimums move it as much as its own split's weights. Checked as the
    // snapshot is published, so the re-based delta is drawn in the same frame as the new layout.
    let seen = manager.getState().snapshot;
    unwatchLayout = manager.subscribe(view => {
      if (view.snapshot === seen) return;
      seen = view.snapshot;
      follow(view, ownPending(manager, splitId) > 0);
    });
    // The seam on screen may still show an earlier command of this client that has not landed:
    // base the drag on the layout its preview is clamped against, anchored where the seam shows.
    if (ownPending(manager, splitId) > 0) follow(manager.getState(), true);
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
