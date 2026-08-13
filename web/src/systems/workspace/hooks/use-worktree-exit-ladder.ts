import { useState } from "react";

import { toWorktreeExitLadder, type WorktreeExitLadder } from "../lib/worktree-exit-ladder";
import { reduceWorktreeExitEvent, type WorktreeExitProgress } from "../lib/worktree-exit-progress";
import type { WorktreeExitEventName, WorktreeExitEventPayload } from "../lib/worktree-events";
import type { WorktreeExitPlanPayload } from "../types";
import { useWorktreeExitPlan } from "./use-worktree-exit";
import { useWorktreeStream, type WorktreeEventSourceFactory } from "./use-worktree-stream";
import type { WorktreeStreamStatus } from "./worktree-stream-store";

export interface UseWorktreeExitLadderOptions {
  enabled?: boolean;
  eventSourceFactory?: WorktreeEventSourceFactory;
}

export interface WorktreeExitLadderModel {
  /** The payload as returned, for callers that need fields the ladder does not project. */
  plan?: WorktreeExitPlanPayload;
  /** Undefined until the plan loads; the control renders nothing rather than guessing. */
  ladder?: WorktreeExitLadder;
  isLoading: boolean;
  error: Error | null;
  /** Live operation, folded from the stream. Terminal states stay until dismissed. */
  progress?: WorktreeExitProgress;
  streamStatus: WorktreeStreamStatus;
  dismissProgress: () => void;
}

/**
 * Renders the daemon's exit ladder and follows the operation it starts.
 *
 * Every decision on screen — which action is primary, which rows are blocked and
 * why — comes from `GET .../exit`. This hook adds no logic of its own beyond
 * folding stream frames into a progress model.
 */
export function useWorktreeExitLadder(
  workspaceID: string,
  worktreeID: string,
  { enabled = true, eventSourceFactory }: UseWorktreeExitLadderOptions = {}
): WorktreeExitLadderModel {
  const plan = useWorktreeExitPlan(workspaceID, worktreeID, { enabled });
  const [progress, setProgress] = useState<WorktreeExitProgress | undefined>(undefined);

  function handleExitEvent(name: WorktreeExitEventName, payload: WorktreeExitEventPayload) {
    setProgress(current => reduceWorktreeExitEvent(current, name, payload));
  }

  const streamStatus = useWorktreeStream(workspaceID, worktreeID, {
    enabled,
    onExitEvent: handleExitEvent,
    eventSourceFactory,
  });

  const ladder = plan.data ? toWorktreeExitLadder(plan.data) : undefined;

  return {
    plan: plan.data,
    ladder,
    isLoading: plan.isPending,
    error: plan.error,
    progress,
    streamStatus,
    dismissProgress: () => setProgress(undefined),
  };
}
