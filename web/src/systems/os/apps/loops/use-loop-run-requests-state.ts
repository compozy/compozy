import { useState } from "react";

import { toast } from "@compozy/ui";

import {
  type LoopRequestAnswerInput,
  LoopRequestError,
  useLoopRequestDetail,
  useRespondLoopRequest,
} from "@/systems/loops";

export interface LoopRequestTarget {
  nodeId: string;
  itemIndex: number;
}

export function loopRequestTargetKey(target: LoopRequestTarget): string {
  return `${target.nodeId}:${target.itemIndex}`;
}

function sameTarget(left: LoopRequestTarget | null, right: LoopRequestTarget): boolean {
  return left !== null && left.nodeId === right.nodeId && left.itemIndex === right.itemIndex;
}

export function useLoopRunRequestsState(workspaceId: string, runId: string) {
  const [engaged, setEngaged] = useState<LoopRequestTarget | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Readonly<Record<string, string>> | undefined>(
    undefined
  );
  const [refusal, setRefusal] = useState<string | undefined>(undefined);
  const [wantsFullContext, setWantsFullContext] = useState(false);
  const respondMutation = useRespondLoopRequest();

  const contextEnabled = wantsFullContext && engaged !== null;
  const contextQuery = useLoopRequestDetail(
    workspaceId,
    runId,
    contextEnabled ? engaged.nodeId : "",
    engaged?.itemIndex,
    contextEnabled
  );

  const clearFeedback = () => {
    setFieldErrors(undefined);
    setRefusal(undefined);
  };

  const engage = (target: LoopRequestTarget) => {
    if (sameTarget(engaged, target)) return;
    setEngaged(target);
    setWantsFullContext(false);
    clearFeedback();
  };

  const handleRequestFullContext = (nodeId: string, itemIndex: number) => {
    engage({ nodeId, itemIndex });
    setWantsFullContext(true);
  };

  const handleAnswerRequest = async (input: LoopRequestAnswerInput) => {
    engage({ nodeId: input.nodeId, itemIndex: input.itemIndex });
    clearFeedback();
    try {
      await respondMutation.mutateAsync({
        workspaceId,
        runId,
        nodeId: input.nodeId,
        data: {
          decision: input.decision,
          item_index: input.itemIndex,
          note: input.note,
          payload: input.payload,
        },
      });
      toast.success("Answer recorded");
      setEngaged(null);
      setWantsFullContext(false);
      clearFeedback();
    } catch (failure) {
      if (failure instanceof LoopRequestError) {
        const daemonFields = failure.fieldErrors;

        if (failure.isAnswerable && Object.keys(daemonFields).length > 0) {
          setFieldErrors(daemonFields);
          setRefusal(undefined);
          return;
        }
        setFieldErrors(undefined);
        setRefusal(failure.message);
        toast.info(failure.message);
        return;
      }
      clearFeedback();
      toast.error(failure instanceof Error ? failure.message : "The answer did not go through");
    }
  };

  return {
    engagedKey: engaged === null ? undefined : loopRequestTargetKey(engaged),

    isAnswerPending: respondMutation.isPending,
    fieldErrors,
    refusal,
    fullContext: contextEnabled ? contextQuery.data?.context : undefined,
    isLoadingFullContext: contextEnabled && contextQuery.isPending,
    onRequestFullContext: handleRequestFullContext,
    onAnswer: handleAnswerRequest,
  };
}
