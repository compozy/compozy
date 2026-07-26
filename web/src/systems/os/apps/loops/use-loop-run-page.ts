import { useReducer } from "react";

import { useNavigate } from "@tanstack/react-router";

import { toast } from "@agh/ui";

import {
  applyLoopEventFrame,
  emptyLoopRunLiveState,
  isTerminalLoopStatus,
  type LoopGateDecision,
  mergeGoalTurnTimeline,
  projectLoopRunPageView,
  useApproveLoopRun,
  useGoalTurns,
  useLoop,
  useLoopRun,
  useLoopStream,
  useNowTick,
  usePauseLoopRun,
  useResumeLoopRun,
  useRunLoop,
  useStopLoopRun,
} from "@/systems/loops";

/**
 * The live run-page view-model (redesign spec §2-§5): composes the run
 * projection (`getLoopRun`) with the pinned definition and the SSE reducer over
 * the forward event contract, then delegates every derived body field to the
 * shared `projectLoopRunPageView` so Storybook fixtures and the live page stay
 * on one derivation. Controls and the gate decision go through the sanctioned
 * mutation hooks.
 */
export function useLoopRunPage(workspaceId: string, runId: string) {
  const navigate = useNavigate();
  const enabled = workspaceId !== "" && runId !== "";
  const runQuery = useLoopRun(workspaceId, runId, enabled);
  const run = runQuery.data?.run;
  const generations = runQuery.data?.generations;
  const watchEvents = runQuery.data?.watch_events ?? undefined;
  const executedDefinition = runQuery.data?.executed_definition;
  const loopName = run?.loop_name ?? "";
  const loopQuery = useLoop(
    workspaceId,
    loopName,
    enabled && loopName !== "" && !executedDefinition
  );
  const definition = executedDefinition ?? loopQuery.data?.definition;

  const [live, dispatch] = useReducer(applyLoopEventFrame, undefined, emptyLoopRunLiveState);
  const isLive = runQuery.isSuccess && !isTerminalLoopStatus(run?.status);
  // Terminal runs replay their retained status event once so generation-zero
  // failures remain visible after navigation or reload; the hook closes on it.
  useLoopStream(workspaceId, runId, { enabled: runQuery.isSuccess, onEvent: dispatch });
  const goalTurnsQuery = useGoalTurns(workspaceId, runId, { enabled: runQuery.isSuccess });
  const goalTurns = mergeGoalTurnTimeline(goalTurnsQuery.data?.turns ?? [], live.goalTurns);

  const pauseMutation = usePauseLoopRun();
  const resumeMutation = useResumeLoopRun();
  const stopMutation = useStopLoopRun();
  const approveMutation = useApproveLoopRun();
  const runLoopMutation = useRunLoop();

  const nowMs = useNowTick(run?.status === "running");
  const view = run ? projectLoopRunPageView({ run, generations, live, definition, nowMs }) : null;
  const effectiveRun = view?.effectiveRun ?? run;

  const handlePause = () => {
    pauseMutation.mutate(
      { workspaceId, runId },
      {
        onSuccess: () => toast.success("Pause requested — pausing at the next generation boundary"),
        onError: error =>
          toast.error(error instanceof Error ? error.message : "Failed to pause run"),
      }
    );
  };

  const handleResume = () => {
    resumeMutation.mutate(
      { workspaceId, runId },
      {
        onSuccess: () => toast.success("Run resumed"),
        onError: error =>
          toast.error(error instanceof Error ? error.message : "Failed to resume run"),
      }
    );
  };

  const handleStop = () => {
    stopMutation.mutate(
      { workspaceId, runId },
      {
        onSuccess: () => toast.success("Run stopped"),
        onError: error =>
          toast.error(error instanceof Error ? error.message : "Failed to stop run"),
      }
    );
  };

  const handleDecision = (decision: LoopGateDecision, gateId: string) => {
    approveMutation.mutate(
      { workspaceId, runId, data: { decision, gate_id: gateId } },
      {
        onSuccess: () => toast.success("Decision recorded"),
        onError: error =>
          toast.error(error instanceof Error ? error.message : "Failed to record the decision"),
      }
    );
  };

  const handleStartNewRun = () => {
    if (!run) return;
    runLoopMutation.mutate(
      { workspaceId, name: run.loop_name, data: { inputs: run.inputs } },
      {
        onSuccess: result => {
          const newRunId = result.run?.id;
          if (newRunId) {
            toast.success("New run started");
            void navigate({ to: "/loop-runs/$runId", params: { runId: newRunId } });
            return;
          }
          toast.error("The daemon accepted the request but returned no run");
        },
        onError: error =>
          toast.error(error instanceof Error ? error.message : "Failed to start a new run"),
      }
    );
  };

  const version = run?.definition_version ?? loopQuery.data?.version;
  const versionLabel =
    version !== undefined
      ? executedDefinition
        ? `v${version} · pinned`
        : `v${version}`
      : undefined;
  const pendingAction = approveMutation.isPending
    ? ("approve" as const)
    : runLoopMutation.isPending
      ? ("start-new-run" as const)
      : undefined;

  return {
    runQuery,
    // `run` is the polled projection (existence/status/chrome). `effectiveRun`
    // overlays the fresher streamed token count and owns Usage + body reads —
    // the two differ only in `tokens_used`.
    run,
    effectiveRun,
    definition,
    graph: view?.graph ?? null,
    watchEvents,
    versionLabel,
    live,
    story: view?.story ?? { rows: [], now: null },
    goalIds: view?.goalIds ?? new Set<string>(),
    goalTurns,
    goalTurnsQuery,
    isLive,
    progress: view?.progress ?? null,
    usageRows: view?.usageRows ?? [],
    usageNote: view?.usageNote ?? null,
    approvalFallbackFacts: view?.approvalFallbackFacts ?? [],
    latestVerdict: view?.latestVerdict ?? null,
    subject: view?.subject ?? null,
    hasWatchSource: view?.hasWatchSource ?? false,
    watchCadence: view?.watchCadence ?? null,
    inputRows: view?.inputRows ?? [],
    startedBy: view?.startedBy ?? "",
    elapsedLabel: view?.elapsedLabel ?? "",
    stepElapsedLabel: view?.stepElapsedLabel ?? null,
    nextNote: view?.nextNote ?? null,
    showNowCard: view?.showNowCard ?? false,
    terminalFromStatus: view?.terminalFromStatus,
    terminalAt: view?.terminalAt,
    handlePause,
    handleResume,
    handleStop,
    handleDecision,
    handleStartNewRun,
    pendingAction,
    isPausePending: pauseMutation.isPending,
    isResumePending: resumeMutation.isPending,
    isStopPending: stopMutation.isPending,
  };
}
