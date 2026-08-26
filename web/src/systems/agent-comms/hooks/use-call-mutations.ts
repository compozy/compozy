/**
 * The four operations a calls surface can perform.
 *
 * Every one of them can lose a race — the operator clicks Cancel on a call that
 * settled a moment ago, or messages a child that has just expired. The runtime
 * answers those honestly (cancel on a terminal call is an idempotent 200 with
 * the real terminal state; a dead target is a typed refusal), so the job here is
 * to **re-read rather than assume**: every settle path invalidates the call's
 * cache entries, and the view snaps to daemon truth instead of showing the state
 * the operator expected.
 *
 * That is why there are no optimistic updates on this surface. An optimistic
 * "canceled" that the daemon then contradicts is precisely the phantom state the
 * spec forbids.
 *
 * All four are workspace-scoped: the daemon derives a call's scope from the
 * route, so a mutation without a workspace segment would act on Global work.
 */
import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  cancelCall,
  createCall,
  drainCallSubtree,
  sendCallMessage,
} from "../adapters/agent-comms-api";
import { agentCommsKeys } from "../lib/query-keys";
import type { AgentCommsScope } from "../lib/agent-comms-scope";
import type { CreateSingleCallRequest, SendCallMessageRequest } from "../types";

export function useCallMutations(scope: AgentCommsScope) {
  const queryClient = useQueryClient();

  /** Rows, counts, and one call's record all move together after a mutation. */
  const invalidateScope = () => {
    void queryClient.invalidateQueries({
      queryKey: agentCommsKeys.callsRoot(scope.workspaceId, scope.profileKey),
    });
    void queryClient.invalidateQueries({
      queryKey: agentCommsKeys.callDetails(scope.workspaceId, scope.profileKey),
    });
  };

  const cancel = useMutation({
    mutationFn: ({ callId, reason }: { callId: string; reason?: string }) =>
      cancelCall(scope.workspaceId, callId, reason ? { reason } : {}, scope.actingProfile),
    onSettled: () => invalidateScope(),
  });

  const create = useMutation({
    mutationFn: (body: CreateSingleCallRequest) =>
      createCall(scope.workspaceId, body, scope.actingProfile),
    onSettled: () => invalidateScope(),
  });

  const message = useMutation({
    mutationFn: ({ body }: { body: SendCallMessageRequest }) =>
      sendCallMessage(scope.workspaceId, body, scope.actingProfile),
    // A message to a child can resolve that child's attention cause, so the
    // calls populations move too — not only the mailbox.
    onSettled: () => {
      void queryClient.invalidateQueries({
        queryKey: agentCommsKeys.messagesRoot(scope.workspaceId, scope.profileKey),
      });
      invalidateScope();
    },
  });

  const drainSubtree = useMutation({
    mutationFn: ({
      sessionId,
      reason,
      profile,
    }: {
      sessionId: string;
      reason: string;
      profile: string;
    }) => drainCallSubtree(scope.workspaceId, sessionId, reason, profile),
    onSettled: () => invalidateScope(),
  });

  return { cancel, create, message, drainSubtree };
}
