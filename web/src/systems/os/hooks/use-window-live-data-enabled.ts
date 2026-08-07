import { useDesktop } from "./use-desktop";

/** A retained OS window owns live-data connections only while it is actually visible. */
export function useWindowLiveDataEnabled(windowId: string): boolean {
  return useDesktop(state => {
    const desktopWindow = state.windows[windowId];
    return (
      desktopWindow !== undefined &&
      state.activeDesktopId === desktopWindow.desktopId &&
      !desktopWindow.minimized &&
      desktopWindow.stackActive
    );
  });
}
