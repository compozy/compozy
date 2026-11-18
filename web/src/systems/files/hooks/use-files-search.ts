import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { useEffect, useMemo, useState } from "react";

import {
  filesErrorCode,
  type FileSearchParams,
  type FileSearchResponse,
  type FilesRootRef,
} from "../adapters/files-api";
import { fileSearchOptions } from "../lib/query-options";
import { splitGlobs } from "../lib/search-globs";
import type { FilesSearchOptions } from "./use-files-search-prefs";

/** Idle time before a typed query runs (US-014.AC-2). */
export const FILES_SEARCH_DEBOUNCE_MS = 300;
export const FILES_SEARCH_MIN_LENGTH = 2;

export type FilesSearchState =
  | { kind: "idle" }
  | { kind: "short" }
  | { kind: "loading" }
  | { kind: "invalid"; message: string }
  | { kind: "unavailable" }
  | { kind: "error"; message: string }
  | { kind: "results"; response: FileSearchResponse; searching: boolean };

export function searchParams(pattern: string, options: FilesSearchOptions): FileSearchParams {
  return {
    pattern,
    regex: options.regex,
    caseSensitive: options.caseSensitive,
    wholeWord: options.wholeWord,
    include: splitGlobs(options.include),
    exclude: splitGlobs(options.exclude),
  };
}

function runnable(pattern: string): boolean {
  return pattern.trim() !== "" && [...pattern].length >= FILES_SEARCH_MIN_LENGTH;
}

function sameParams(a: FileSearchParams | null, b: FileSearchParams): boolean {
  return a !== null && JSON.stringify(a) === JSON.stringify(b);
}

export interface UseFilesSearchResult {
  /** What the operator typed. */
  draft: string;
  setDraft: (next: string) => void;
  /** Runs the current draft now (Enter). */
  submit: () => void;
  /** The request that produced (or is producing) the shown results. */
  submitted: FileSearchParams | null;
  state: FilesSearchState;
}

/**
 * Content search: a query of at least 2 characters runs after 300 ms idle, or at once on
 * Enter. A newer request replaces the older one, whose fetch is aborted when it loses its
 * last observer (US-014.AC-6); the previous results stay dimmed while it runs.
 */
export function useFilesSearch(
  root: FilesRootRef,
  options: FilesSearchOptions,
  initialDraft = ""
): UseFilesSearchResult {
  const [draft, setDraft] = useState(initialDraft);
  const [submitted, setSubmitted] = useState<FileSearchParams | null>(null);
  const pending = useMemo(() => searchParams(draft, options), [draft, options]);

  useEffect(() => {
    if (!runnable(pending.pattern)) return undefined;
    const timer = setTimeout(() => {
      setSubmitted(current => (sameParams(current, pending) ? current : pending));
    }, FILES_SEARCH_DEBOUNCE_MS);
    return () => clearTimeout(timer);
  }, [pending]);

  const active = runnable(draft) ? submitted : null;
  const result = useQuery({
    ...fileSearchOptions(root, active ?? pending),
    enabled: active !== null,
    placeholderData: keepPreviousData,
  });

  let state: FilesSearchState;
  if (draft.trim() === "") state = { kind: "idle" };
  else if (!runnable(draft)) state = { kind: "short" };
  else if (result.isError && !result.isPlaceholderData) {
    const code = filesErrorCode(result.error);
    state =
      code === "invalid_query"
        ? { kind: "invalid", message: result.error.message }
        : code === "root_unavailable"
          ? { kind: "unavailable" }
          : { kind: "error", message: result.error.message };
  } else if (active && result.data) {
    const searching = result.isPlaceholderData || !sameParams(active, pending);
    state = { kind: "results", response: result.data, searching };
  } else state = { kind: "loading" };

  return {
    draft,
    setDraft,
    submit: () => {
      if (runnable(pending.pattern)) setSubmitted(pending);
    },
    submitted: active,
    state,
  };
}
