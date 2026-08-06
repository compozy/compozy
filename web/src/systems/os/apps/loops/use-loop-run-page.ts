import { useNavigate } from "@tanstack/react-router";
import { useSelector } from "@xstate/store-react";

import { toast } from "@compozy/ui";
import { useStoreBinding } from "@/hooks/use-store-binding";

import {
  buildNodeNowLines,
  isTerminalLoopStatus,
  type LoopGateDecision,
  type LoopNodeLifecycle,
  loopRunVerbs,
  mergeGoalTurnTimeline,
  projectLoopRunPageView,
  useApproveLoopRun,
  useCancelLoopRun,
  useGoalTurns,
  useKillLoopRun,
  useLoop,
  useLoopRun,
  useLoopStream,
  useNowTick,
  usePauseLoopRun,
  useResumeLoopRun,
  useRunLoop,
} from "@/systems/loops";
import { loopRunPageLogic } from "./use-loop-run-page-state";

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
  const bindingKey = `${workspaceId}\u0000${runId}`;
  const { store: runPageStore } = useStoreBinding(bindingKey, () =>
    loopRunPageLogic.createStore({ workspaceId, runId })
  );
  const live = useSelector(runPageStore, state => state.context.live);
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

  const isLive = runQuery.isSuccess && !isTerminalLoopStatus(run?.status);
  // Terminal runs replay their retained status event once so generation-zero
  // failures remain visible after navigation or reload; the hook closes on it.
  useLoopStream(workspaceId, runId, {
    enabled: runQuery.isSuccess,
    onEvent: (frame, subscription) =>
      runPageStore.trigger.streamFrameReceived({
        frame,
        subscription,
      }),
  });
  const goalTurnsQuery = useGoalTurns(workspaceId, runId, { enabled: runQuery.isSuccess });
  const goalTurns = mergeGoalTurnTimeline(goalTurnsQuery.data?.turns ?? [], live.goalTurns);

  const pauseMutation = usePauseLoopRun();
  const resumeMutation = useResumeLoopRun();
  const cancelMutation = useCancelLoopRun();
  const killMutation = useKillLoopRun();
  const approveMutation = useApproveLoopRun();
  const runLoopMutation = useRunLoop();

  const nowMs = useNowTick(run?.status === "running");
  const view = run
    ? projectLoopRunPageView({
        run,
        generations,
        live,
        definition,
        nowMs,
        nodeControls: runQuery.data?.node_controls,
        waits: runQuery.data?.waits,
      })
    : null;
  const effectiveRun = view?.effectiveRun ?? run;
  const nodeLifecycles = view?.nodeLifecycles ?? [];
  const nodesById = new Map<string, LoopNodeLifecycle>(
    nodeLifecycles.map(node => [node.nodeId, node])
  );

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

  const handleCancel = async (): Promise<boolean> => {
    try {
      await cancelMutation.mutateAsync({ workspaceId, runId });
      toast.success("Cancellation requested");
      return true;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to cancel run");
      return false;
    }
  };

  const handleKill = async (): Promise<boolean> => {
    try {
      await killMutation.mutateAsync({ workspaceId, runId });
      toast.success("Run killed — in-flight work was stopped immediately");
      return true;
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to kill run");
      return false;
    }
  };

  const resetRunControlErrors = () => {
    cancelMutation.reset();
    killMutation.reset();
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
    generations: generations ?? [],
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
    terminalCause: view?.terminalCause,
    nodeLifecycles,
    nodesById,
    nodeNowLines: buildNodeNowLines(nodeLifecycles, view?.graph ?? null, live.retrySchedules),
    waitingNodes: view?.waitingNodes ?? [],
    attentionNodes: view?.attentionNodes ?? [],
    nodeSessions: view?.nodeSessions ?? new Map<string, string>(),
    handlePause,
    handleResume,
    handleCancel,
    handleKill,
    handleDecision,
    handleStartNewRun,
    pendingAction,
    isPausePending: pauseMutation.isPending,
    isResumePending: resumeMutation.isPending,
    isCancelPending: cancelMutation.isPending,
    isKillPending: killMutation.isPending,
    cancelError: cancelMutation.error instanceof Error ? cancelMutation.error.message : undefined,
    killError: killMutation.error instanceof Error ? killMutation.error.message : undefined,
    resetRunControlErrors,
    /** Kill is offerable exactly while the daemon reports a live run. */
    canKillRun: Boolean(run) && loopRunVerbs(run?.status, false).includes("kill"),
    // The daemon settles one run verb before the next is offerable, so the
    // controls read a single in-flight verb rather than four parallel flags.
    pendingRunVerb: pauseMutation.isPending
      ? ("pause" as const)
      : resumeMutation.isPending
        ? ("resume" as const)
        : cancelMutation.isPending
          ? ("cancel" as const)
          : killMutation.isPending
            ? ("kill" as const)
            : undefined,
  };
}
