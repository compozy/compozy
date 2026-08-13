import { useNavigate } from "@tanstack/react-router";

import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import { useLoop, useLoopConfig, useLoopRuns } from "@/systems/loops";
import { useActiveWorkspace } from "@/systems/workspace";

/**
 * View-model for the run-form route: the loop definition, its effective config, and
 * the run of this loop that is already live.
 *
 * The live run is a server-filtered read (`live=true`) — the daemon owns which
 * statuses count as live, so the form never re-derives that from a status list.
 */
export function useLoopRunFormPage(name: string) {
  const navigate = useNavigate();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = runtimeWorkspaceId ?? "";
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const enabled = workspaceId !== "" && liveDataEnabled;
  const loopQuery = useLoop(workspaceId, name, enabled);
  const configQuery = useLoopConfig(workspaceId, name, enabled);
  const liveRunsQuery = useLoopRuns(workspaceId, { limit: 1, live: true, loop: name }, enabled);
  return {
    activeRun: liveRunsQuery.data?.runs[0] ?? null,
    configQuery,
    loopQuery,
    openLoop: () => void navigate({ to: "/loops/$name", params: { name } }),
    openLoops: () => void navigate({ to: "/loops" }),
    openRun: (runId: string) => void navigate({ to: "/loop-runs/$runId", params: { runId } }),
    workspaceId,
  };
}
