import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useEffectEvent, useState } from "react";

import { createStreamEventSource } from "@/lib/ticketed-event-source";

import { cmdPaletteKeys } from "../lib/cmd-palette-query-keys";
import {
  openCmdPaletteStream,
  type CmdPaletteEventSourceFactory,
  type CmdPaletteStreamStatus,
} from "../lib/cmd-palette-stream";

export interface UseCmdPaletteStreamOptions {
  readonly workspaceId: string | null;
  readonly enabled?: boolean;
  readonly eventSourceFactory?: CmdPaletteEventSourceFactory;
}

/**
 * Keeps the projection converged on the daemon's catalog revision.
 *
 * The stream carries no replay cursor by design, so both a change event and a
 * reconnect resolve the same way: invalidate the workspace's catalog reads and
 * let the refetch settle the revision.
 */
export function useCmdPaletteStream({
  workspaceId,
  enabled = true,
  eventSourceFactory,
}: UseCmdPaletteStreamOptions): CmdPaletteStreamStatus | "disabled" {
  const queryClient = useQueryClient();
  const workspace = workspaceId?.trim() ?? "";
  const [status, setStatus] = useState<CmdPaletteStreamStatus | "disabled">("disabled");
  // The factory is a seam for tests and for the desktop shell, not a reason to
  // reconnect: a caller passing an inline function would otherwise tear the
  // socket down and rebuild it on every render.
  const createSource = useEffectEvent((url: string) =>
    (eventSourceFactory ?? createStreamEventSource)(url)
  );

  useEffect(() => {
    if (!enabled || workspace === "") return undefined;
    const reconcile = () => {
      // Every client key under this workspace: the catalog is keyed by the
      // attachment whose context resolved it, and all of them just went stale.
      void queryClient.invalidateQueries({
        queryKey: cmdPaletteKeys.workspaceCatalogs(workspace),
      });
    };
    const close = openCmdPaletteStream(
      workspace,
      { onCatalogChanged: reconcile, onReconcile: reconcile, onStatusChange: setStatus },
      createSource
    );
    return () => {
      close();
    };
  }, [enabled, queryClient, workspace]);

  return enabled && workspace !== "" ? status : "disabled";
}
