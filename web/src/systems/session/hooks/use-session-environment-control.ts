import { useEffect, useImperativeHandle, useState, type Ref } from "react";

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

  useEffect(() => {
    if (materialization.status === "ready" && materialization.worktree) {
      setTarget(materialization.worktree.id);
    }
  }, [materialization.status, materialization.worktree]);

  useImperativeHandle(ref, () => ({ openFork: () => setForkOpen(true) }), []);

  function setOpen(open: boolean) {
    setForkOpen(open);
    if (open) return;
    setTarget("");
    materialization.reset();
  }

  return {
    confirm: async () => {
      await fork.mutateAsync({ confirmed: true, worktree: target });
      setOpen(false);
    },
    forkOpen,
    isPending: fork.isPending,
    materialization,
    open: () => setForkOpen(true),
    setOpen,
    setTarget,
    target,
  };
}
