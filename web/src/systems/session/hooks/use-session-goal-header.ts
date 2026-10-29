import { useRef } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { sessionKeys } from "../lib/query-keys";
import type { SessionGoalCommandResult, SessionGoalResponse, SessionState } from "../types";
import { describeGoalCommandFailure } from "../lib/session-goal-chat-transport";
import { mutateSessionGoal } from "../adapters/session-goal-api";
import { sessionStore } from "../stores/session-store";
import { useSessionGoalFeedback } from "./use-session-store";
import { useSessionGoal } from "./use-sessions";
import { type GoalControlAction, loopsKeys, useLoopStream } from "@/systems/loops";

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
  options: {
    onPrefillComposer?: (text: string) => void;
    /**
     * One owner per session subscribes to the loop stream for reconciliation
     * (the goal strip); secondary readers pass false
     * and read the shared query cache.
     */
    stream?: boolean;
    enabled?: boolean;
    sessionState?: SessionState;
    stopping?: boolean;
    readOnly?: boolean;
  } = {}
) {
  const { enabled = true, onPrefillComposer, stream = true, readOnly = false } = options;
  const queryClient = useQueryClient();
  const query = useSessionGoal(workspaceId, sessionId, enabled);
  const feedback = useSessionGoalFeedback(sessionId);
  const submitting = useRef(false);
  const snapshot = query.data?.goal ?? null;
  const continuationDisabledReason =
    options.sessionState === "stopping"
      ? "Wait for the session to stop. Send a message to reopen it, then resume the goal."
      : options.stopping
        ? "Wait for the current work to stop before continuing this goal."
        : options.sessionState === "starting"
          ? "Wait for the session to finish starting before continuing this goal."
          : options.sessionState !== "active"
            ? "Send a message to reopen this session, then resume the goal."
            : undefined;

  useLoopStream(workspaceId, snapshot?.run_id ?? "", {
    enabled: enabled && snapshot?.live === true && stream,
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
      if (frame.kind === "goal_status_changed" || frame.kind === "status_changed") {
        void queryClient.invalidateQueries({ queryKey: sessionKeys.goal(workspaceId, sessionId) });
      }
    },
  });

  const applyResult = (next: SessionGoalCommandResult) => {
    // Control errors stay beside these controls; prompt-command errors belong to the composer.
    if (next.outcome !== "error")
      sessionStore.trigger.goalCommandReported({ sessionId, result: next });
    if (next.snapshot !== null || next.outcome === "cleared") {
      queryClient.setQueryData<SessionGoalResponse>(sessionKeys.goal(workspaceId, sessionId), {
        goal: next.snapshot,
      });
    }
    void queryClient.invalidateQueries({ queryKey: loopsKeys.runsByWorkspace(workspaceId) });
    void queryClient.invalidateQueries({ queryKey: sessionKeys.detail(workspaceId, sessionId) });
  };

  const mutation = useMutation({
    mutationFn: async (action: GoalControlAction) => {
      // An older read must not replace the control's acknowledged state.
      await queryClient.cancelQueries({ queryKey: sessionKeys.goal(workspaceId, sessionId) });
      const operation = action === "approve" ? "resume" : action;
      const next = await mutateSessionGoal(workspaceId, sessionId, { operation });
      await queryClient.cancelQueries({ queryKey: sessionKeys.goal(workspaceId, sessionId) });
      applyResult(next);
      if (next.outcome === "error") {
        throw new Error(
          describeGoalCommandFailure(next.reason_code) ?? "Couldn't update the goal. Try again."
        );
      }
      // A terminal frame can race the HTTP response; reread after installing its receipt.
      void queryClient.invalidateQueries({ queryKey: sessionKeys.goal(workspaceId, sessionId) });
      return next;
    },
    onSettled: () => {
      submitting.current = false;
    },
  });

  const command = (action: GoalControlAction) => {
    if (readOnly || submitting.current) return;
    if ((action === "resume" || action === "approve") && continuationDisabledReason) return;
    submitting.current = true;
    mutation.mutate(action);
  };

  const result = feedback?.result;
  const resultSnapshot = result?.snapshot;
  const replaceRequired =
    resultSnapshot &&
    (result.reason_code === "goal_replace_required" || result.reason_code === "goal_replace_stale");
  const composerAffordance = replaceRequired
    ? {
        kind: "replace" as const,
        expectedRunId: resultSnapshot.run_id,
        objective: replacementObjective(feedback?.command) ?? resultSnapshot.objective,
      }
    : undefined;

  return {
    composerAffordance,
    continuationDisabledReason,
    hasLiveGoal: query.data === undefined ? undefined : snapshot?.live === true,
    error: query.error,
    actionError: mutation.error,
    refreshing: query.isFetching,
    onRefresh: () => {
      void query.refetch();
    },
    onApprove:
      !readOnly && snapshot?.live && snapshot.run_status === "needs-approval"
        ? () => command("approve")
        : undefined,
    onClear: !readOnly && snapshot && !snapshot.live ? () => command("clear") : undefined,
    onStop: !readOnly && snapshot?.live ? () => command("stop") : undefined,
    onPause:
      !readOnly && snapshot?.live && snapshot.status === "active"
        ? () => command("pause")
        : undefined,
    onPrefillComposer: readOnly ? undefined : onPrefillComposer,
    onResume:
      !readOnly &&
      snapshot?.live &&
      (snapshot.status === "paused" || snapshot.run_status === "paused")
        ? () => command("resume")
        : undefined,
    pendingAction: mutation.isPending ? mutation.variables : undefined,
    snapshot,
  };
}
