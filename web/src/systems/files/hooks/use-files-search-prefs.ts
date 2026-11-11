import { createStoreLogic } from "@xstate/store";
import { persist, rehydrateStore } from "@xstate/store/persist";
import { useSelector } from "@xstate/store-react";

import type { FilesRootRef } from "../adapters/files-api";

/** Content search toggles and glob fields, remembered per root (US-014.AC-7). */
export interface FilesSearchOptions {
  regex: boolean;
  caseSensitive: boolean;
  wholeWord: boolean;
  /** Comma-separated globs as typed: `web/**, *.go`. */
  include: string;
  exclude: string;
  /** The "files to include/exclude" row is open. */
  showGlobs: boolean;
}

export interface FilesRootPrefs {
  /** Recently opened files, newest first (US-013.AC-7). */
  recent: string[];
  /** Last search queries, newest first. */
  history: string[];
  options: FilesSearchOptions;
}

export const FILES_RECENT_LIMIT = 10;
export const FILES_HISTORY_LIMIT = 10;
export const FILES_SEARCH_PREFS_STORAGE_KEY = "compozy:files:search:v1";

export const DEFAULT_FILES_SEARCH_OPTIONS: FilesSearchOptions = {
  regex: false,
  caseSensitive: false,
  wholeWord: false,
  include: "",
  exclude: "",
  showGlobs: false,
};

const EMPTY_PREFS: FilesRootPrefs = {
  recent: [],
  history: [],
  options: DEFAULT_FILES_SEARCH_OPTIONS,
};

interface FilesSearchPrefsContext {
  byRoot: Record<string, FilesRootPrefs>;
}

/** Stable storage key for one root: the session's root, or a project checkout/worktree. */
export function filesRootKey(root: FilesRootRef): string {
  return root.kind === "session"
    ? `session:${root.workspaceId}:${root.sessionId}`
    : `workspace:${root.workspaceId}:${root.worktreeId ?? ""}`;
}

function pushUnique(list: readonly string[], value: string, limit: number): string[] {
  return [value, ...list.filter(item => item !== value)].slice(0, limit);
}

function stringList(value: unknown, limit: number): string[] {
  if (!Array.isArray(value)) return [];
  return value.filter((item): item is string => typeof item === "string").slice(0, limit);
}

function normalizedOptions(value: unknown): FilesSearchOptions {
  if (typeof value !== "object" || value === null) return DEFAULT_FILES_SEARCH_OPTIONS;
  const record = value as Record<string, unknown>;
  const flag = (key: keyof FilesSearchOptions) =>
    typeof record[key] === "boolean" ? (record[key] as boolean) : false;
  const text = (key: keyof FilesSearchOptions) =>
    typeof record[key] === "string" ? (record[key] as string) : "";
  return {
    regex: flag("regex"),
    caseSensitive: flag("caseSensitive"),
    wholeWord: flag("wholeWord"),
    include: text("include"),
    exclude: text("exclude"),
    showGlobs: flag("showGlobs"),
  };
}

function normalizedPrefs(value: unknown): FilesRootPrefs | null {
  if (typeof value !== "object" || value === null) return null;
  const record = value as Record<string, unknown>;
  return {
    recent: stringList(record.recent, FILES_RECENT_LIMIT),
    history: stringList(record.history, FILES_HISTORY_LIMIT),
    options: normalizedOptions(record.options),
  };
}

function mergePrefs(
  persisted: Partial<FilesSearchPrefsContext>,
  current: FilesSearchPrefsContext
): FilesSearchPrefsContext {
  const byRoot = { ...current.byRoot };
  const stored = persisted.byRoot;
  if (typeof stored === "object" && stored !== null && !Array.isArray(stored)) {
    for (const [key, value] of Object.entries(stored)) {
      const prefs = normalizedPrefs(value);
      if (prefs) byRoot[key] = prefs;
    }
  }
  return { byRoot };
}

function update(
  context: FilesSearchPrefsContext,
  key: string,
  change: (prefs: FilesRootPrefs) => FilesRootPrefs
): FilesSearchPrefsContext {
  return { byRoot: { ...context.byRoot, [key]: change(context.byRoot[key] ?? EMPTY_PREFS) } };
}

export const filesSearchPrefsLogic = createStoreLogic({
  context: (): FilesSearchPrefsContext => ({ byRoot: {} }),
  on: {
    fileOpened: (context, event: { key: string; path: string }) =>
      update(context, event.key, prefs => ({
        ...prefs,
        recent: pushUnique(prefs.recent, event.path, FILES_RECENT_LIMIT),
      })),
    queryRun: (context, event: { key: string; query: string }) =>
      update(context, event.key, prefs => ({
        ...prefs,
        history: pushUnique(prefs.history, event.query, FILES_HISTORY_LIMIT),
      })),
    optionsChanged: (context, event: { key: string; patch: Partial<FilesSearchOptions> }) =>
      update(context, event.key, prefs => ({
        ...prefs,
        options: { ...prefs.options, ...event.patch },
      })),
  },
});

export const filesSearchPrefsStore = filesSearchPrefsLogic
  .createStore()
  .with(persist({ name: FILES_SEARCH_PREFS_STORAGE_KEY, merge: mergePrefs }));

if (typeof window !== "undefined") {
  window.addEventListener("storage", event => {
    if (event.key === FILES_SEARCH_PREFS_STORAGE_KEY) {
      void rehydrateStore(filesSearchPrefsStore);
    }
  });
}

/** Records an opened file for a root (by `filesRootKey`); usable outside render. */
export function recordRecentFile(key: string, path: string): void {
  filesSearchPrefsStore.trigger.fileOpened({ key, path });
}

export interface UseFilesSearchPrefsResult extends FilesRootPrefs {
  recordQuery: (query: string) => void;
  setOptions: (patch: Partial<FilesSearchOptions>) => void;
}

/** Client-owned search state for one root: recent files, query history, and options. */
export function useFilesSearchPrefs(root: FilesRootRef): UseFilesSearchPrefsResult {
  const key = filesRootKey(root);
  const prefs = useSelector(
    filesSearchPrefsStore,
    snapshot => snapshot.context.byRoot[key] ?? EMPTY_PREFS
  );
  return {
    ...prefs,
    recordQuery: query => filesSearchPrefsStore.trigger.queryRun({ key, query }),
    setOptions: patch => filesSearchPrefsStore.trigger.optionsChanged({ key, patch }),
  };
}
