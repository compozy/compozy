import { shallowEqual } from "@xstate/store";

import { orderedDesktops } from "../lib/desktop-order";
import { useDesktop } from "./use-desktop";

/** Desktops 1–9 carry a numbered registry command (and its keyboard chord). */
export const MOVE_TO_DESKTOP_SLOT_COUNT = 9;

export function moveToDesktopSlotCommandId(slot: number): string {
  return `window.move_to_desktop.${slot}`;
}

export interface WindowMoveTarget {
  /**
   * Registry slot command (`window.move_to_desktop.N`, 1-based desktop order)
   * for the first nine desktops; `null` past them, where the destination moves
   * the window by desktop id instead.
   */
  slotCommandId: string | null;
  desktopId: string;
  name: string;
}

/**
 * Every desktop the focused window can move to, in switcher order. A slot
 * destination runs the same registry command as its palette row; the current
 * desktop is left out (VC-11).
 */
export function useWindowMoveTargets(): readonly WindowMoveTarget[] {
  return useDesktop(state => {
    const current = state.activeDesktopId;
    return orderedDesktops(state.desktops).flatMap((desktop, index) =>
      desktop.id === current
        ? []
        : [
            {
              slotCommandId:
                index < MOVE_TO_DESKTOP_SLOT_COUNT ? moveToDesktopSlotCommandId(index + 1) : null,
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
