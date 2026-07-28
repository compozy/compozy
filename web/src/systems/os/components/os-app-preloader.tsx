import { useOsAppPreload } from "../hooks/use-os-app-preload";

export interface OsAppPreloaderProps {
  windowId: string;
}

/**
 * Shell-level cache warmer for a logical window. Keeping this orchestration
 * outside OsWindow preserves the window component's provider-free visual and
 * gesture contract while still warming restored and background apps.
 */
export function OsAppPreloader({ windowId }: OsAppPreloaderProps) {
  useOsAppPreload(windowId);
  return null;
}
