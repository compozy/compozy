import { useImperativeHandle, useState, type Ref } from "react";

import { useWorktreeMaterialization } from "@/systems/workspace";

import type { SessionEnvironmentControlHandle } from "../components/session-environment-control";
import { useChangeSessionWorktree } from "./use-change-session-worktree";
import { useForkSessionToWorktree } from "./use-fork-session-to-worktree";

interface UseSessionEnvironmentControlParams {
  ref?: Ref<SessionEnvironmentControlHandle>;
  sessionId: string;
  workspaceId: string;
  currentWorktreeId: string;
}

/** Coordinates environment changes and the existing independent fork flow. */
export function useSessionEnvironmentControl({
  ref,
  sessionId,
  workspaceId,
  currentWorktreeId,
}: UseSessionEnvironmentControlParams) {
  const [mode, setMode] = useState<"fork" | "bind" | null>(null);
  const change = useChangeSessionWorktree(workspaceId, sessionId);
  const [expectedWorktreeId, setExpectedWorktreeId] = useState(currentWorktreeId);
  const [target, setTarget] = useState("");
  const fork = useForkSessionToWorktree(workspaceId, sessionId);
  const materialization = useWorktreeMaterialization(workspaceId);
  const resolvedTarget = materialization.worktree?.id ?? target;

  useImperativeHandle(ref, () => ({ openFork: () => setMode("fork") }), []);

  function setOpen(open: boolean) {
    setMode(current => (open ? (current ?? "fork") : null));
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
    forkOpen: mode === "fork",
    bindingOpen: mode === "bind",
    bindingPending: change.isPending,
    openBinding: () => {
      change.reset();
      setExpectedWorktreeId(currentWorktreeId);
      setMode("bind");
    },
    confirmBinding: () =>
      change.mutate(
        { worktree: resolvedTarget, expectedWorktreeId },
        { onSuccess: () => setOpen(false) }
      ),
    useWorkspaceRoot: () => change.mutate({ worktree: "", expectedWorktreeId: currentWorktreeId }),
    isPending: fork.isPending,
    materialization,
    startNewWorktree: () => {
      setTarget("");
      materialization.start("");
    },
    open: () => setMode("fork"),
    setOpen,
    setTarget: (nextTarget: string) => {
      materialization.reset();
      setTarget(nextTarget);
    },
    target: resolvedTarget,
  };
}
