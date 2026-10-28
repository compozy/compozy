import { shallowEqual } from "@xstate/store";

import type { OsAttentionRow } from "../lib/attention-model";
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

/** Geometry cannot change the pager's destinations or attention indicators. */
export function useDesktopPager(needsYou: readonly OsAttentionRow[]) {
  const navigation = useDesktop(
    state => ({
      activeDesktopId: state.activeDesktopId,
      desktops: state.desktops,
      canSwitchDesktop: windowManagerCommandsAvailable(state),
    }),
    shallowEqual
  );
  const needsYouDesktopIds = useDesktop(
    state => desktopIdsNeedingYou(state.windows, needsYou),
    sameDesktopIds
  );
  return { ...navigation, needsYouDesktopIds };
}

function sameDesktopIds(left: ReadonlySet<string> | undefined, right: ReadonlySet<string>) {
  if (left === undefined || left.size !== right.size) return false;
  for (const id of right) {
    if (!left.has(id)) return false;
  }
  return true;
}
