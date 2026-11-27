import { normalizeComputerId, type ComputerId } from "@/lib/computer-scope";
import { sessionBoardDrawerOpen } from "@/systems/session";

import { useDesktop } from "../../../hooks/use-desktop";
import { useWindowLiveDataEnabled } from "../../../hooks/use-window-live-data-enabled";

export function useSessionWindowDesktopState(windowId: string) {
  const pathname = useDesktop(state => state.windows[windowId]?.route.pathname ?? "");
  const drawerOpen = useDesktop(state =>
    sessionBoardDrawerOpen(state.windows[windowId]?.route.search)
  );
  const computerId: ComputerId = useDesktop(state => {
    const value = state.windows[windowId]?.route.search.computer;
    return normalizeComputerId(typeof value === "string" ? value : undefined);
  });
  const liveTailEnabled = useWindowLiveDataEnabled(windowId);
  return { computerId, drawerOpen, liveTailEnabled, pathname };
}
