import { useQuery } from "@tanstack/react-query";
import { useEffect, useReducer, useRef, useState } from "react";

import type { DiffViewerContents } from "@compozy/ui";

import {
  getSessionChangeContents,
  getSessionChangePatches,
  sessionChangesErrorCode,
  type SessionChangesDetailRequest,
  type SessionChangesScope,
  type SessionChangesSummary,
} from "../adapters/session-changes-api";
import { firstChangedLine, type SessionChangeItem } from "../lib/session-changes";
import {
  createPatchBatcher,
  nextStaleFence,
  type StaleFenceState,
} from "../lib/session-changes-loading";
import { sessionChangesOptions } from "../lib/session-changes-query";

/** Why a file's diff has no patch to draw; its header says so. */
export type SessionChangeNote = "too_large" | "sensitive" | "no_content";

export interface UseSessionChangesInput {
  workspaceId: string;
  sessionId: string;
  scope?: SessionChangesScope;
  base?: string;
  /** Sensitive paths the operator revealed; their patches are fetched with `reveal=true`. */
  revealed: ReadonlySet<string>;
  /** Re-fetches items whose patch was stale once the fresh summary lands. */
  reloadItems: (ids: readonly string[]) => void;
}

export interface SessionChangesModel {
  summary: SessionChangesSummary | undefined;
  /** First load: no summary yet. */
  loading: boolean;
  refreshing: boolean;
  /** The summary read failed with no summary to keep (not a repo, base not found, …). */
  errorCode: string | null;
  errorMessage: string | null;
  /** "Couldn't refresh · Retry": a refresh failed, or the stale fence gave up. */
  refreshFailed: boolean;
  refresh: () => void;
  retry: () => void;
  loadPatch: (item: SessionChangeItem) => Promise<string | null>;
  loadContents: (item: SessionChangeItem) => Promise<DiffViewerContents>;
  noteOf: (id: string) => SessionChangeNote | undefined;
  firstLineOf: (id: string) => number | undefined;
}

class StalePatchError extends Error {
  constructor(path: string) {
    super(`changes moved for ${path}`);
    this.name = "StalePatchError";
  }
}

function detailRequest(
  input: Pick<UseSessionChangesInput, "workspaceId" | "sessionId">,
  summary: SessionChangesSummary
): SessionChangesDetailRequest {
  const scope = summary.scope === "uncommitted" ? "uncommitted" : "branch";
  return {
    workspaceId: input.workspaceId,
    sessionId: input.sessionId,
    scope,
    ...(scope === "branch" && summary.base.ref ? { base: summary.base.ref } : {}),
    revision: summary.revision,
  };
}

/**
 * The Changes tab's data (US-007–US-010): the summary read, batched patch loads fenced by the
 * summary's revision and each file's fingerprint, and the one-shot stale refetch.
 */
export function useSessionChanges(input: UseSessionChangesInput): SessionChangesModel {
  const { workspaceId, sessionId, scope, base, revealed } = input;
  const query = useQuery({
    ...sessionChangesOptions({
      workspaceId,
      sessionId,
      ...(scope ? { scope } : {}),
      ...(base ? { base } : {}),
    }),
    enabled: workspaceId !== "" && sessionId !== "",
  });
  const summary = query.data;
  const [fence, dispatchFence] = useReducer(nextStaleFence, "clean" as StaleFenceState);
  const [notes] = useState(() => new Map<string, SessionChangeNote>());
  const [firstLines] = useState(() => new Map<string, number>());
  const staleIds = useRef(new Set<string>());
  const reloadItemsRef = useRef(input.reloadItems);
  const refetchRef = useRef(query.refetch);
  useEffect(() => {
    reloadItemsRef.current = input.reloadItems;
    refetchRef.current = query.refetch;
  });

  // Refetch once per stale episode; the fresh summary re-renders moved files by version, and
  // files whose version held are reloaded explicitly so none keeps the stale error.
  useEffect(() => {
    if (fence !== "refetching") return;
    let cancelled = false;
    void refetchRef.current().then(() => {
      if (cancelled) return;
      dispatchFence("summary");
      const ids = [...staleIds.current];
      staleIds.current.clear();
      if (ids.length > 0) reloadItemsRef.current(ids);
    });
    return () => {
      cancelled = true;
    };
  }, [fence]);

  const request = summary ? detailRequest({ workspaceId, sessionId }, summary) : null;
  // Batchers live per summary revision; they are created on first load, outside render.
  const [batchers] = useState(() => new Map<string, ReturnType<typeof createPatchBatcher>>());
  const batcherFor = (detail: SessionChangesDetailRequest, reveal: boolean) => {
    const key = `${workspaceId}|${sessionId}|${detail.scope}|${detail.base ?? ""}|${detail.revision}|${reveal}`;
    let batcher = batchers.get(key);
    if (!batcher) {
      if (batchers.size > 4) batchers.clear();
      batcher = createPatchBatcher(targets =>
        getSessionChangePatches(detail, targets, { reveal }).then(response => response.patches)
      );
      batchers.set(key, batcher);
    }
    return batcher;
  };

  const markStale = (id: string) => {
    staleIds.current.add(id);
    dispatchFence("stale");
  };

  const loadPatch = async (item: SessionChangeItem): Promise<string | null> => {
    if (!request) throw new Error("changes are not loaded");
    const target = { path: item.path, ...(item.prevPath ? { oldPath: item.prevPath } : {}) };
    const batcher = batcherFor(request, revealed.has(item.path));
    let patch;
    try {
      patch = await batcher(target);
    } catch (error) {
      if (sessionChangesErrorCode(error) === "changes_stale") markStale(item.id);
      throw error;
    }
    // Same revision, different file identity: the file changed after the summary.
    if (patch.fingerprint !== item.fingerprint) {
      markStale(item.id);
      throw new StalePatchError(item.path);
    }
    dispatchFence("patch");
    notes.delete(item.id);
    if (patch.too_large) {
      notes.set(item.id, "too_large");
      return null;
    }
    if (patch.sensitive) {
      notes.set(item.id, "sensitive");
      return null;
    }
    if (!patch.patch) {
      notes.set(item.id, "no_content");
      return null;
    }
    const line = firstChangedLine(patch.patch);
    if (line !== undefined) firstLines.set(item.id, line);
    return patch.patch;
  };

  const loadContents = async (item: SessionChangeItem): Promise<DiffViewerContents> => {
    if (!request) throw new Error("changes are not loaded");
    let contents;
    try {
      contents = await getSessionChangeContents(
        request,
        { path: item.path, ...(item.prevPath ? { oldPath: item.prevPath } : {}) },
        { reveal: revealed.has(item.path) }
      );
    } catch (error) {
      if (sessionChangesErrorCode(error) === "changes_stale") markStale(item.id);
      throw error;
    }
    if (contents.fingerprint !== item.fingerprint) {
      markStale(item.id);
      throw new StalePatchError(item.path);
    }
    if (contents.old?.sensitive || contents.new?.sensitive) {
      throw new Error("sensitive contents stay hidden");
    }
    return { oldText: contents.old?.text ?? null, newText: contents.new?.text ?? "" };
  };

  const hasSummary = summary !== undefined;
  return {
    summary,
    loading: query.isPending,
    refreshing: query.isFetching && hasSummary,
    errorCode: !hasSummary && query.isError ? (sessionChangesErrorCode(query.error) ?? "") : null,
    errorMessage: !hasSummary && query.isError ? query.error.message : null,
    refreshFailed: (hasSummary && query.isError && !query.isFetching) || fence === "failed",
    refresh: () => void query.refetch(),
    retry: () => {
      if (fence === "failed") dispatchFence("retry");
      else void query.refetch();
    },
    loadPatch,
    loadContents,
    noteOf: id => notes.get(id),
    firstLineOf: id => firstLines.get(id),
  };
}
