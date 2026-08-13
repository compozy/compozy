import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import { getOsApp } from "../lib/app-registry";
import { useDesktop } from "./use-desktop";
import { useWindowLiveDataEnabled } from "./use-window-live-data-enabled";
import { useActiveWorkspace } from "@/systems/workspace";

/** Warms the canonical app queries for restored and unfocused windows. */
export function useOsAppPreload(windowId: string): void {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const appId = useDesktop(state => state.windows[windowId]?.app);
  const enabled = useWindowLiveDataEnabled(windowId);

  useEffect(() => {
    if (!enabled || !appId || !runtimeWorkspaceId) return;
    const preload = getOsApp(appId).preload;
    if (!preload) return;

    // Route preloaders settle their owned queries, so restored background
    // windows warm the same cache without creating a second error surface.
    void preload(queryClient, { workspaceId: runtimeWorkspaceId });
  }, [appId, enabled, queryClient, runtimeWorkspaceId]);
}
