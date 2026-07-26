import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import { useActiveWorkspace } from "@/systems/workspace";

import { getOsApp } from "../lib/app-registry";
import { useDesktop } from "./use-desktop";

/** Warms the canonical app queries for restored and unfocused windows. */
export function useOsAppPreload(windowId: string): void {
  const queryClient = useQueryClient();
  const { activeWorkspaceId } = useActiveWorkspace();
  const appId = useDesktop(state => state.windows[windowId]?.app);

  useEffect(() => {
    if (!appId || !activeWorkspaceId) return;
    const preload = getOsApp(appId).preload;
    if (!preload) return;

    // Route preloaders settle their owned queries, so restored background
    // windows warm the same cache without creating a second error surface.
    void preload(queryClient, { workspaceId: activeWorkspaceId });
  }, [activeWorkspaceId, appId, queryClient]);
}
