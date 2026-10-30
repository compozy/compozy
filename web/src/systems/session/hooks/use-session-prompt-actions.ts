import { useMutation, useQueryClient } from "@tanstack/react-query";

import { createClientId } from "@/lib/client-id";
import { useActiveWorkspace } from "@/systems/workspace";
import {
  cancelQueuedSessionPrompt,
  SessionGoalCommandError,
  sendSessionPrompt,
} from "../adapters/session-api";
import type { SessionPromptRuntimeSnapshot } from "../contexts/session-prompt-runtime-context-value";
import { sessionKeys } from "../lib/query-keys";
import { invalidateSessionMutationQueries } from "../lib/session-query-invalidation";
import { sessionStore } from "../stores/session-store";
import type {
  SessionPromptAttachment,
  SessionPromptPayload,
  SessionPromptRequest,
  SessionPromptSendResult,
} from "../types";
import {
  requireWorkspace,
  resolveWorkspaceId,
  type UseSessionWorkspaceOptions,
} from "./session-action-workspace";

export interface SessionPromptActionParams {
  attachments?: SessionPromptAttachment[];
  expectedTurnId?: string;
  idempotencyKey?: string;
  id: string;
  messageId?: string;
  message: string;
  runtime?: SessionPromptRuntimeSnapshot;
}

export interface SendSessionPromptParams extends SessionPromptActionParams {
  mode?: SessionPromptRequest["mode"];
}

export interface CancelQueuedSessionPromptParams {
  id: string;
  queueEntryId: string;
}

function createPromptIdentity(): { idempotencyKey: string; messageId: string } {
  return {
    idempotencyKey: createClientId(),
    messageId: createClientId(),
  };
}

const actionPromptIdentities = new WeakMap<
  SessionPromptActionParams,
  {
    idempotencyKey: string;
    messageId: string;
  }
>();

function promptIdentityForAction(params: SessionPromptActionParams): {
  idempotencyKey: string;
  messageId: string;
} {
  if (params.messageId !== undefined || params.idempotencyKey !== undefined) {
    if (
      typeof params.messageId !== "string" ||
      params.messageId.trim().length === 0 ||
      typeof params.idempotencyKey !== "string" ||
      params.idempotencyKey.trim().length === 0
    ) {
      throw new Error(
        "A session prompt action requires both non-empty message_id and idempotency_key"
      );
    }
    return { idempotencyKey: params.idempotencyKey, messageId: params.messageId };
  }
  const existing = actionPromptIdentities.get(params);
  if (existing) return existing;
  const identity = createPromptIdentity();
  actionPromptIdentities.set(params, identity);
  return identity;
}

function promptRequestFromAction(
  params: SessionPromptActionParams,
  mode?: SendSessionPromptParams["mode"]
): SessionPromptRequest {
  const identity = promptIdentityForAction(params);
  return {
    ...(params.attachments?.length ? { attachments: params.attachments } : {}),
    idempotency_key: identity.idempotencyKey,
    message_id: identity.messageId,
    messages: [
      {
        id: identity.messageId,
        parts: [{ text: params.message, type: "text" }],
        role: "user",
      },
    ],
    ...(mode ? { mode } : {}),
    ...(params.expectedTurnId ? { expected_turn_id: params.expectedTurnId } : {}),
    ...(params.runtime ? { runtime: params.runtime } : {}),
  };
}

/**
 * The one busy-send mutation: `mode` names the verb (steer | queue | interrupt)
 * the composer resolved — from the daemon default, the one-shot modifier, or an
 * explicit button. `expectedTurnId` is a strict fence when provided; omitted, the
 * daemon resolves the active turn at admission and echoes it (invariant 6).
 */
export function useSendSessionPrompt(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, runtimeWorkspaceId);

  return useMutation<SessionPromptSendResult, Error, SendSessionPromptParams>({
    mutationFn: params =>
      sendSessionPrompt(
        requireWorkspace(workspaceId),
        params.id,
        promptRequestFromAction(params, params.mode)
      ),
    /** Projects typed Goal outcomes before refreshing authoritative session reads. */
    onSettled: (data, error, params) => {
      const goal =
        error instanceof SessionGoalCommandError
          ? error.result
          : data && "outcome" in data
            ? data
            : null;
      if (goal) {
        sessionStore.trigger.goalCommandReported({
          sessionId: params.id,
          command: params.message,
          result: goal,
        });
        if (workspaceId && (goal.snapshot !== null || goal.outcome === "cleared")) {
          queryClient.setQueryData(sessionKeys.goal(workspaceId, params.id), {
            goal: goal.snapshot,
          });
        }
      }
      if (workspaceId) void invalidateSessionMutationQueries(queryClient, workspaceId, params.id);
    },
  });
}

export function useCancelQueuedSessionPrompt(options: UseSessionWorkspaceOptions = {}) {
  const queryClient = useQueryClient();
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const workspaceId = resolveWorkspaceId(options.workspaceId, runtimeWorkspaceId);

  return useMutation<SessionPromptPayload, Error, CancelQueuedSessionPromptParams>({
    mutationFn: ({ id, queueEntryId }) =>
      cancelQueuedSessionPrompt(requireWorkspace(workspaceId), id, queueEntryId),
    onSettled: (_data, _error, params) => {
      if (workspaceId) void invalidateSessionMutationQueries(queryClient, workspaceId, params.id);
    },
  });
}
