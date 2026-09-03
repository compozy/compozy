import { answerTerminalInputRequest } from "../adapters/terminal-api";
import type { TerminalInputRequest } from "../types";

export interface AnswerTerminalInputArgs {
  request: TerminalInputRequest;
  value: string;
}

/**
 * Delivers an answer without putting the typed bytes in TanStack's mutation
 * cache. This command has no cached read model or mutation state, so the secret
 * stays in the call closure and disappears when the request settles.
 */
export function useTerminalInputAnswer(
  workspaceId: string,
  options: {
    onSuccess: () => void | Promise<void>;
    onError: (error: Error) => void;
  }
) {
  return {
    mutate: ({ request, value }: AnswerTerminalInputArgs) => {
      void answerTerminalInputRequest(workspaceId, request.terminal_id, request.id, value, {
        profile: request.profile_name,
      })
        .then(options.onSuccess)
        .catch(error => {
          options.onError(error instanceof Error ? error : new Error("Failed to answer"));
        });
    },
  };
}
