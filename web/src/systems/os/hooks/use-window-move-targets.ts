import { shallowEqual } from "@xstate/store";

import { orderedDesktops } from "../lib/desktop-order";
import { useDesktop } from "./use-desktop";

export interface WindowMoveTarget {
  /** Registry slot command (`window.move_to_desktop.N`, 1-based desktop order). */
  commandId: string;
  desktopId: string;
  name: string;
}

/**
 * The desktops the focused window can move to, in switcher order. The slot
 * index is the desktop's position, so the item runs the same registry command
 * as its palette row; the current desktop is left out (VC-11).
 */
export function useWindowMoveTargets(): readonly WindowMoveTarget[] {
  return useDesktop(state => {
    const current = state.activeDesktopId;
    return orderedDesktops(state.desktops)
      .slice(0, 9)
      .flatMap((desktop, index) =>
        desktop.id === current
          ? []
          : [
              {
                commandId: `window.move_to_desktop.${index + 1}`,
                desktopId: desktop.id,
                name: desktop.name,
              },
            ]
      );
  }, sameTargets);
}

function sameTargets(
  left: readonly WindowMoveTarget[] | undefined,
  right: readonly WindowMoveTarget[]
): boolean {
  if (left === undefined || left.length !== right.length) return false;
  return left.every((target, index) => shallowEqual(target, right[index]));
}
