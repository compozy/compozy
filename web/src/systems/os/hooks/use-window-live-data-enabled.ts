import { use } from "react";

import { useDocumentVisible } from "@/hooks/use-document-visible";

import { WindowLiveDataContext } from "../contexts/window-live-data-context";
import { useDesktop } from "./use-desktop";

/** Defaults to live outside the retained-window OS shell. */
export function useCurrentWindowLiveDataEnabled(): boolean {
  return use(WindowLiveDataContext);
}

/** A retained OS window owns live-data connections only while it is actually visible. */
export function useWindowLiveDataEnabled(windowId: string): boolean {
  const documentVisible = useDocumentVisible();
  const windowVisible = useDesktop(state => {
    const desktopWindow = state.windows[windowId];
    return (
      desktopWindow !== undefined &&
      state.activeDesktopId === desktopWindow.desktopId &&
      !desktopWindow.minimized &&
      desktopWindow.stackActive
    );
  });
  return documentVisible && windowVisible;
}
