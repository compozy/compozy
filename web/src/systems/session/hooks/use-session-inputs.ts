import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  cancelQueuedSessionPrompt,
  clearSessionInputs,
  promoteSessionInputToSteer,
  replaceSessionInput,
} from "../adapters/session-api";
import { sessionKeys } from "../lib/query-keys";
import { SESSION_INPUTS_REFETCH_INTERVAL_MS, sessionInputsOptions } from "../lib/query-options";
import { withCachedInputs } from "../lib/queued-prompt";
import type {
  PromoteSessionInputRequest,
  ReplaceSessionInputRequest,
  SessionInputClearResponse,
  SessionInputPayload,
  SessionInputsResponse,
  SessionPromptPayload,
} from "../types";

export function useSessionInputs(
  workspaceId: string,
  sessionId: string,
  options: { enabled?: boolean; refetchInterval?: number } = {}
) {
  const queryOptions = sessionInputsOptions(workspaceId, sessionId, options.enabled ?? true);
  const { refetchInterval } = options;
  return useQuery(
    // A caller may tighten the cadence (the 1s control poll during a prompt POST); never loosen it.
    refetchInterval === undefined
      ? queryOptions
      : {
          ...queryOptions,
          refetchInterval: Math.min(refetchInterval, SESSION_INPUTS_REFETCH_INTERVAL_MS),
        }
  );
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
  return withCachedInputs(
    current,
    current.inputs.map(input => (input.id === replacedQueueEntryId ? replacement : input))
  );
}

function removeCachedInput(
  current: SessionInputsResponse | undefined,
  queueEntryId: string
): SessionInputsResponse {
  return withCachedInputs(
    current,
    current?.inputs.filter(input => input.id !== queueEntryId) ?? []
  );
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

/** What the daemon's clear answer leaves in the list: every entry it did not cancel. */
function survivingInputs(
  current: SessionInputsResponse | undefined,
  response: SessionInputClearResponse
): SessionInputsResponse {
  return withCachedInputs(
    current,
    response.inputs.filter(input => input.status !== "canceled")
  );
}

export function useClearSessionInputs(workspaceId: string, sessionId: string) {
  const queryClient = useQueryClient();
  const queryKey = sessionKeys.inputQueue(workspaceId, sessionId);
  return useMutation<SessionInputClearResponse, Error, void>({
    mutationFn: () => clearSessionInputs(workspaceId, sessionId),
    onSuccess: response => {
      queryClient.setQueryData<SessionInputsResponse>(queryKey, current =>
        survivingInputs(current, response)
      );
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey, exact: true });
    },
  });
}
