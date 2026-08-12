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
  const readCommittedFrame = React.useEffectEvent(() => frame);
  const mergeTargeted = useSelector(
    windowManagerStore,
    snapshot => snapshot.context.deckDropTarget?.frameId === frame.id
  );

  const frameId = frame.id;
  React.useEffect(() => {
    if (!enabled) return;
    return registerWindowMergeTarget(manager, {
      frameId,
      getFrame: readCommittedFrame,
      getChrome: () => chromeRef.current,
    });
  }, [enabled, frameId, manager]);

  return { chromeRef, mergeTargeted };
}
