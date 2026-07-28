import type { ReactNode } from "react";
import type { ThreadMessage } from "@assistant-ui/react";

import {
  SessionTranscriptErrorContext,
  SessionTranscriptFetchingOlderContext,
  SessionTranscriptHasOlderContext,
  SessionTranscriptLoadOlderContext,
  SessionTranscriptMessagesContext,
  SessionTranscriptRetryContext,
  SessionTranscriptStatusContext,
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
  return (
    <SessionTranscriptMessagesContext.Provider value={messages}>
      <SessionTranscriptStatusContext.Provider value={status}>
        <SessionTranscriptErrorContext.Provider value={error}>
          <SessionTranscriptRetryContext.Provider value={retry}>
            <SessionTranscriptHasOlderContext.Provider value={hasOlder}>
              <SessionTranscriptFetchingOlderContext.Provider value={isFetchingOlder}>
                <SessionTranscriptLoadOlderContext.Provider value={loadOlder}>
                  {children}
                </SessionTranscriptLoadOlderContext.Provider>
              </SessionTranscriptFetchingOlderContext.Provider>
            </SessionTranscriptHasOlderContext.Provider>
          </SessionTranscriptRetryContext.Provider>
        </SessionTranscriptErrorContext.Provider>
      </SessionTranscriptStatusContext.Provider>
    </SessionTranscriptMessagesContext.Provider>
  );
}
