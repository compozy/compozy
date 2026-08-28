import { useMutation } from "@tanstack/react-query";
import { useRef } from "react";

import { answerTerminalInputRequest } from "../adapters/terminal-api";
import type { TerminalInputRequest } from "../types";

export interface TerminalInputAnswerVariables {
  request: TerminalInputRequest;
}

export interface AnswerTerminalInputArgs {
  request: TerminalInputRequest;
  value: string;
}

/**
 * Delivers an answer without putting the typed bytes in TanStack's mutation
 * cache. The secret lives in a one-shot map for the duration of the call and
 * is deleted before the request returns; mutation variables carry only the
 * request identity.
 */
export function useTerminalInputAnswer(
  workspaceId: string,
  options: {
    onSuccess: () => void | Promise<void>;
    onError: (error: Error) => void;
  }
) {
  const staged = useRef(new Map<string, string>());
  const mutation = useMutation({
    mutationFn: ({ request }: TerminalInputAnswerVariables) => {
      const value = staged.current.get(request.id) ?? "";
      staged.current.delete(request.id);
      return answerTerminalInputRequest(workspaceId, request.terminal_id, request.id, value, {
        profile: request.profile_name,
      });
    },
    onSuccess: options.onSuccess,
    onError: error => {
      options.onError(error instanceof Error ? error : new Error("Failed to answer"));
    },
  });

  return {
    mutate: ({ request, value }: AnswerTerminalInputArgs) => {
      staged.current.set(request.id, value);
      mutation.mutate({ request });
    },
  };
}
