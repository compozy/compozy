import { createStore } from "@xstate/store";

import {
  isRemoteComputerId,
  LOCAL_COMPUTER_ID,
  normalizeComputerId,
  type ComputerId,
} from "@/lib/computer-scope";

import type { SessionListTarget, SessionPayload } from "../types";

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

/** Row key for a session payload whose computer is known or inferred from its data. */
export function sessionRowKeyFor(
  session: Pick<SessionListTarget, "id" | "computerId">,
  computerId?: ComputerId
): string {
  return sessionRowKey({
    computerId: computerId ?? sessionComputerOf(session),
    sessionId: session.id,
  });
}

/** Stamps computer ownership onto a fetched session payload so structural sharing preserves it. */
export function withSessionComputer<T extends SessionPayload>(
  session: T,
  computerId?: ComputerId
): T {
  const normalized = normalizeComputerId(computerId);
  if (!isRemoteComputerId(normalized)) return session;
  return session.computerId === normalized ? session : { ...session, computerId: normalized };
}

/** Stamps computer ownership onto a page of fetched session payloads. */
export function withSessionsComputer<T extends SessionPayload>(
  sessions: readonly T[],
  computerId?: ComputerId
): T[] {
  const normalized = normalizeComputerId(computerId);
  if (!isRemoteComputerId(normalized)) return [...sessions];
  return sessions.map(session => withSessionComputer(session, normalized));
}

/** Resolves the computer owning a session payload; defaults to this computer. */
export function sessionComputerOf(
  session: Pick<SessionListTarget, "computerId">,
  fallback?: ComputerId
): ComputerId {
  return normalizeComputerId(session.computerId ?? fallback);
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
