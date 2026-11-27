import { createStore } from "@xstate/store";

import {
  isRemoteComputerId,
  LOCAL_COMPUTER_ID,
  normalizeComputerId,
  type ComputerId,
} from "@/lib/computer-scope";

import type { SessionPayload } from "../types";

/** Stable identity of one session row across computers (`P6`). */
export interface SessionRowRef {
  computerId: ComputerId;
  sessionId: string;
}

const SESSION_ROW_KEY_SEP = "\u0000";

/**
 * Canonical key for one row in maps, React keys, selection sets and drag payloads.
 * Local rows keep their bare session id; remote rows prefix their computer id so
 * identical session ids on two daemons never collide.
 */
export function sessionRowKey(ref: SessionRowRef): string {
  const computerId = normalizeComputerId(ref.computerId);
  return isRemoteComputerId(computerId)
    ? `${computerId}${SESSION_ROW_KEY_SEP}${ref.sessionId}`
    : ref.sessionId;
}

/** Recovers `{ computerId, sessionId }` from a key produced by `sessionRowKey`. */
export function parseSessionRowKey(key: string): SessionRowRef {
  const separator = key.indexOf(SESSION_ROW_KEY_SEP);
  if (separator === -1) return { computerId: LOCAL_COMPUTER_ID, sessionId: key };
  return {
    computerId: normalizeComputerId(key.slice(0, separator)),
    sessionId: key.slice(separator + 1),
  };
}

/** Row key for a session payload whose computer is known or inferred from provenance. */
export function sessionRowKeyFor(session: SessionPayload, computerId?: ComputerId): string {
  return sessionRowKey({
    computerId: computerId ?? sessionComputerOf(session),
    sessionId: session.id,
  });
}

// WeakMap preserves frozen TanStack Query payloads while giving two payloads with
// the same session id on different computers distinct computer ownership.
const SESSION_COMPUTER_OWNERSHIP = new WeakMap<SessionPayload, ComputerId>();

/** Associates a fetched session payload with the computer that served it. */
export function recordSessionComputer<T extends SessionPayload>(
  session: T,
  computerId?: ComputerId
): T {
  const normalized = normalizeComputerId(computerId);
  if (isRemoteComputerId(normalized)) {
    SESSION_COMPUTER_OWNERSHIP.set(session, normalized);
  }
  return session;
}

/** Associates a page of session payloads with the computer that served them. */
export function recordSessionsComputer<T extends SessionPayload>(
  sessions: readonly T[],
  computerId?: ComputerId
): readonly T[] {
  const normalized = normalizeComputerId(computerId);
  if (isRemoteComputerId(normalized)) {
    for (const session of sessions) SESSION_COMPUTER_OWNERSHIP.set(session, normalized);
  }
  return sessions;
}

/** Resolves the computer owning a session payload; defaults to this computer. */
export function sessionComputerOf(session: SessionPayload, fallback?: ComputerId): ComputerId {
  return SESSION_COMPUTER_OWNERSHIP.get(session) ?? normalizeComputerId(fallback);
}

interface SessionDragComputerContext {
  sourceComputerId: ComputerId | null;
  refusedComputerId: ComputerId | null;
}

/**
 * Shared drag-source computer state across sibling computer sections.
 * HTML5 dragover cannot read `dataTransfer` values, so a section checks this store
 * to refuse a drop from another computer and show the Q10 refusal reason inline.
 */
export const sessionDragComputerStore = createStore({
  context: {
    sourceComputerId: null,
    refusedComputerId: null,
  } as SessionDragComputerContext,
  on: {
    started: (_context, { computerId }: { computerId: ComputerId }) => ({
      sourceComputerId: normalizeComputerId(computerId),
      refusedComputerId: null,
    }),
    hoveredComputer: (context, { computerId }: { computerId: ComputerId }) => {
      if (context.sourceComputerId === null) return context;
      const target = normalizeComputerId(computerId);
      const refusedComputerId = target !== context.sourceComputerId ? target : null;
      return context.refusedComputerId === refusedComputerId
        ? context
        : { ...context, refusedComputerId };
    },
    leftComputer: (context, { computerId }: { computerId: ComputerId }) => {
      const target = normalizeComputerId(computerId);
      return context.refusedComputerId === target
        ? { ...context, refusedComputerId: null }
        : context;
    },
    ended: () => ({
      sourceComputerId: null,
      refusedComputerId: null,
    }),
  },
});
