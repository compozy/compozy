import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  cancelQueuedSessionPrompt,
  promoteSessionInputToSteer,
  replaceSessionInput,
} from "../adapters/session-api";
import { sessionKeys } from "../lib/query-keys";
import { sessionInputsOptions } from "../lib/query-options";
import type {
  PromoteSessionInputRequest,
  ReplaceSessionInputRequest,
  SessionInputPayload,
  SessionInputsResponse,
  SessionPromptPayload,
} from "../types";

export function useSessionInputs(
  workspaceId: string,
  sessionId: string,
  options: { enabled?: boolean } = {}
) {
  return useQuery(sessionInputsOptions(workspaceId, sessionId, options.enabled ?? true));
}

export interface ReplaceSessionInputVariables {
  queueEntryId: string;
  request: ReplaceSessionInputRequest;
}

export interface PromoteSessionInputVariables {
  queueEntryId: string;
  request: PromoteSessionInputRequest;
}

function replaceCachedInput(
  current: SessionInputsResponse | undefined,
  replacedQueueEntryId: string,
  replacement: SessionInputPayload
): SessionInputsResponse {
  if (!current) return { inputs: [replacement] };
  return {
    inputs: current.inputs.map(input => (input.id === replacedQueueEntryId ? replacement : input)),
  };
}

function removeCachedInput(
  current: SessionInputsResponse | undefined,
  queueEntryId: string
): SessionInputsResponse {
  return { inputs: current?.inputs.filter(input => input.id !== queueEntryId) ?? [] };
}

export function useReplaceSessionInput(workspaceId: string, sessionId: string) {
  const queryClient = useQueryClient();
  const queryKey = sessionKeys.inputQueue(workspaceId, sessionId);
  return useMutation<SessionInputPayload, Error, ReplaceSessionInputVariables>({
    mutationFn: ({ queueEntryId, request }) =>
      replaceSessionInput(workspaceId, sessionId, queueEntryId, request),
    onSuccess: (replacement, { queueEntryId }) => {
      queryClient.setQueryData<SessionInputsResponse>(queryKey, current =>
        replaceCachedInput(current, queueEntryId, replacement)
      );
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey, exact: true });
    },
  });
}

export function usePromoteSessionInput(workspaceId: string, sessionId: string) {
  const queryClient = useQueryClient();
  const queryKey = sessionKeys.inputQueue(workspaceId, sessionId);
  return useMutation<SessionPromptPayload, Error, PromoteSessionInputVariables>({
    mutationFn: ({ queueEntryId, request }) =>
      promoteSessionInputToSteer(workspaceId, sessionId, queueEntryId, request),
    onSuccess: (_result, { queueEntryId }) => {
      queryClient.setQueryData<SessionInputsResponse>(queryKey, current =>
        removeCachedInput(current, queueEntryId)
      );
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey, exact: true });
    },
  });
}

export function useCancelSessionInput(workspaceId: string, sessionId: string) {
  const queryClient = useQueryClient();
  const queryKey = sessionKeys.inputQueue(workspaceId, sessionId);
  return useMutation<SessionPromptPayload, Error, string>({
    mutationFn: queueEntryId => cancelQueuedSessionPrompt(workspaceId, sessionId, queueEntryId),
    onSuccess: (_result, queueEntryId) => {
      queryClient.setQueryData<SessionInputsResponse>(queryKey, current =>
        removeCachedInput(current, queueEntryId)
      );
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey, exact: true });
    },
  });
}
