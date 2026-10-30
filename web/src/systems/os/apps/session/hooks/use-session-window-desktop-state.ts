import { sessionBoardDrawerOpen } from "@/systems/session/lib/session-deeplink-search";

import { useDesktop } from "../../../hooks/use-desktop";
import { useWindowLiveDataEnabled } from "../../../hooks/use-window-live-data-enabled";

export function useSessionWindowDesktopState(windowId: string) {
  const pathname = useDesktop(state => state.windows[windowId]?.route.pathname ?? "");
  const drawerOpen = useDesktop(state =>
    sessionBoardDrawerOpen(state.windows[windowId]?.route.search)
  );
  const liveTailEnabled = useWindowLiveDataEnabled(windowId);
  return { drawerOpen, liveTailEnabled, pathname };
}
