import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";
import { useSelector, useStore } from "@xstate/store-react";

import { useDebouncedInput } from "@/hooks/use-debounced-input";

import type { SessionTranscriptSearchMatch } from "../types";
import {
  sessionFindLogic,
  type SessionFindJumpHandlers,
} from "../lib/session-navigation-find-store";
import { sessionKeys } from "../lib/query-keys";
import {
  sessionTranscriptOutlineOptions,
  sessionTranscriptSearchOptions,
} from "../lib/query-options";
import { normalizeTranscriptSearchQuery } from "../lib/session-navigation";

/** How long the "+N new" note stays before the count settles (artboard §04). */
export const SESSION_FIND_APPENDED_NOTE_MS = 4_000;
/** Typing-to-query debounce: enough to skip keystrokes, short enough to feel live. */
export const SESSION_FIND_QUERY_DEBOUNCE_MS = 180;
const EMPTY_MATCHES: readonly SessionTranscriptSearchMatch[] = [];

interface SessionNavigationScope {
  workspaceId: string;
  sessionId: string;
  /**
   * Changes when the durable history changed under the reader (a settled turn,
   * an epoch/generation fence bump): the host derives it from the transcript
   * cache; the reads refetch without stealing focus or resetting selection.
   */
  refreshKey?: string;
}

/**
 * The operator-message outline over full retained history (S9). Refetches on
 * the host's refresh key; keeps the previous rows on screen while it does.
 */
export function useSessionTranscriptOutline({
  workspaceId,
  sessionId,
  refreshKey,
  enabled = true,
}: SessionNavigationScope & { enabled?: boolean }) {
  const queryClient = useQueryClient();
  const query = useQuery(sessionTranscriptOutlineOptions(workspaceId, sessionId, enabled));
  useEffect(() => {
    if (refreshKey === undefined || !enabled) return;
    void queryClient.invalidateQueries({
      queryKey: sessionKeys.transcriptOutline(workspaceId, sessionId),
      exact: true,
    });
  }, [enabled, queryClient, refreshKey, sessionId, workspaceId]);
  return query;
}

/**
 * Full-history literal search for one committed query (S8). A new query is a
 * new cache entry, so an older in-flight answer can never land on it; the
 * previous list stays visible until the next one resolves.
 */
export function useSessionTranscriptSearch({
  workspaceId,
  sessionId,
  query,
  refreshKey,
}: SessionNavigationScope & { query: string }) {
  const queryClient = useQueryClient();
  const search = useQuery(sessionTranscriptSearchOptions(workspaceId, sessionId, query));
  useEffect(() => {
    if (refreshKey === undefined) return;
    void queryClient.invalidateQueries({
      queryKey: sessionKeys.transcriptSearchRoot(workspaceId, sessionId),
    });
  }, [queryClient, refreshKey, sessionId, workspaceId]);
  return search;
}

export interface UseSessionFindOptions extends SessionNavigationScope {
  /** The bar is mounted; closing resets the interaction state. */
  open: boolean;
  /** Seed for the field (selected text when the bar opened). */
  initialQuery?: string;
  handlers: SessionFindJumpHandlers;
}

/**
 * Find in conversation: the field's draft/committed split, the server search
 * for the committed query, and the reader's interaction with the results
 * (active match, stepping, jumping through unloaded history). Live refreshes
 * reconcile by match identity — the active match and the reader's focus never
 * move on their own (US-028.EC-2).
 */
export function useSessionFind({
  workspaceId,
  sessionId,
  open,
  initialQuery = "",
  refreshKey,
  handlers,
}: UseSessionFindOptions) {
  const store = useStore(sessionFindLogic);
  const input = useDebouncedInput({
    delayMs: SESSION_FIND_QUERY_DEBOUNCE_MS,
    externalValue: initialQuery,
    onCommit: () => store.trigger.queryChanged(),
  });
  const committed = normalizeTranscriptSearchQuery(input.committedValue);
  // A draft the debounce has not committed yet is a search in progress, not "no matches".
  const draftPending = normalizeTranscriptSearchQuery(input.draftValue) !== committed;
  const search = useSessionTranscriptSearch({
    query: open ? committed : "",
    refreshKey,
    sessionId,
    workspaceId,
  });
  const matches = committed.length > 0 ? (search.data?.matches ?? EMPTY_MATCHES) : EMPTY_MATCHES;
  const activeSequence = useSelector(store, snapshot => snapshot.context.activeSequence);
  const appended = useSelector(store, snapshot => snapshot.context.appended);
  const jump = useSelector(store, snapshot => snapshot.context.jump);
  const jumpError = useSelector(store, snapshot => snapshot.context.jumpError);

  useEffect(() => {
    store.trigger.matchesObserved({ matches });
  }, [matches, store]);

  useEffect(() => {
    if (!open) store.trigger.closed();
  }, [open, store]);

  useEffect(() => {
    if (appended === 0) return;
    const timer = window.setTimeout(
      () => store.trigger.appendedAcknowledged(),
      SESSION_FIND_APPENDED_NOTE_MS
    );
    return () => window.clearTimeout(timer);
  }, [appended, store]);

  return {
    activeSequence,
    appended,
    committedQuery: committed,
    draftQuery: input.draftValue,
    error: search.error,
    isError: search.isError,
    isFetching: search.isFetching,
    isPending: draftPending || (committed.length > 0 && search.isPending),
    jump,
    jumpError,
    jumpTo: (sequence: number) => store.trigger.jumpRequested({ handlers, sequence }),
    matches,
    retry: () => void search.refetch(),
    select: (sequence: number) => store.trigger.selected({ sequence }),
    setDraftQuery: input.setDraftValue,
    step: (direction: 1 | -1) => store.trigger.stepped({ direction, handlers }),
    truncated: search.data?.truncated ?? false,
  };
}

export type SessionFindModel = ReturnType<typeof useSessionFind>;
