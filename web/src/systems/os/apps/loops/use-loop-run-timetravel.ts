import { useState } from "react";

import { useNavigate } from "@tanstack/react-router";

import { toast } from "@compozy/ui";

import {
  type LoopInputSchema,
  LoopInputValidationError,
  type LoopRunGeneration,
  LoopTimetravelError,
  useForkLoopRun,
} from "@/systems/loops";

export interface LoopRunTimetravelInput {
  workspaceId: string;
  runId: string;
  loopName: string;
  generations: readonly LoopRunGeneration[];

  inputSchema?: LoopInputSchema;

  sourceInputs: Readonly<Record<string, unknown>>;
}

function forkableGenerations(generations: readonly LoopRunGeneration[]): number[] {
  return [...new Set(generations.map(generation => generation.generation))].sort(
    (left, right) => right - left
  );
}

export function useLoopRunTimetravel({
  workspaceId,
  runId,
  loopName,
  generations,
  inputSchema,
  sourceInputs,
}: LoopRunTimetravelInput) {
  const navigate = useNavigate();
  const [forkGeneration, setForkGeneration] = useState<number | null>(null);
  const [forkFieldErrors, setForkFieldErrors] = useState<Readonly<Record<string, string>>>();
  const [forkBlockedReason, setForkBlockedReason] = useState<string>();
  const forkMutation = useForkLoopRun();

  const openRun = (targetRunId: string) => {
    void navigate({
      to: "/loop-runs/$runId",
      params: { runId: targetRunId },
      search: { workspace: workspaceId },
    });
  };

  const compareGeneration = (generation: number) => {
    void navigate({
      to: "/loop-runs/$runId/diff",
      params: { runId },
      search: { generation, workspace: workspaceId },
    });
  };

  const openFork = (generation: number) => {
    setForkFieldErrors(undefined);
    setForkBlockedReason(undefined);
    setForkGeneration(generation);
  };

  const closeFork = () => {
    setForkGeneration(null);
    setForkFieldErrors(undefined);
    setForkBlockedReason(undefined);
  };

  const submitFork = async (input: {
    generation: number;
    inputs: Record<string, unknown>;
    reason: string;
  }) => {
    setForkFieldErrors(undefined);
    setForkBlockedReason(undefined);
    const reason = input.reason.trim();
    try {
      const result = await forkMutation.mutateAsync({
        workspaceId,
        runId,
        data: {
          generation: input.generation,
          inputs: input.inputs,
          reason: reason === "" ? undefined : reason,
        },
      });
      toast.success("Fork started");
      setForkGeneration(null);
      openRun(result.run.id);
    } catch (failure) {
      if (failure instanceof LoopInputValidationError) {
        setForkFieldErrors(failure.fieldErrors);
        return;
      }
      if (failure instanceof LoopTimetravelError) {
        if (failure.status === 422 && Object.keys(failure.details).length > 0) {
          setForkFieldErrors(failure.details);
          return;
        }
        setForkBlockedReason(failure.message);
        toast.info(failure.message);
        return;
      }
      toast.error(failure instanceof Error ? failure.message : "The fork did not go through");
    }
  };

  return {
    loopName,
    forkGeneration,
    forkGenerations: forkableGenerations(generations),
    forkInputSchema: inputSchema,
    forkSourceInputs: sourceInputs,
    forkFieldErrors,
    forkBlockedReason,
    isForkPending: forkMutation.isPending,
    onOpenRun: openRun,
    onCompareGeneration: compareGeneration,
    onForkGeneration: openFork,
    onCloseFork: closeFork,
    onSubmitFork: submitFork,
  };
}
