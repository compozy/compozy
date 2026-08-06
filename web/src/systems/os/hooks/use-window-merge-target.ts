import { useSelector } from "@xstate/store-react";
import * as React from "react";

import type { OsWindowFrameModel } from "../lib/group-projection";
import { registerWindowMergeTarget } from "../lib/window-merge-target-coordinator";
import { windowManagerStore } from "../stores/window-manager-store";
import { useOsShell } from "./use-os-shell";

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

  const frameId = frame.id;
  React.useEffect(() => {
    if (!enabled || !gestureElsewhere) return;
    return registerWindowMergeTarget(manager, {
      frameId,
      getFrame: () => frameRef.current,
      getChrome: () => chromeRef.current,
    });
  }, [enabled, frameId, gestureElsewhere, manager]);

  return { chromeRef, mergeTargeted };
}
