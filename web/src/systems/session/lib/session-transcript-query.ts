import type { InfiniteData } from "@tanstack/react-query";

import type {
  NormalizedSessionTranscriptEntry,
  NormalizedSessionTranscriptResponse,
  SessionMessage,
  SessionTranscriptPage,
  SessionTranscriptQuery,
  TranscriptDeltaPayload,
  TranscriptSnapshotPayload,
} from "../types";

const DEFAULT_TRANSCRIPT_LIMIT = 200;

export interface OlderTranscriptPageParam {
  beforeSequence: number;
  epoch: number;
  generation: number;
}

export type SessionTranscriptPageParam = OlderTranscriptPageParam | undefined;
export type SessionTranscriptData = InfiniteData<SessionTranscriptPage, unknown>;

interface TranscriptFence {
  epoch: number;
  generation: number;
}

export function transcriptPageFromResponse(
  response: NormalizedSessionTranscriptResponse
): SessionTranscriptPage {
  return { ...response, cursor: response.max_sequence };
}

export function transcriptPageRequest(
  pageParam: SessionTranscriptPageParam
): SessionTranscriptQuery {
  return pageParam ? { before_sequence: pageParam.beforeSequence } : {};
}

export function nextTranscriptPageParam(page: SessionTranscriptPage): SessionTranscriptPageParam {
  if (!page.has_older || page.next_before_sequence === undefined) {
    return undefined;
  }
  return {
    beforeSequence: page.next_before_sequence,
    epoch: page.epoch,
    generation: page.generation,
  };
}

export function transcriptPageMatchesFence(
  response: TranscriptFence,
  pageParam: SessionTranscriptPageParam
): boolean {
  return (
    pageParam === undefined ||
    (response.epoch === pageParam.epoch && response.generation === pageParam.generation)
  );
}

export function flattenTranscriptEntries(
  data: SessionTranscriptData | undefined
): NormalizedSessionTranscriptEntry[] {
  if (!data) return [];
  return mergeTranscriptEntries(...data.pages.map(page => page.entries));
}

export function flattenTranscriptMessages(
  data: SessionTranscriptData | undefined
): SessionMessage[] {
  return flattenTranscriptEntries(data).map(entry => entry.message);
}

export function transcriptFrameMatches(
  data: SessionTranscriptData | undefined,
  frame: TranscriptFence
): boolean {
  const head = data?.pages[0];
  return !head || (head.epoch === frame.epoch && head.generation === frame.generation);
}

export function transcriptStreamCursor(data: SessionTranscriptData | undefined) {
  const head = data?.pages[0];
  if (!head) return {};
  return {
    afterSequence: head.cursor,
    epoch: head.epoch,
    generation: head.generation,
  };
}

export function applyTranscriptSnapshot(
  data: SessionTranscriptData | undefined,
  snapshot: Omit<TranscriptSnapshotPayload, "entries"> & {
    entries: NormalizedSessionTranscriptEntry[];
  },
  eventCursor: number
): SessionTranscriptData {
  const existingHead = data?.pages[0];
  const reset = snapshot.reset || !data || !existingHead;
  const entries = reset
    ? mergeTranscriptEntries(snapshot.entries)
    : mergeTranscriptEntries(existingHead.entries, snapshot.entries);
  const head: SessionTranscriptPage = {
    cursor: reset
      ? Math.max(snapshot.max_sequence, eventCursor)
      : Math.max(existingHead?.cursor ?? 0, snapshot.max_sequence, eventCursor),
    entries,
    epoch: snapshot.epoch,
    generation: snapshot.generation,
    has_older: snapshot.has_older,
    limit: existingHead?.limit ?? Math.max(snapshot.entries.length, DEFAULT_TRANSCRIPT_LIMIT),
    max_sequence: reset
      ? snapshot.max_sequence
      : Math.max(existingHead?.max_sequence ?? 0, snapshot.max_sequence),
    ...(snapshot.next_before_sequence === undefined
      ? {}
      : { next_before_sequence: snapshot.next_before_sequence }),
  };

  if (reset) {
    return withBoundedTranscriptHead(undefined, head);
  }
  return withBoundedTranscriptHead(data, head);
}

export function applyTranscriptDelta(
  data: SessionTranscriptData | undefined,
  delta: Omit<TranscriptDeltaPayload, "entries"> & {
    entries: NormalizedSessionTranscriptEntry[];
  },
  eventCursor: number
): SessionTranscriptData {
  const existingHead = data?.pages[0];
  const head: SessionTranscriptPage = {
    cursor: Math.max(existingHead?.cursor ?? 0, delta.cursor, eventCursor),
    entries: mergeTranscriptEntries(existingHead?.entries ?? [], delta.entries),
    epoch: delta.epoch,
    generation: delta.generation,
    has_older: existingHead?.has_older ?? false,
    limit: existingHead?.limit ?? DEFAULT_TRANSCRIPT_LIMIT,
    max_sequence: Math.max(existingHead?.max_sequence ?? 0, delta.max_sequence),
    ...(existingHead?.next_before_sequence === undefined
      ? {}
      : { next_before_sequence: existingHead.next_before_sequence }),
  };

  if (!data) {
    return withBoundedTranscriptHead(undefined, head);
  }
  return withBoundedTranscriptHead(data, head);
}

export function reconcileTranscriptTail(
  data: SessionTranscriptData | undefined,
  tail: SessionTranscriptPage
): SessionTranscriptData {
  const existingHead = data?.pages[0];
  if (
    !data ||
    !existingHead ||
    existingHead.epoch !== tail.epoch ||
    existingHead.generation !== tail.generation
  ) {
    return withBoundedTranscriptHead(undefined, tail);
  }
  return withBoundedTranscriptHead(data, {
    ...tail,
    cursor: Math.max(existingHead.cursor, tail.cursor),
    entries: mergeTranscriptEntries(existingHead.entries, tail.entries),
  });
}

export function appendSyntheticTranscriptMessage(
  data: SessionTranscriptData | undefined,
  message: SessionMessage,
  sequence: number
): SessionTranscriptData | undefined {
  const head = data?.pages[0];
  if (!data || !head) return data;
  const syntheticEntry: NormalizedSessionTranscriptEntry = {
    message,
    sequence,
    start_sequence: Number.MAX_SAFE_INTEGER,
  };
  return withBoundedTranscriptHead(data, {
    ...head,
    entries: mergeTranscriptEntries(head.entries, [syntheticEntry]),
  });
}

function mergeTranscriptEntries(
  ...collections: readonly NormalizedSessionTranscriptEntry[][]
): NormalizedSessionTranscriptEntry[] {
  const entriesByStart = new Map<number, NormalizedSessionTranscriptEntry>();
  for (const entries of collections) {
    for (const entry of entries) {
      const current = entriesByStart.get(entry.start_sequence);
      if (!current || entry.sequence >= current.sequence) {
        entriesByStart.set(entry.start_sequence, entry);
      }
    }
  }
  return [...entriesByStart.values()].sort(
    (left, right) => left.start_sequence - right.start_sequence
  );
}

function withBoundedTranscriptHead(
  data: SessionTranscriptData | undefined,
  head: SessionTranscriptPage
): SessionTranscriptData {
  const headLimit = Math.max(1, head.limit);
  const headSplit = Math.max(0, head.entries.length - headLimit);
  let overflow = head.entries.slice(0, headSplit);
  const boundedHead = { ...head, entries: head.entries.slice(headSplit) };
  const existingHistory = data?.pages.slice(1) ?? [];
  const history: SessionTranscriptPage[] = [];

  for (const page of existingHistory) {
    if (overflow.length === 0) {
      history.push(page);
      continue;
    }
    const entries = mergeTranscriptEntries(overflow, page.entries);
    const pageLimit = Math.max(1, page.limit);
    const split = Math.max(0, entries.length - pageLimit);
    overflow = entries.slice(0, split);
    history.push({ ...page, entries: entries.slice(split) });
  }

  while (overflow.length > 0) {
    const template = history.at(-1) ?? boundedHead;
    const pageLimit = Math.max(1, template.limit);
    const split = Math.max(0, overflow.length - pageLimit);
    const entries = overflow.slice(split);
    overflow = overflow.slice(0, split);
    history.push({ ...template, entries });
  }

  const pageParams = data?.pageParams.slice(0, history.length + 1) ?? [undefined];
  while (pageParams.length < history.length + 1) pageParams.push(undefined);
  return { pages: [boundedHead, ...history], pageParams };
}
