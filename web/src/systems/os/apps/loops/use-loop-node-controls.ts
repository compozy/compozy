import { useState } from "react";

import { toast } from "@compozy/ui";

import {
  buildRerunSet,
  loopControlAnswer,
  LoopLifecycleConflictError,
  type LoopAmendment,
  type LoopControlAnswer,
  type LoopDefinition,
  type LoopGraph,
  type LoopNodeLifecycle,
  type LoopNodeTimetravelCapability,
  type LoopNodeVerb,
  type LoopNodeVerbCommit,
  type LoopNodeVerbRequest,
  loopNodeVerbs,
  loopNodeWaitResumeItemIndex,
  LoopRequestError,
  LoopTimetravelError,
  useAmendLoopNode,
  useCancelLoopNode,
  useKillLoopNode,
  usePauseLoopNode,
  useRequeueLoopNode,
  useRerunLoopRun,
  useResumeLoopNode,
} from "@/systems/loops";

export interface LoopNodeControlsContext {
  definition?: LoopDefinition;

  graph: LoopGraph | null;
  runStatus?: string;

  isGenerationBusy: boolean;

  amendments?: readonly LoopAmendment[];
}

type LoopNodeTimetravelVerb = "amend" | "rerun";

interface LoopNodeTimetravelRequest {
  verb: LoopNodeTimetravelVerb;
  node: LoopNodeLifecycle;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  return typeof value === "object" && value !== null ? (value as Record<string, unknown>) : null;
}

function nodeOutputSchema(definition: LoopDefinition | undefined, nodeId: string): unknown {
  const node = definition?.graph.nodes.find(entry => entry.id === nodeId);
  if (!node) return undefined;
  return asRecord(node.params)?.output_schema ?? node.output_schema;
}

function amendedOutput(
  amendments: readonly LoopAmendment[] | undefined,
  node: LoopNodeLifecycle
): unknown {
  let newest: LoopAmendment | undefined;
  for (const amendment of amendments ?? []) {
    if (amendment.node_id !== node.nodeId || amendment.generation !== node.generation) continue;
    if (node.itemIndex !== null && amendment.item_index !== node.itemIndex) continue;
    if (!newest || amendment.amendment_seq > newest.amendment_seq) newest = amendment;
  }
  if (!newest) return undefined;
  return newest.amended ?? newest.original;
}

function timetravelAnswer(failure: unknown, subject: string): LoopControlAnswer | null {
  if (!(failure instanceof LoopTimetravelError) && !(failure instanceof LoopRequestError)) {
    return null;
  }
  const details = failure.details;
  return loopControlAnswer({
    code: failure.code,
    message: failure.message,
    actualState: details.actual_state ?? "",
    allowedTransitions: (details.allowed_transitions ?? "")
      .split(",")
      .map(verb => verb.trim())
      .filter(verb => verb !== ""),
    winnerActorKind: details.winner_actor_kind ?? "",
    winnerActorId: details.winner_actor_id ?? "",
    winnerReason: details.winner_reason ?? "",
    winnerRequestedAt: details.winner_requested_at ?? "",
    subject,
  });
}

export function useLoopNodeControls(
  workspaceId: string,
  runId: string,
  context: LoopNodeControlsContext
) {
  const [request, setRequest] = useState<LoopNodeVerbRequest | null>(null);
  const [answer, setAnswer] = useState<LoopControlAnswer | undefined>(undefined);
  const [error, setError] = useState<string | undefined>(undefined);
  const [quarantineNodeId, setQuarantineNodeId] = useState<string | null>(null);
  const [timetravel, setTimetravel] = useState<LoopNodeTimetravelRequest | null>(null);
  const [timetravelRefusal, setTimetravelRefusal] = useState<LoopControlAnswer | null>(null);
  const [amendFieldErrors, setAmendFieldErrors] = useState<Readonly<Record<string, string>>>();

  const pauseNode = usePauseLoopNode();
  const resumeNode = useResumeLoopNode();
  const cancelNode = useCancelLoopNode();
  const killNode = useKillLoopNode();
  const requeueNode = useRequeueLoopNode();
  const amendMutation = useAmendLoopNode();
  const rerunMutation = useRerunLoopRun();

  const isPending =
    pauseNode.isPending ||
    resumeNode.isPending ||
    cancelNode.isPending ||
    killNode.isPending ||
    requeueNode.isPending;

  const isTimetravelPending = amendMutation.isPending || rerunMutation.isPending;

  const closeDialog = () => {
    setRequest(null);
    setAnswer(undefined);
    setError(undefined);
  };

  const closeTimetravel = () => {
    setTimetravel(null);
    setTimetravelRefusal(null);
    setAmendFieldErrors(undefined);
  };

  const timetravelFor = (node: LoopNodeLifecycle): LoopNodeTimetravelCapability => ({
    hasOutputShape: nodeOutputSchema(context.definition, node.nodeId) !== undefined,
    isGenerationBusy: context.isGenerationBusy,
  });

  const nodeVerbs = (node: LoopNodeLifecycle): LoopNodeVerb[] =>
    loopNodeVerbs(node, context.runStatus, timetravelFor(node));

  const handleError = (failure: unknown, node: LoopNodeLifecycle) => {
    if (failure instanceof LoopLifecycleConflictError) {
      const conflict: LoopLifecycleConflictError = failure;
      const resolved = loopControlAnswer({
        code: conflict.code,
        message: conflict.message,
        actualState: conflict.actualState,
        allowedTransitions: conflict.allowedTransitions,
        winnerActorKind: conflict.winnerActorKind,
        winnerActorId: conflict.winnerActorId,
        winnerReason: conflict.winnerReason,
        winnerRequestedAt: conflict.winnerRequestedAt,
        subject: node.nodeId,
      });
      // Information, not an error: the daemon answered, the state simply moved.
      toast.info(resolved.title, { description: resolved.detail });
      setAnswer(resolved);
      setError(undefined);
      return;
    }
    const message = failure instanceof Error ? failure.message : "The request did not go through";
    toast.error(message);
    setAnswer(undefined);
    setError(message);
  };

  const onVerb = (verb: LoopNodeVerb, node: LoopNodeLifecycle) => {
    setAnswer(undefined);
    setError(undefined);
    if (verb === "open-quarantine") {
      setQuarantineNodeId(node.nodeId);
      return;
    }
    if (verb === "amend" || verb === "rerun") {
      if (!nodeVerbs(node).includes(verb)) return;
      closeTimetravel();
      setTimetravel({ verb, node });
      return;
    }
    setRequest({ verb, node });
  };

  const handleTimetravelFailure = (failure: unknown, node: LoopNodeLifecycle) => {
    const resolved = timetravelAnswer(failure, node.nodeId);
    if (resolved) {
      toast.info(resolved.title, { description: resolved.detail });
      setTimetravelRefusal(resolved);
      return;
    }
    const message = failure instanceof Error ? failure.message : "The request did not go through";
    toast.error(message);
    setTimetravelRefusal(null);
  };

  const commitAmend = async ({
    payload,
    reason,
  }: {
    payload: Record<string, unknown>;
    reason: string;
  }) => {
    const node = timetravel?.node;
    if (!node) return;
    const trimmedReason = reason.trim();
    setTimetravelRefusal(null);
    setAmendFieldErrors(undefined);
    try {
      await amendMutation.mutateAsync({
        workspaceId,
        runId,
        nodeId: node.nodeId,
        data: {
          generation: node.generation,
          item_index: node.itemIndex ?? undefined,
          payload,
          reason: trimmedReason === "" ? undefined : trimmedReason,
        },
      });
      toast.success(`${node.nodeId} output amended`);
      closeTimetravel();
    } catch (failure) {
      if (failure instanceof LoopRequestError && Object.keys(failure.fieldErrors).length > 0) {
        setAmendFieldErrors(failure.fieldErrors);
        setTimetravelRefusal(null);
        return;
      }
      handleTimetravelFailure(failure, node);
    }
  };

  const commitRerun = async ({ reason }: { reason: string }) => {
    const node = timetravel?.node;
    if (!node) return;
    const trimmedReason = reason.trim();
    setTimetravelRefusal(null);
    try {
      const result = await rerunMutation.mutateAsync({
        workspaceId,
        runId,
        data: {
          from_node: node.nodeId,
          reason: trimmedReason === "" ? undefined : trimmedReason,
        },
      });
      toast.success(`Generation ${result.generation} opened from ${node.nodeId}`);
      closeTimetravel();
    } catch (failure) {
      handleTimetravelFailure(failure, node);
    }
  };

  const commit = ({ verb, node, mode, payload, reason }: LoopNodeVerbCommit) => {
    const target = { workspaceId, runId, nodeId: node.nodeId };
    const trimmedReason = reason?.trim() ?? "";
    const reasonBody = trimmedReason === "" ? {} : { reason: trimmedReason };
    const onSuccess = (message: string) => () => {
      toast.success(message);
      closeDialog();
    };
    const onError = (failure: unknown) => handleError(failure, node);
    switch (verb) {
      case "pause":
        pauseNode.mutate(
          { ...target, data: { mode: mode ?? "drain", ...reasonBody } },
          { onSuccess: onSuccess(`${node.nodeId} paused`), onError }
        );
        return;
      case "resume":
      case "resume-reset-attempts":
      case "resume-immediate":
        resumeNode.mutate(
          { ...target, data: { mode: mode ?? "plain" } },
          { onSuccess: onSuccess(`${node.nodeId} resumed`), onError }
        );
        return;
      case "resume-wait": {
        // A by-hand wait resume stands in for the event, so the payload must be
        // real JSON; the daemon validates it against the wait's expectation.
        let parsed: unknown;
        try {
          parsed = payload && payload.trim() !== "" ? JSON.parse(payload) : {};
        } catch (parseError) {
          setAnswer(undefined);
          setError(
            `That payload isn't valid JSON: ${
              parseError instanceof Error ? parseError.message : "unparseable"
            }`
          );
          return;
        }
        const itemIndex = loopNodeWaitResumeItemIndex(node);
        if (itemIndex === undefined) {
          setAnswer(undefined);
          setError("This wait is no longer open.");
          return;
        }
        resumeNode.mutate(
          {
            ...target,
            data: { mode: "plain", item_index: itemIndex, payload: parsed },
          },
          { onSuccess: onSuccess(`${node.nodeId} resumed`), onError }
        );
        return;
      }
      case "cancel":
        cancelNode.mutate(
          { ...target, data: reasonBody },
          { onSuccess: onSuccess(`${node.nodeId} canceled`), onError }
        );
        return;
      case "kill":
        killNode.mutate(
          { ...target, data: reasonBody },
          { onSuccess: onSuccess(`${node.nodeId} killed`), onError }
        );
        return;
      case "requeue":
        requeueNode.mutate(
          { ...target, data: reasonBody },
          {
            onSuccess: () => {
              toast.success(`${node.nodeId} requeued`);
              closeDialog();
              setQuarantineNodeId(null);
            },
            onError,
          }
        );
        return;
      default:
        return;
    }
  };

  const amendNode = timetravel?.verb === "amend" ? timetravel.node : null;
  const rerunNode = timetravel?.verb === "rerun" ? timetravel.node : null;
  return {
    request,
    answer,
    error,
    isPending,

    isBusy: isPending || isTimetravelPending,
    quarantineNodeId,
    onVerb,
    commit,
    closeDialog,
    openQuarantine: (nodeId: string) => setQuarantineNodeId(nodeId),
    closeQuarantine: () => setQuarantineNodeId(null),

    timetravelFor,
    amendNode,
    amendOriginalOutput: amendNode ? amendedOutput(context.amendments, amendNode) : undefined,
    amendOutputSchema: amendNode
      ? nodeOutputSchema(context.definition, amendNode.nodeId)
      : undefined,
    amendFieldErrors,
    rerunNode,
    rerunSet: buildRerunSet(context.graph, rerunNode?.nodeId ?? ""),
    timetravelAnswer: timetravelRefusal,
    isTimetravelPending,
    commitAmend,
    commitRerun,
    closeTimetravel,
  };
}
