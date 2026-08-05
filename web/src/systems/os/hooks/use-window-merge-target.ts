import { useSelector } from "@xstate/store-react";
import * as React from "react";

import type { OsWindowFrameModel } from "../lib/group-projection";
import type { OsDesktopRuntimeStore, OsRect } from "../lib/os-types";
import { windowManagerStore } from "../stores/window-manager-store";
import { useAnimationFrameLatest } from "./use-animation-frame-latest";
import { useOsShell } from "./use-os-shell";

// Mirrors the deck row's hit forgiveness so both merge affordances feel alike.
const HEAD_HIT_SLOP_TOP = 4;
const HEAD_HIT_SLOP_BOTTOM = 8;

function pointInRect(point: { x: number; y: number }, rect: OsRect): boolean {
  return (
    point.x >= rect.x &&
    point.x <= rect.x + rect.w &&
    point.y >= rect.y &&
    point.y <= rect.y + rect.h
  );
}

/** A higher floating frame over the pointer occludes this head visually. */
function coveredByHigherFrame(
  state: OsDesktopRuntimeStore,
  frame: OsWindowFrameModel,
  draggedWindowId: string,
  layerPoint: { x: number; y: number }
): boolean {
  const frames = state.frames[frame.desktopId] ?? [];
  return frames.some(
    other =>
      other.id !== frame.id &&
      other.kind === "floating" &&
      !other.minimized &&
      !other.members.includes(draggedWindowId) &&
      other.layer > frame.layer &&
      pointInRect(layerPoint, other.rect)
  );
}

function updateSoloMergeTarget(input: {
  frame: OsWindowFrameModel;
  state: OsDesktopRuntimeStore;
  chrome: HTMLElement | null;
  clientX: number;
  clientY: number;
}): void {
  const { frame, state, chrome, clientX, clientY } = input;
  const storeContext = windowManagerStore.getSnapshot().context;
  const gesture = storeContext.gesture;
  if (gesture?.status !== "active" || frame.members.includes(gesture.source.windowId) || !chrome) {
    windowManagerStore.trigger.deckDropCleared({ frameId: frame.id });
    return;
  }
  const head = chrome.querySelector('[data-slot="os-window-head"]');
  if (!(head instanceof HTMLElement)) {
    windowManagerStore.trigger.deckDropCleared({ frameId: frame.id });
    return;
  }
  const box = head.getBoundingClientRect();
  const inside =
    clientX >= box.left &&
    clientX <= box.right &&
    clientY >= box.top - HEAD_HIT_SLOP_TOP &&
    clientY <= box.bottom + HEAD_HIT_SLOP_BOTTOM;
  const origin = storeContext.workArea?.origin ?? { x: 0, y: 0 };
  const layerPoint = { x: clientX - origin.x, y: clientY - origin.y };
  if (!inside || coveredByHigherFrame(state, frame, gesture.source.windowId, layerPoint)) {
    windowManagerStore.trigger.deckDropCleared({ frameId: frame.id });
    return;
  }
  windowManagerStore.trigger.deckDropTargeted({
    target: {
      frameId: frame.id,
      targetWindowId: frame.activeWindowId,
      insertIndex: frame.members.length,
    },
  });
}

export interface WindowMergeTargetModel {
  chromeRef: React.RefObject<HTMLElement | null>;
  /** This frame is the advertised merge drop of the active window drag. */
  mergeTargeted: boolean;
}

/**
 * Head-drop counterpart of the deck merge zone (US-001): a frame without a
 * deck advertises its head as the `window.stack.group` target while another
 * frame's drag hovers it, so two solo windows can fold into one tab group.
 * The store owns the drop-target fact; the drag commit reads it at release.
 */
export function useWindowMergeTarget(
  frame: OsWindowFrameModel,
  enabled: boolean
): WindowMergeTargetModel {
  const { manager } = useOsShell();
  const chromeRef = React.useRef<HTMLElement | null>(null);
  // The frame projection is reallocated every publish; the listener reads the
  // latest through a ref so mid-drag re-renders never tear the drop target
  // down. A mounted frame keeps its id, so the cleanup clear stays scoped.
  const frameRef = React.useRef(frame);
  const gestureElsewhere = useSelector(windowManagerStore, snapshot => {
    const gesture = snapshot.context.gesture;
    return gesture?.status === "active" && !frame.members.includes(gesture.source.windowId);
  });
  const mergeTargeted = useSelector(
    windowManagerStore,
    snapshot => snapshot.context.deckDropTarget?.frameId === frame.id
  );

  React.useEffect(() => {
    frameRef.current = frame;
  }, [frame]);

  const { schedule: schedulePointerFrame, cancel: cancelPointerFrame } = useAnimationFrameLatest(
    (point: { clientX: number; clientY: number }) => {
      updateSoloMergeTarget({
        frame: frameRef.current,
        state: manager.getState(),
        chrome: chromeRef.current,
        clientX: point.clientX,
        clientY: point.clientY,
      });
    }
  );

  const frameId = frame.id;
  React.useEffect(() => {
    if (!enabled || !gestureElsewhere) return;
    const onPointerMove = (event: globalThis.PointerEvent) => {
      schedulePointerFrame({ value: { clientX: event.clientX, clientY: event.clientY } });
    };
    window.addEventListener("pointermove", onPointerMove);
    return () => {
      window.removeEventListener("pointermove", onPointerMove);
      cancelPointerFrame();
      windowManagerStore.trigger.deckDropCleared({ frameId });
    };
  }, [cancelPointerFrame, enabled, frameId, gestureElsewhere, manager, schedulePointerFrame]);

  return { chromeRef, mergeTargeted };
}
