import type { ReactNode } from "react";
import type { ThreadMessage } from "@assistant-ui/react";

import {
  SessionTranscriptThreadContext,
  type SessionTranscriptThreadStatus,
} from "./session-transcript-thread-context-value";

const noop = () => {};

export function SessionTranscriptThreadProvider({
  children,
  messages,
  status,
  error,
  hasOlder = false,
  isFetchingOlder = false,
  loadOlder = noop,
  retry,
}: {
  children: ReactNode;
  messages: readonly ThreadMessage[];
  status: SessionTranscriptThreadStatus;
  error: Error | null;
  hasOlder?: boolean;
  isFetchingOlder?: boolean;
  loadOlder?: () => void;
  retry: () => void;
}) {
  const contextValue = {
    messages,
    status,
    isPending: status === "pending",
    isError: status === "error",
    error,
    hasOlder,
    isFetchingOlder,
    loadOlder,
    retry,
  };
  return (
    <SessionTranscriptThreadContext.Provider value={contextValue}>
      {children}
    </SessionTranscriptThreadContext.Provider>
  );
}
