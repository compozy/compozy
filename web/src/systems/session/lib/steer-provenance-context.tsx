import type { ReactNode } from "react";

import { useSessionTranscriptThreadState } from "../hooks/use-session-transcript-thread-messages";
import { deriveSteerProvenance, type SteerProvenanceIndex } from "./steer-provenance";
import { SteerProvenanceContext } from "./steer-provenance-context-value";

// One derivation per loaded message list: the index is a pure function of the
// thread messages, cached by their identity so unrelated renders reuse it and
// nothing is patched into the runtime or the query cache.
const indexByMessages = new WeakMap<object, SteerProvenanceIndex>();

function indexFor(messages: readonly object[]): SteerProvenanceIndex {
  const cached = indexByMessages.get(messages);
  if (cached) return cached;
  const index = deriveSteerProvenance(messages);
  indexByMessages.set(messages, index);
  return index;
}

/** Binds steer markers to message identities over the thread's loaded messages (per thread, per session). */
export function SteerProvenanceProvider({ children }: { children: ReactNode }) {
  const { messages } = useSessionTranscriptThreadState();
  return (
    <SteerProvenanceContext.Provider value={indexFor(messages)}>
      {children}
    </SteerProvenanceContext.Provider>
  );
}
