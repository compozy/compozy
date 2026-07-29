import { useDesktop } from "../../../hooks/use-desktop";
import { useWindowLiveDataEnabled } from "../../../hooks/use-window-live-data-enabled";

export function useSessionWindowDesktopState(windowId: string) {
  const pathname = useDesktop(state => state.windows[windowId]?.route.pathname ?? "");
  const liveTailEnabled = useWindowLiveDataEnabled(windowId);
  return { liveTailEnabled, pathname };
}
