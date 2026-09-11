import { useGoalTurns, type GoalTurnsRead, useLoopRunEventsRead } from "@/systems/loops";
import { useLoopNodeControls } from "./use-loop-node-controls";
import { useLoopRunDetailDialogs, type LoopRunDetailDialogs } from "./use-loop-run-detail-dialogs";
import { useLoopRunPage } from "./use-loop-run-page";
import { useLoopRunRequestsState } from "./use-loop-run-requests-state";
import { useLoopRunTimetravel } from "./use-loop-run-timetravel";

const NO_INPUTS: Readonly<Record<string, unknown>> = {};

export interface UseLoopRunDetailResult {
  goalTurns?: GoalTurnsRead;
  page: ReturnType<typeof useLoopRunPage>;
  nodeControls: ReturnType<typeof useLoopNodeControls>;
  requests: ReturnType<typeof useLoopRunRequestsState>;
  timetravel: ReturnType<typeof useLoopRunTimetravel>;
  dialogs: LoopRunDetailDialogs;
  events: ReturnType<typeof useLoopRunEventsRead>;
}

/** Coordinates run state, node controls, requests, time travel, and Goal history reads. */
export function useLoopRunDetail(
  workspaceId: string,
  runId: string,
  options: { liveDataEnabled: boolean }
): UseLoopRunDetailResult {
  const page = useLoopRunPage(workspaceId, runId, { liveDataEnabled: options.liveDataEnabled });
  const nodeControls = useLoopNodeControls(workspaceId, runId, {
    definition: page.definition,
    graph: page.graph,
    runStatus: page.run?.status,
    isGenerationBusy: page.isGenerationBusy,
    amendments: page.amendments,
  });
  const requests = useLoopRunRequestsState(workspaceId, runId);
  const timetravel = useLoopRunTimetravel({
    workspaceId,
    runId,
    loopName: page.run?.loop_name ?? "",
    generations: page.generations,
    inputSchema: page.inputSchema,
    sourceInputs: page.run?.inputs ?? NO_INPUTS,
  });
  const dialogs = useLoopRunDetailDialogs({
    resetRunControlErrors: page.resetRunControlErrors,
    handleCancel: page.handleCancel,
  });
  // The Events lane's raw `view=all` read, started only once Inspect is open —
  // which is the only place that lane exists. It is composed here rather than in
  // `useLoopRunPage` because the disclosure state is owned by `dialogs`, and
  // because the page hook must keep exactly one timeline read driving the stream
  // seam. This one never touches `useLoopStream`.
  const events = useLoopRunEventsRead(
    workspaceId,
    runId,
    options.liveDataEnabled && dialogs.inspectOpen
  );

  const hasGoal = page.definition?.graph.nodes.some(node => node.kind === "goal") ?? false;
  const goalTurns = useGoalTurns(
    workspaceId,
    runId,
    options.liveDataEnabled && dialogs.inspectOpen && hasGoal,
    page.isLive
  );
  return {
    page,
    nodeControls,
    requests,
    timetravel,
    dialogs,
    events,
    goalTurns: hasGoal ? goalTurns : undefined,
  };
}
