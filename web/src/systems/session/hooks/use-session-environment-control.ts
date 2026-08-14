import { useImperativeHandle, useState, type Ref } from "react";

import { useWorktreeMaterialization } from "@/systems/workspace";

import type { SessionEnvironmentControlHandle } from "../components/session-environment-control";
import { useForkSessionToWorktree } from "./use-fork-session-to-worktree";

interface UseSessionEnvironmentControlParams {
  ref?: Ref<SessionEnvironmentControlHandle>;
  sessionId: string;
  workspaceId: string;
}

/** Coordinates the immutable-session fork flow behind the environment chip. */
export function useSessionEnvironmentControl({
  ref,
  sessionId,
  workspaceId,
}: UseSessionEnvironmentControlParams) {
  const [forkOpen, setForkOpen] = useState(false);
  const [target, setTarget] = useState("");
  const fork = useForkSessionToWorktree(workspaceId, sessionId);
  const materialization = useWorktreeMaterialization(workspaceId);
  const resolvedTarget = materialization.worktree?.id ?? target;

  useImperativeHandle(ref, () => ({ openFork: () => setForkOpen(true) }), []);

  function setOpen(open: boolean) {
    setForkOpen(open);
    if (open) return;
    setTarget("");
    materialization.cancel();
    materialization.reset();
  }

  return {
    confirm: async () => {
      await fork.mutateAsync({ confirmed: true, worktree: resolvedTarget });
      setOpen(false);
    },
    forkOpen,
    isPending: fork.isPending,
    materialization,
    startNewWorktree: () => {
      setTarget("");
      materialization.start("");
    },
    open: () => setForkOpen(true),
    setOpen,
    setTarget: (nextTarget: string) => {
      materialization.reset();
      setTarget(nextTarget);
    },
    target: resolvedTarget,
  };
}
