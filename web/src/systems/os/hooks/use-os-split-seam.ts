import { useSelector, useStore } from "@xstate/store-react";
import { useEffect, useLayoutEffect, useRef, type KeyboardEvent, type PointerEvent } from "react";

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
import {
  authoritativeSeam,
  authoritativeSeamShift,
  authoritativeSeamSplitFitsEven,
  authoritativeSplitMembers,
  DEFAULT_WINDOW_MANAGER_WORK_AREA,
} from "../lib/window-manager-view";
import type { PixelRect, ProjectedSeam } from "../lib/window-manager-types";
import { commandClockNow } from "../runtime/window-manager-runtime-helpers";
import { windowManagerStore } from "../stores/window-manager-store";
import {
  SEAM_WEIGHT_EPSILON,
  seamDragCommit,
  seamDragLogic,
  seamKeyStep,
  type SeamShiftAdmission,
} from "./seam-drag-store";
import {
  afterOwnCommands,
  currentSeamFrame,
  formatSeamFrame,
  ownBusy,
  ownPending,
  seamInputHolds,
  trackOwnCommand,
  type SeamInputScope,
  type SeamKeyStep,
} from "./seam-command-queue";
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
 * Whether two layouts of a seam separate the same two sides. A boundary index names a different
 * pair once a member is added or removed before it.
 */
function samePair(left: ProjectedSeam, right: ProjectedSeam): boolean {
  const same = (a: readonly string[], b: readonly string[]) =>
    a.length === b.length && a.every((id, index) => id === b[index]);
  return (
    same(left.leadingWindowIds, right.leadingWindowIds) &&
    same(left.trailingWindowIds, right.trailingWindowIds)
  );
}

/** Every published input `authoritativeSeam` projects from, by identity. */
function geometryInputs(view: OsDesktopRuntimeStore): readonly unknown[] {
  return [
    view.snapshot,
    view.windowManagerConfig,
    view.client,
    view.presentation,
    projectedWorkArea(),
  ];
}

function sameInputs(left: readonly unknown[], right: readonly unknown[]): boolean {
  return left.every((value, index) => value === right[index]);
}

/** The frame (binding, shown desktop and tab, work area) seam input is made in, as rendered. */
function useSeamGestureFrame(): string {
  const binding = useSelector(windowManagerStore, snapshot => snapshot.context.binding);
  const rect = useSelector(windowManagerStore, snapshot => snapshot.context.workArea?.rect);
  const desktopId = useDesktop(state => state.activeDesktopId);
  const tabId = useDesktop(state => state.activeTabIds[state.activeDesktopId ?? ""] ?? "");
  return formatSeamFrame(
    binding,
    { activeDesktopId: desktopId, activeTabIds: { [desktopId ?? ""]: tabId } },
    rect
  );
}

export type OsSplitSeamState = "drag" | "limit" | undefined;

/**
 * One seam's gesture. Every command it sends — a released drag, an arrow-key step, a balance — is
 * authored when it is handed to the manager, against the seam, member count and orientation of
 * one projection of the authoritative layout, never the rendered (possibly previewed) seam. A
 * command made while this client has a seam command in flight — on any split, since a seam's
 * geometry depends on the splits around it — waits behind it, strictly one at a time, and keeps
 * the scope and time of its input: it is dropped when the binding or the shown frame changed, its
 * seam unmounted or its deadline passed (on the monotonic command clock). Arrow-key steps on one
 * seam that wait back to back merge into one command. The command carries its input time, so the
 * runtime's deadline is what is left of the same one.
 *
 * A pointer drag previews the two sides' shares through the shared seam preview, stops at either
 * side's pixel minimum and commits one `layout.resize`. Escape, a cancelled or lost pointer, a
 * frame change, the seam gone or turned, its layout changed by someone else while no command
 * of this client is in flight, and unmounting cancel it and restore the shares; with an own
 * command in flight (a seam command or any other), a layout change re-bases it instead.
 */
export function useOsSplitSeam(seam: ProjectedSeam | undefined) {
  const { manager } = useOsShell();
  const store = useStore(seamDragLogic);
  // The limit state of the frame on screen, so it appears with the clamp, never a frame early.
  const state = useSelector(store, (snapshot): OsSplitSeamState => {
    if (snapshot.context.phase !== "dragging") return undefined;
    return snapshot.context.limitShown ? "limit" : "drag";
  });
  const contextKey = useSeamGestureFrame();
  const cancelRef = useRef<((reason: GestureCancelReason) => void) | null>(null);
  /** The live drag's check of a published layout against the seam it is based on. */
  const watchRef = useRef<((view: OsDesktopRuntimeStore) => void) | null>(null);
  const mountedRef = useRef(true);
  const dragging = state !== undefined;

  // Every publication under a drag — held release included — that changes an input of the seam's
  // projection (snapshot, effective config, client, presentation, work area) is checked against
  // the seam the drag is based on; the hook's own preview publications change none of them. The
  // check runs as the view is published, so a re-based delta is drawn in the same frame as the new
  // layout; attached before paint, it first catches up on anything published since the press.
  useLayoutEffect(() => {
    if (!dragging) return;
    const check = (view: OsDesktopRuntimeStore) => watchRef.current?.(view);
    check(manager.getState());
    return manager.subscribe(check);
  }, [dragging, manager]);

  // A drag outlives neither its seam nor the frame it was drawn in.
  const seamId = seam?.id;
  useEffect(() => {
    const drag = store.getSnapshot().context;
    if (drag.phase !== "dragging") return;
    if (drag.seam === null || drag.seam.id !== seamId || drag.contextKey !== contextKey) {
      cancelRef.current?.("context-changed");
    }
  }, [store, seamId, contextKey]);
  useEffect(() => {
    mountedRef.current = true;
    return () => {
      mountedRef.current = false;
      cancelRef.current?.("disposed");
    };
  }, []);

  /** Where and when an input is made, for a command that may have to wait. */
  const inputScope = (): SeamInputScope => ({
    frame: currentSeamFrame(manager),
    at: commandClockNow(),
    mounted: () => mountedRef.current,
  });

  /**
   * Commands are fenced to the revision they are authored at and to their seam, and carry the
   * time of their input.
   */
  const fenceFor = (
    target: ProjectedSeam,
    inputAt: number
  ): WindowManagerCommandFence | undefined => {
    const revision = manager.getState().snapshot?.revision;
    if (revision === undefined) return undefined;
    const { splitId, boundaryIndex } = target;
    return { expectedRevision: revision, rebase: { splitId, boundaryIndex }, inputAt };
  };

  const sent = (outcome: WindowManagerCommandOutcome): Promise<boolean> =>
    outcome.accepted ? trackOwnCommand(manager, outcome.completion) : outcome.completion;

  /**
   * Sends a keyboard or balance command authored when it is sent: now, or behind this client's
   * seam commands. It reads the seam and the split's member count from the authoritative layout
   * at that moment; a seam that is gone by then gets nothing.
   */
  const sendSeamCommand = (
    target: ProjectedSeam,
    author: (current: ProjectedSeam, at: { members: number; inputAt: number }) => void,
    step?: SeamKeyStep
  ) => {
    const scope = inputScope();
    afterOwnCommands(manager, {
      scope,
      ...(step === undefined ? {} : { step }),
      run: () => {
        const view = manager.getState();
        const current = authoritativeSeam(view, projectedWorkArea(), target);
        const members = authoritativeSplitMembers(view.snapshot, target.splitId);
        if (current === null || members === null || !samePair(current, target)) return;
        author(current, { members, inputAt: scope.at });
      },
    });
  };

  const onPointerDown = (event: PointerEvent<HTMLElement>) => {
    const pressed = manager.getState();
    const revision = pressed.snapshot?.revision;
    if (seam === undefined || revision === undefined || event.button !== 0) return;
    if (store.getSnapshot().context.phase !== "idle") return;
    // The drag is based on the authoritative seam; the rendered one may carry a preview.
    const base = authoritativeSeam(pressed, projectedWorkArea(), seam);
    if (base === null || base.orientation !== seam.orientation || !samePair(base, seam)) return;
    // Suppresses the browser's text selection under the moving pointer.
    event.preventDefault();
    const element = event.currentTarget;
    const pointerId = event.pointerId;
    const vertical = seam.orientation === "vertical";
    const coordinate = (pointer: { clientX: number; clientY: number }) =>
      vertical ? pointer.clientX : pointer.clientY;
    // Every frame's shift goes through the one rule the rendered preview does, against the
    // authoritative layout as it is when the frame is made.
    const admit: SeamShiftAdmission = requestedPx =>
      authoritativeSeamShift(manager.getState(), projectedWorkArea(), base, requestedPx)?.shiftPx ??
      0;
    let frame: number | null = null;
    let live = true;
    let unbindCancellation = () => {};
    // The projection inputs this drag has checked last, starting with those it was pressed on.
    let seen = geometryInputs(pressed);
    const watch = (view: OsDesktopRuntimeStore) => {
      const inputs = geometryInputs(view);
      if (sameInputs(inputs, seen)) return;
      seen = inputs;
      // Any command of this client in flight — a seam command or not (a palette balance, a
      // move) — may be what changed the layout: the drag is re-based on it, not cancelled.
      follow(view, ownPending(manager) > 0 || manager.commandsInFlight());
    };

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
      if (watchRef.current === watch) watchRef.current = null;
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
      store.trigger.framePresented({});
      windowManagerStore.trigger.seamPreviewSet({
        preview: {
          splitId: drag.seam.splitId,
          boundaryIndex: drag.seam.boundaryIndex,
          // The shift the pointer asks for: the preview admits it by the same rule `shownPx` came
          // from, against the layout it renders.
          deltaPx: drag.deltaPx,
        },
      });
    };
    /**
     * Holds the drag to the seam's latest unpreviewed layout — the seam the preview is clamped
     * against. Same geometry: re-admitted and shown. Changed while this client has a command that
     * can move the layout in flight, seam command or not, or while its release is held behind one
     * (`owned`): re-based, the seam on screen kept under the pointer, and shown at once.
     * Changed otherwise, turned to the other axis, separating another pair, or gone: cancelled.
     */
    const follow = (view: OsDesktopRuntimeStore, owned: boolean): boolean => {
      const drag = store.getSnapshot().context;
      if (drag.phase !== "dragging" || drag.seam === null) return false;
      const latest = authoritativeSeam(view, projectedWorkArea(), drag.seam);
      if (latest === null || view.snapshot === null) {
        cancel("context-changed");
        return false;
      }
      if (!samePair(latest, drag.seam)) {
        cancel("context-changed");
        return false;
      }
      if (sameSeamGeometry(latest, drag.seam)) {
        // Shown with the layout that changed it, the preview and its limit state together.
        store.trigger.readmitted({ admit });
        showPreview();
        return true;
      }
      if (!owned || latest.orientation !== drag.seam.orientation) {
        cancel("context-changed");
        return false;
      }
      store.trigger.rebased({ seam: latest, revision: view.snapshot.revision, admit });
      showPreview();
      return true;
    };
    /** Authors the release (made at `inputAt`) against the layout as it is now, then sends it. */
    const commit = (owned: boolean, inputAt: number) => {
      if (!follow(manager.getState(), owned)) return;
      const result = seamDragCommit(store.getSnapshot().context);
      teardown();
      store.trigger.dragEnded({ pointerId });
      if (result === null) {
        if (generation === previewGeneration) clearPreview();
        return;
      }
      // The preview stays until the command settles, so the panes never flash back; a later
      // drag that has taken the preview over keeps it.
      const fence = { ...result.fence, inputAt };
      void sent(
        manager.resizeLayout(result.splitId, result.boundaryIndex, result.delta, fence)
      ).finally(() => {
        if (generation === previewGeneration) clearPreview();
      });
    };
    const onMove = (move: globalThis.PointerEvent) => {
      if (move.pointerId !== pointerId) return;
      store.trigger.pointerMoved({ pointerId, coordinate: coordinate(move), admit });
      if (frame !== null) return;
      frame = requestAnimationFrame(() => {
        frame = null;
        showPreview();
      });
    };
    const onUp = (up: globalThis.PointerEvent) => {
      if (up.pointerId !== pointerId) return;
      store.trigger.pointerMoved({ pointerId, coordinate: coordinate(up), admit });
      // The release frame is the last one shown, and the commit sends its shift.
      showPreview();
      const scope = inputScope();
      if (!ownBusy(manager) && !manager.commandsInFlight()) {
        commit(false, scope.at);
        return;
      }
      // A command of this client is in flight (or a seam command queued), and its result may
      // move the layout this release would be sent against: hold the release, previewed, in line
      // behind the seam commands and until every own command has settled; then author it against
      // the latest layout and send — or cancel it if its input no longer holds by then.
      detach();
      const drop = () => {
        if (live) cancel("context-changed");
      };
      // Waiting only on a non-seam command, the release still takes its place in the seam queue,
      // so seam input made after it (an arrow step, another seam's release) waits behind it.
      if (!ownBusy(manager)) {
        void trackOwnCommand(
          manager,
          manager.commandsSettled().then(() => true)
        );
      }
      const afterOwnSettled = () => {
        if (!live) return;
        if (!manager.commandsInFlight()) {
          commit(true, scope.at);
          return;
        }
        // Still counted as an own seam command while it waits, so the seam queue stays blocked
        // until this release commits or drops.
        const waited = manager.commandsSettled().then(() => {
          if (!live) return true;
          if (seamInputHolds(scope, currentSeamFrame(manager), commandClockNow())) {
            afterOwnSettled();
          } else drop();
          return true;
        });
        void trackOwnCommand(manager, waited);
      };
      afterOwnCommands(manager, { scope, run: afterOwnSettled, drop });
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
    // Anchored where the seam shows, based on where the layout really has it.
    if (!sameSeamGeometry(base, seam)) {
      store.trigger.rebased({ seam: base, revision, admit });
      showPreview();
    }
    unbindCancellation = bindLayoutGestureCancellation(window, cancel);
    cancelRef.current = cancel;
    element.addEventListener("pointermove", onMove);
    element.addEventListener("pointerup", onUp);
    watchRef.current = watch;
  };

  const onKeyDown = (event: KeyboardEvent<HTMLElement>) => {
    if (seam === undefined) return;
    const direction = keyDirection(event.key);
    if (direction === null) return;
    event.preventDefault();
    // Key repeat while earlier steps wait merges into the waiting step on this seam.
    const step: SeamKeyStep = { seamId: seam.id, weight: direction * seamKeyStep(event) };
    sendSeamCommand(
      seam,
      (current, { inputAt }) => {
        // The same admission a drag frame goes through, against the layout at send time: a step
        // never leaves the split one member at a time, so the focused seam stays mounted.
        const admitted = authoritativeSeamShift(
          manager.getState(),
          projectedWorkArea(),
          current,
          step.weight * current.axisSpan
        );
        if (admitted === null) return;
        const delta = seamWeightDelta(admitted.seam, admitted.shiftPx);
        if (Math.abs(delta) <= SEAM_WEIGHT_EPSILON) return;
        const fence = fenceFor(current, inputAt);
        void sent(manager.resizeLayout(current.splitId, current.boundaryIndex, delta, fence));
      },
      step
    );
  };

  /**
   * Balances the two sides this seam separates, against the layout at send time, so a balance
   * never leaves the split one member at a time. With two members, `layout.balance` when the split
   * fits with the exactly equal weights it writes (an odd span's tie goes to the first member, so
   * the fit is tested on those weights, not on a shift that only nears them). Otherwise, and for
   * a pair in a wider split (the others keep theirs), `layout.resize` by the evening shift as a
   * key step admits it: the nearest shift that fits. The member count is the split's when the
   * command is sent; a seam that by then separates another pair gets nothing.
   */
  const onDoubleClick = () => {
    if (seam === undefined) return;
    sendSeamCommand(seam, (current, { members, inputAt }) => {
      const view = manager.getState();
      const workArea = projectedWorkArea();
      const fence = fenceFor(current, inputAt);
      if (members <= 2 && authoritativeSeamSplitFitsEven(view, workArea, current) === true) {
        void sent(manager.balanceLayout({ splitId: current.splitId }, fence));
        return;
      }
      const requestedPx = ((current.trailingWeight - current.leadingWeight) / 2) * current.axisSpan;
      const admitted = authoritativeSeamShift(view, workArea, current, requestedPx);
      if (admitted === null) return;
      const delta = seamWeightDelta(admitted.seam, admitted.shiftPx);
      if (Math.abs(delta) <= SEAM_WEIGHT_EPSILON) return;
      void sent(manager.resizeLayout(current.splitId, current.boundaryIndex, delta, fence));
    });
  };

  return { state, onPointerDown, onKeyDown, onDoubleClick };
}
