import { useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { type GoalControlAction, loopsKeys, useLoopStream } from "@/systems/loops";
import { sessionKeys } from "../lib/query-keys";
import type { SessionGoalCommandResult, SessionGoalResponse } from "../types";
import { useSendSessionPrompt } from "./use-session-actions";
import { useSessionStore } from "./use-session-store";
import { useSessionGoal } from "./use-sessions";

function isGoalResult(value: unknown): value is SessionGoalCommandResult {
  return typeof value === "object" && value !== null && "outcome" in value;
}

function replacementObjective(command: string | undefined): string | null {
  const value = command?.trim();
  if (!value?.startsWith("/goal ")) return null;
  const rest = value.slice("/goal ".length).trim();
  if (!rest.startsWith("replace ")) return rest || null;
  const replacement = rest.slice("replace ".length).trim();
  const separator = replacement.indexOf(" ");
  if (separator < 0) return null;
  return replacement.slice(separator + 1).trim() || null;
}

export function useSessionGoalHeader(
  workspaceId: string,
  sessionId: string,
  onPrefillComposer?: (text: string) => void
) {
  const queryClient = useQueryClient();
  const query = useSessionGoal(workspaceId, sessionId);
  const result = useSessionStore(state => state.goalResults[sessionId]);
  const resultCommand = useSessionStore(state => state.goalResultCommands[sessionId]);
  const setGoalResult = useSessionStore(state => state.setGoalResult);
  const mutation = useSendSessionPrompt({ workspaceId });
  const [pendingAction, setPendingAction] = useState<GoalControlAction>();
  const snapshot = query.data?.goal ?? null;

  useLoopStream(workspaceId, snapshot?.run_id ?? "", {
    enabled: snapshot?.live === true,
    onEvent: frame => {
      if (frame.kind === "goal_turn_started") {
        const payload = frame.payload as Record<string, unknown> | undefined;
        const turn = typeof payload?.turn === "number" ? payload.turn : null;
        if (turn !== null) {
          queryClient.setQueryData<SessionGoalResponse>(
            sessionKeys.goal(workspaceId, sessionId),
            current =>
              current?.goal
                ? { goal: { ...current.goal, turns_used: Math.max(current.goal.turns_used, turn) } }
                : current
          );
        }
      }
      if (frame.kind === "goal_status_changed") {
        void queryClient.invalidateQueries({ queryKey: sessionKeys.goal(workspaceId, sessionId) });
      }
    },
  });

  const applyResult = (next: SessionGoalCommandResult) => {
    setGoalResult(sessionId, next);
    if (next.snapshot !== null || next.outcome === "cleared") {
      queryClient.setQueryData<SessionGoalResponse>(sessionKeys.goal(workspaceId, sessionId), {
        goal: next.snapshot,
      });
    }
    void queryClient.invalidateQueries({ queryKey: loopsKeys.runsByWorkspace(workspaceId) });
  };

  const command = (action: GoalControlAction, message: string) => {
    setPendingAction(action);
    mutation.mutate(
      { id: sessionId, message },
      {
        onSuccess: response => {
          if (isGoalResult(response)) applyResult(response);
        },
        onError: error => toast.error(error.message),
        onSettled: () => setPendingAction(undefined),
      }
    );
  };

  const resultSnapshot = result?.snapshot;
  const replaceRequired =
    resultSnapshot &&
    (result.reason_code === "goal_replace_required" || result.reason_code === "goal_replace_stale");
  const composerAffordance = replaceRequired
    ? {
        kind: "replace" as const,
        expectedRunId: resultSnapshot.run_id,
        objective: replacementObjective(resultCommand) ?? resultSnapshot.objective,
      }
    : undefined;

  return {
    composerAffordance,
    error: query.error,
    onApprove:
      snapshot?.live && snapshot.run_status === "needs-approval"
        ? () => command("approve", "/goal resume")
        : undefined,
    onClear: snapshot ? () => command("clear", "/goal clear") : undefined,
    onPause:
      snapshot?.live && snapshot.status === "active"
        ? () => command("pause", "/goal pause")
        : undefined,
    onPrefillComposer,
    onResume:
      snapshot?.live && (snapshot.status === "paused" || snapshot.run_status === "paused")
        ? () => command("resume", "/goal resume")
        : undefined,
    pendingAction,
    snapshot,
  };
}
