import { useState } from "react";

import { toast } from "@compozy/ui";

import {
  LoopLifecycleConflictError,
  type LoopNodeLifecycle,
  type LoopNodeVerb,
  type LoopNodeVerbCommit,
  type LoopNodeVerbRequest,
  loopControlAnswer,
  loopNodeWaitResumeItemIndex,
  useCancelLoopNode,
  useKillLoopNode,
  usePauseLoopNode,
  useRequeueLoopNode,
  useResumeLoopNode,
} from "@/systems/loops";

/**
 * Node verb orchestration for the run page (VC-R3). It owns the confirm/commit
 * cycle for every node lifecycle verb plus the quarantine sheet's open state.
 *
 * The daemon's deterministic answers (409/422) never surface as failures: they
 * arrive as `LoopLifecycleConflictError` carrying the state that won, and this
 * hook renders them as an informational toast and keeps the dialog open with the
 * daemon's own text, so the operator sees why nothing changed.
 */
export function useLoopNodeControls(workspaceId: string, runId: string) {
  const [request, setRequest] = useState<LoopNodeVerbRequest | null>(null);
  const [answer, setAnswer] = useState<string | undefined>(undefined);
  const [quarantineNodeId, setQuarantineNodeId] = useState<string | null>(null);

  const pauseNode = usePauseLoopNode();
  const resumeNode = useResumeLoopNode();
  const cancelNode = useCancelLoopNode();
  const killNode = useKillLoopNode();
  const requeueNode = useRequeueLoopNode();

  const isPending =
    pauseNode.isPending ||
    resumeNode.isPending ||
    cancelNode.isPending ||
    killNode.isPending ||
    requeueNode.isPending;

  const closeDialog = () => {
    setRequest(null);
    setAnswer(undefined);
  };

  const handleError = (error: unknown, node: LoopNodeLifecycle) => {
    if (error instanceof LoopLifecycleConflictError) {
      const conflict: LoopLifecycleConflictError = error;
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
      setAnswer(resolved.detail);
      return;
    }
    toast.error(error instanceof Error ? error.message : "The request did not go through");
    setAnswer(error instanceof Error ? error.message : undefined);
  };

  const onVerb = (verb: LoopNodeVerb, node: LoopNodeLifecycle) => {
    setAnswer(undefined);
    if (verb === "open-quarantine") {
      setQuarantineNodeId(node.nodeId);
      return;
    }
    setRequest({ verb, node });
  };

  const commit = ({ verb, node, mode, payload }: LoopNodeVerbCommit) => {
    const target = { workspaceId, runId, nodeId: node.nodeId };
    const onSuccess = (message: string) => () => {
      toast.success(message);
      closeDialog();
    };
    const onError = (error: unknown) => handleError(error, node);
    switch (verb) {
      case "pause":
        pauseNode.mutate(
          { ...target, data: { mode: mode ?? "drain" } },
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
          setAnswer(
            `That payload isn't valid JSON: ${
              parseError instanceof Error ? parseError.message : "unparseable"
            }`
          );
          return;
        }
        const itemIndex = loopNodeWaitResumeItemIndex(node);
        if (itemIndex === undefined) {
          setAnswer("This wait is no longer open.");
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
          { ...target, data: {} },
          { onSuccess: onSuccess(`${node.nodeId} canceled`), onError }
        );
        return;
      case "kill":
        killNode.mutate(
          { ...target, data: {} },
          { onSuccess: onSuccess(`${node.nodeId} killed`), onError }
        );
        return;
      case "requeue":
        requeueNode.mutate(
          { ...target, data: {} },
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

  return {
    request,
    answer,
    isPending,
    quarantineNodeId,
    onVerb,
    commit,
    closeDialog,
    openQuarantine: (node: LoopNodeLifecycle) => setQuarantineNodeId(node.nodeId),
    closeQuarantine: () => setQuarantineNodeId(null),
  };
}
