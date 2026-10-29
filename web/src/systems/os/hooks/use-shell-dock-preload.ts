import { useCallback } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { profileViewKey, readProfileView, useProfileReadScope } from "@/systems/profiles";
import { preloadSessionEntry } from "@/systems/session";
import { useActiveWorkspace } from "@/systems/workspace";

import { getOsApp, preloadOsAppCode } from "../lib/app-registry";
import type { DockPreload } from "./use-desktop-dock";

/** Warms an app's code and first read on rail intent, within the current profile lens. */
export function useShellDockPreload(): DockPreload {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId, pending } = useActiveWorkspace();
  const { key: profileKey } = useProfileReadScope();
  return useCallback<DockPreload>(
    (appId, session) => {
      if (pending || profileKey !== profileViewKey(readProfileView(queryClient))) return;
      const reads: Promise<unknown>[] = [preloadOsAppCode(appId)];
      if (appId === "session" && session !== null) {
        reads.push(preloadSessionEntry(queryClient, session));
      } else {
        const preload = getOsApp(appId).preload;
        if (preload) reads.push(preload(queryClient, { workspaceId: runtimeWorkspaceId }));
      }
      return Promise.allSettled(reads);
    },
    [pending, profileKey, queryClient, runtimeWorkspaceId]
  );
}
