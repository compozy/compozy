import { createContext } from "react";
import type { ThreadMessage } from "@assistant-ui/react";

export type SessionTranscriptThreadStatus = "pending" | "error" | "success";

export interface SessionTranscriptThreadState {
  messages: readonly ThreadMessage[];
  status: SessionTranscriptThreadStatus;
  error: Error | null;
  hasOlder: boolean;
  isFetchingOlder: boolean;
  isPending: boolean;
  isError: boolean;
  loadOlder: () => void;
  retry: () => void;
}

export const SessionTranscriptMessagesContext = createContext<readonly ThreadMessage[] | undefined>(
  undefined
);
export const SessionDecisionMessagesContext = createContext<readonly ThreadMessage[] | undefined>(
  undefined
);
export const SessionTranscriptStatusContext = createContext<
  SessionTranscriptThreadStatus | undefined
>(undefined);
export const SessionTranscriptErrorContext = createContext<Error | null | undefined>(undefined);
export const SessionTranscriptRetryContext = createContext<(() => void) | undefined>(undefined);
export const SessionTranscriptHasOlderContext = createContext<boolean | undefined>(undefined);
export const SessionTranscriptFetchingOlderContext = createContext<boolean | undefined>(undefined);
export const SessionTranscriptLoadOlderContext = createContext<(() => void) | undefined>(undefined);
