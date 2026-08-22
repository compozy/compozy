import { useState } from "react";
import { useSelector } from "@xstate/store-react";

import { toast } from "@compozy/ui";
import { useStoreBinding } from "@/hooks/use-store-binding";

import { loopRunPageLogic } from "./use-loop-run-page-state";
import {
  isTerminalLoopStatus,
  loopControlAnswer,
  LoopLifecycleConflictError,
  type LoopControlAnswer,
  type LoopGateDecision,
  type LoopNodeLifecycle,
  type LoopNodeSelection,
  type LoopRunGeneration,
  type LoopRunRecord,
  loopPrunedSessionIds,
  loopRunVerbs,
  projectLoopRunPageView,
  projectLoopRunRegisters,
  selectedRosterNode,
  useLoopNodeSessionAvailability,
  loopStreamSeam,
  useApproveLoopRun,
  useCancelLoopRun,
  useKillLoopRun,
  useLoop,
  useLoopRun,
  useLoopRunBriefing,
  useLoopRunRoster,
  useLoopRunTimeline,
  useLoopStream,
  useNowTick,
  usePauseLoopRun,
  useResumeLoopRun,
} from "@/systems/loops";

const SETTLED_OUTPUT_STATUSES = new Set(["succeeded", "failed", "partial", "canceled"]);

function isGenerationBusy(
  run: LoopRunRecord | undefined,
  generations: readonly LoopRunGeneration[] | undefined
): boolean {
  if (!run || isTerminalLoopStatus(run.status)) return false;
  const rounds = generations ?? [];
  const newest = rounds.reduce((max, round) => Math.max(max, round.generation), 0);
  let settled = 0;
  for (const round of rounds) {
    if (round.generation !== newest) continue;
    for (const cell of round.outputs) {
      if (!SETTLED_OUTPUT_STATUSES.has(cell.status)) return true;
      settled += 1;
    }
  }

  return settled === 0;
}

function runControlFeedback(
  failure: unknown,
  runId: string
): { answer?: LoopControlAnswer; error?: string } {
  if (failure instanceof LoopLifecycleConflictError) {
    return {
      answer: loopControlAnswer({
        code: failure.code,
        message: failure.message,
        actualState: failure.actualState,
        allowedTransitions: failure.allowedTransitions,
        winnerActorKind: failure.winnerActorKind,
        winnerActorId: failure.winnerActorId,
        winnerReason: failure.winnerReason,
        winnerRequestedAt: failure.winnerRequestedAt,
        subject: runId,
      }),
    };
  }
  if (failure instanceof Error) return { error: failure.message };
  return {};
}

/**
 * The live run-page view-model (redesign spec §2-§5): composes the run
 * projection (`getLoopRun`) with the pinned definition and the SSE reducer over
 * the forward event contract, then delegates every derived body field to the
 * shared `projectLoopRunPageView` so Storybook fixtures and the live page stay
 * on one derivation. Controls and the gate decision go through the sanctioned
 * mutation hooks.
 */
export function useLoopRunPage(
  workspaceId: string,
  runId: string,
  { liveDataEnabled = true }: { liveDataEnabled?: boolean } = {}
) {
  const bindingKey = `${workspaceId}\u0000${runId}`;
  const { store: runPageStore } = useStoreBinding(bindingKey, () =>
    loopRunPageLogic.createStore({ workspaceId, runId })
  );
  const live = useSelector(runPageStore, state => state.context.live);
  const enabled = workspaceId !== "" && runId !== "";
  const queryEnabled = enabled && liveDataEnabled;
  const runQuery = useLoopRun(workspaceId, runId, queryEnabled);
  const run = runQuery.data?.run;
  const generations = runQuery.data?.generations;
  const executedDefinition = runQuery.data?.executed_definition;
  const materializedContract = runQuery.data?.materialized_contract;
  const loopName = run?.loop_name ?? "";
  const loopQuery = useLoop(
    workspaceId,
    loopName,
    queryEnabled && loopName !== "" && !executedDefinition
  );
  const definition = executedDefinition ?? loopQuery.data?.definition;

  // The three run reads (ADR-005). They are the source for both registers; the
  // stream only tells them when to re-read.
  const briefingRead = useLoopRunBriefing(workspaceId, runId, liveDataEnabled);
  const rosterRead = useLoopRunRoster(workspaceId, runId, liveDataEnabled);
  const timelineRead = useLoopRunTimeline(workspaceId, runId, "notable", liveDataEnabled);

  const isLive = runQuery.isSuccess && !isTerminalLoopStatus(run?.status);
  // Terminal runs keep replaying through their post-status effect results so
  // generation-zero failures and terminal reactions survive navigation or reload.
  // The no-gap seam. The stream may not open until the newest timeline page has
  // published `head_seq`: attaching earlier would start from nothing and drop
  // every event between the read and the subscribe, which is the gap this whole
  // design exists to close. `head_seq = 0` is a real fence for an empty run,
  // so the gate is "known", not "truthy".
  const seam = loopStreamSeam(timelineRead.headSeq);
  useLoopStream(workspaceId, runId, {
    afterSequence: seam.afterSequence,
    enabled: runQuery.isSuccess && liveDataEnabled && seam.ready,
    onEvent: (frame, subscription) =>
      runPageStore.trigger.streamFrameReceived({
        frame,
        subscription,
      }),
  });
  // Opening a node is what makes its session worth asking about, so the page owns
  // the selection: the roster hands over an id, and only the session store knows
  // whether retention has since taken it. One node, one read — never a walk of
  // the roster to answer a question about the row somebody actually opened.
  const [nodeSelection, setNodeSelection] = useState<LoopNodeSelection | null>(null);
  const selectedSessionId =
    selectedRosterNode(rosterRead.nodes, nodeSelection)?.session_id?.trim() || null;
  const sessionAvailability = useLoopNodeSessionAvailability(
    workspaceId,
    selectedSessionId,
    liveDataEnabled
  );

  const pauseMutation = usePauseLoopRun();
  const resumeMutation = useResumeLoopRun();
  const cancelMutation = useCancelLoopRun();
  const killMutation = useKillLoopRun();
  const approveMutation = useApproveLoopRun();

  const nowMs = useNowTick(run?.status === "running" && liveDataEnabled);
  const view = run
    ? projectLoopRunPageView({
        run,
        generations,
        live,
        definition,
        nowMs,
        nodeControls: runQuery.data?.node_controls,
        waits: runQuery.data?.waits,
        requests: runQuery.data?.requests,
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
    } catch (failure) {
      const feedback = runControlFeedback(failure, runId);
      if (feedback.answer) {
        toast.info(feedback.answer.title, { description: feedback.answer.detail });
      } else {
        toast.error(feedback.error ?? "Failed to cancel run");
      }
      return false;
    }
  };

  const handleKill = async (): Promise<boolean> => {
    try {
      await killMutation.mutateAsync({ workspaceId, runId });
      toast.success("Run killed — in-flight work was stopped immediately");
      return true;
    } catch (failure) {
      const feedback = runControlFeedback(failure, runId);
      if (feedback.answer) {
        toast.info(feedback.answer.title, { description: feedback.answer.detail });
      } else {
        toast.error(feedback.error ?? "Failed to kill run");
      }
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

  const version = run?.definition_version ?? loopQuery.data?.version;
  const versionLabel =
    version !== undefined
      ? executedDefinition
        ? `v${version} · pinned`
        : `v${version}`
      : undefined;
  const pendingAction = approveMutation.isPending ? ("approve" as const) : undefined;

  return {
    runQuery,
    // `run` is the polled projection (existence/status/chrome). `effectiveRun`
    // overlays the fresher streamed token count and owns Usage + body reads —
    // the two differ only in `tokens_used`.
    run,
    effectiveRun,
    definition,
    materializedContract,
    generations: generations ?? [],
    watchEvents: runQuery.data?.watch_events ?? null,

    amendments: runQuery.data?.amendments ?? [],

    inputSchema: definition?.inputs,
    isGenerationBusy: isGenerationBusy(run, generations),
    graph: view?.graph ?? null,
    versionLabel,
    live,
    registers: projectLoopRunRegisters({
      briefing: briefingRead.briefing,
      nodes: rosterRead.nodes,
      rollups: rosterRead.rollups,
      timeline: timelineRead.entries,
      // The fork point in the story links the related run, and the run record is
      // the only place that branch is recorded (US-009.EC-3).
      lineage: {
        forkedFrom: effectiveRun?.forked_from ?? null,
        forks: effectiveRun?.forks ?? [],
      },
      graph: view?.graph ?? null,
      rosterIsComplete: rosterRead.isComplete,
      rosterIsTruncated: rosterRead.isTruncated,
    }),
    rosterNodes: rosterRead.nodes,
    rosterRollups: rosterRead.rollups,
    onLoadMoreRoster: rosterRead.loadMore,
    isLoadingMoreRoster: rosterRead.isLoadingMore,
    // Whether the roster read has answered at all. The Inspect lanes need this
    // to tell "this run reached no step" from "we have not read its steps yet";
    // without it a pending or failed read renders as `No steps ran`.
    rosterRead: { isLoading: rosterRead.isLoading, isError: rosterRead.isError },
    nodeSelection,
    onNodeSelectionChange: setNodeSelection,
    prunedSessionIds: loopPrunedSessionIds(selectedSessionId, sessionAvailability),
    storyPaging: {
      hasOlder: timelineRead.hasOlder,
      isLoading: timelineRead.isLoading,
      // Folding this only into `isReconnecting` told the reader the transport is
      // degraded but still let the story print "Nothing has happened in this run
      // yet." The story owns that sentence, so it needs the flag itself.
      isError: timelineRead.isError,
      isLoadingOlder: timelineRead.isLoadingOlder,
      onLoadOlder: timelineRead.loadOlder,
    },
    // A read that errored means the page is showing the last thing it
    // successfully reconciled, and it has to say so. All three durable reads
    // count — a failed timeline read is degraded transport, not evidence that
    // nothing happened — and a settled run says it too: an unreadable terminal
    // run is still unread, however finished it is.
    isReconnecting: briefingRead.isError || rosterRead.isError || timelineRead.isError,
    isLive,
    // The same clock the page's own derivations run on, so the roster's elapsed
    // readings and the Usage rail never disagree by a tick.
    nowMs,
    usageRows: view?.usageRows ?? [],
    usageNote: view?.usageNote ?? null,
    approvalFallbackFacts: view?.approvalFallbackFacts ?? [],
    inputRows: view?.inputRows ?? [],
    startedBy: view?.startedBy ?? "",
    elapsedLabel: view?.elapsedLabel ?? "",
    nodeLifecycles,
    nodesById,
    waitingNodes: view?.waitingNodes ?? [],
    requests: view?.requests ?? [],
    handlePause,
    handleResume,
    handleCancel,
    handleKill,
    handleDecision,
    pendingAction,
    isPausePending: pauseMutation.isPending,
    isResumePending: resumeMutation.isPending,
    isCancelPending: cancelMutation.isPending,
    isKillPending: killMutation.isPending,
    cancelAnswer: runControlFeedback(cancelMutation.error, runId).answer,
    killAnswer: runControlFeedback(killMutation.error, runId).answer,
    cancelError: runControlFeedback(cancelMutation.error, runId).error,
    killError: runControlFeedback(killMutation.error, runId).error,
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
