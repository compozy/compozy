import { useDesktop } from "./use-desktop";

/** A retained OS window owns long-lived live-data connections only while visible and focused. */
export function useWindowLiveDataEnabled(windowId: string): boolean {
  return useDesktop(state => {
    const window = state.windows[windowId];
    return (
      window !== undefined &&
      state.focusedId === windowId &&
      state.activeDesktopId === window.desktopId &&
      !window.minimized
    );
  });
}
