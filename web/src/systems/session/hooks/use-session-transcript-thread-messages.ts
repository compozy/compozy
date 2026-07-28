import { use } from "react";
import type { ThreadMessage } from "@assistant-ui/react";

import {
  SessionTranscriptErrorContext,
  SessionTranscriptFetchingOlderContext,
  SessionTranscriptHasOlderContext,
  SessionTranscriptLoadOlderContext,
  SessionTranscriptMessagesContext,
  SessionTranscriptRetryContext,
  SessionTranscriptStatusContext,
  type SessionTranscriptThreadState,
} from "../lib/session-transcript-thread-context-value";

function requireContext<T>(value: T | undefined, name: string): T {
  if (value === undefined) {
    throw new Error(`Session transcript ${name} consumers must render within a thread provider`);
  }
  return value;
}

export function useSessionTranscriptThreadMessages(): readonly ThreadMessage[] {
  return requireContext(use(SessionTranscriptMessagesContext), "message");
}

export function useSessionTranscriptThreadStatus(): SessionTranscriptThreadState["status"] {
  return requireContext(use(SessionTranscriptStatusContext), "status");
}

export function useSessionTranscriptThreadState(): SessionTranscriptThreadState {
  const messages = requireContext(use(SessionTranscriptMessagesContext), "message");
  const status = requireContext(use(SessionTranscriptStatusContext), "status");
  const error = requireContext(use(SessionTranscriptErrorContext), "error");
  const retry = requireContext(use(SessionTranscriptRetryContext), "retry");
  const hasOlder = requireContext(use(SessionTranscriptHasOlderContext), "pagination");
  const isFetchingOlder = requireContext(use(SessionTranscriptFetchingOlderContext), "pagination");
  const loadOlder = requireContext(use(SessionTranscriptLoadOlderContext), "pagination");

  return {
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
}
