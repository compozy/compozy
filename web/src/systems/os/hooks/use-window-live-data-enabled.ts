import { useDesktop } from "./use-desktop";

/** A retained OS window owns long-lived live-data connections only while visible and focused. */
export function useWindowLiveDataEnabled(windowId: string): boolean {
  return useDesktop(state => {
    const desktopWindow = state.windows[windowId];
    return (
      desktopWindow !== undefined &&
      state.focusedId === windowId &&
      state.activeDesktopId === desktopWindow.desktopId &&
      !desktopWindow.minimized
    );
  });
}
