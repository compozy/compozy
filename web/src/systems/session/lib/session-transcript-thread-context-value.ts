import { createContext } from "react";
import type { ThreadMessage } from "@assistant-ui/react";

export type SessionTranscriptThreadStatus = "pending" | "error" | "success";

export interface SessionTranscriptThreadState {
  messages: readonly ThreadMessage[];
  status: SessionTranscriptThreadStatus;
  isPending: boolean;
  isError: boolean;
  error: Error | null;
  hasOlder: boolean;
  isFetchingOlder: boolean;
  loadOlder: () => void;
  retry: () => void;
}

const noop = () => {};

const SessionTranscriptThreadContext = createContext<SessionTranscriptThreadState>({
  messages: [],
  status: "success",
  isPending: false,
  isError: false,
  error: null,
  hasOlder: false,
  isFetchingOlder: false,
  loadOlder: noop,
  retry: noop,
});

export { SessionTranscriptThreadContext };
