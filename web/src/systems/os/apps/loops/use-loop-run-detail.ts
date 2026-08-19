import { useLoopNodeControls } from "./use-loop-node-controls";
import { useLoopRunDetailDialogs, type LoopRunDetailDialogs } from "./use-loop-run-detail-dialogs";
import { useLoopRunPage } from "./use-loop-run-page";
import { useLoopRunRequestsState } from "./use-loop-run-requests-state";
import { useLoopRunTimetravel } from "./use-loop-run-timetravel";

const NO_INPUTS: Readonly<Record<string, unknown>> = {};

export interface UseLoopRunDetailResult {
  page: ReturnType<typeof useLoopRunPage>;
  nodeControls: ReturnType<typeof useLoopNodeControls>;
  requests: ReturnType<typeof useLoopRunRequestsState>;
  timetravel: ReturnType<typeof useLoopRunTimetravel>;
  dialogs: LoopRunDetailDialogs;
}

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
    handleKill: page.handleKill,
  });

  return { page, nodeControls, requests, timetravel, dialogs };
}
