import { useNavigate } from "@tanstack/react-router";

import { useCurrentWindowLiveDataEnabled } from "../../hooks/use-window-live-data-enabled";
import { useLoop, useLoopConfig, useLoopRuns } from "@/systems/loops";
import { useLoopWindowScope } from "./hooks/use-loop-window-scope";

/**
 * View-model for the run-form route: the loop definition, its effective config, and
 * the run of this loop that is already live.
 *
 * The live run is a server-filtered read (`live=true`) — the daemon owns which
 * statuses count as live, so the form never re-derives that from a status list.
 */
export function useLoopRunFormPage(name: string) {
  const navigate = useNavigate();
  const { scope, workspaceId, search } = useLoopWindowScope();
  const liveDataEnabled = useCurrentWindowLiveDataEnabled();
  const enabled = workspaceId !== "" && liveDataEnabled;
  const loopQuery = useLoop(workspaceId, name, enabled);
  const configQuery = useLoopConfig(workspaceId, name, enabled);
  const liveRunsQuery = useLoopRuns(workspaceId, { limit: 1, live: true, loop: name }, enabled);
  return {
    activeRun: liveRunsQuery.data?.runs[0] ?? null,
    configQuery,
    loopQuery,
    /**
     * The live-run read has not answered. It decides whether a notice leads the form, so the
     * form waits for it rather than shifting down when the notice arrives.
     */
    liveRunPending: liveRunsQuery.isPending,
    openLoop: () => void navigate({ to: "/loops/$name", params: { name }, search }),
    openLoops: () => void navigate({ to: "/loops" }),
    openRun: (runId: string) =>
      void navigate({ to: "/loop-runs/$runId", params: { runId }, search }),
    scope,
    workspaceId,
  };
}
