import { shallowEqual } from "@xstate/store";

import type { OsAttentionRow } from "../lib/attention-model";
import type { LayoutDesktop } from "../lib/window-manager-types";
import { desktopIdsNeedingYou } from "../lib/desktop-pager-attention";
import { windowManagerCommandsAvailable } from "../lib/window-manager-command-availability";
import { useDesktop } from "./use-desktop";

/** Selector-stable shell projection for wallpaper and mounted window hosts. */
export function useDesktopShellState() {
  const appearance = useDesktop(
    state => ({
      wallpaper: state.wallpaper,
      presentation: state.presentation,
    }),
    shallowEqual
  );
  const windowIds = useDesktop(state => Object.keys(state.windows), shallowEqual);
  return { ...appearance, windowIds };
}

type DesktopDestination = Pick<LayoutDesktop, "id" | "name">;

function sameDestinations(
  left: readonly DesktopDestination[] | undefined,
  right: readonly DesktopDestination[]
) {
  return (
    left !== undefined &&
    left.length === right.length &&
    left.every((desktop, index) => {
      const next = right[index];
      return next !== undefined && desktop.id === next.id && desktop.name === next.name;
    })
  );
}

/** Tab topology and geometry cannot change the pager's destinations or attention indicators. */
export function useDesktopPager(needsYou: readonly OsAttentionRow[]) {
  const desktops = useDesktop(
    state => state.desktops.map(({ id, name }) => ({ id, name })),
    sameDestinations
  );
  const navigation = useDesktop(
    state => ({
      activeDesktopId: state.activeDesktopId,
      canSwitchDesktop: windowManagerCommandsAvailable(state),
    }),
    shallowEqual
  );
  const needsYouDesktopIds = useDesktop(
    state => desktopIdsNeedingYou(state.windows, needsYou),
    sameDesktopIds
  );
  return { ...navigation, desktops, needsYouDesktopIds };
}

function sameDesktopIds(left: ReadonlySet<string> | undefined, right: ReadonlySet<string>) {
  if (left === undefined || left.size !== right.size) return false;
  for (const id of right) {
    if (!left.has(id)) return false;
  }
  return true;
}
