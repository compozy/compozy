import {
  apiClient,
  apiErrorCode,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";
import type { OperationResponse } from "@/lib/api-contract";

export type FileEntriesResponse = OperationResponse<"listSessionFileEntries", 200>;
export type FileEntry = FileEntriesResponse["entries"][number];
export type FileContentResponse = OperationResponse<"readSessionFileContent", 200>;
export type FileRootPayload = FileEntriesResponse["root"];
export type FileFindResponse = OperationResponse<"findSessionFiles", 200>;
export type FileFindMatch = FileFindResponse["results"][number];
export type FileSearchResponse = OperationResponse<"searchSessionFileContents", 200>;
export type FileSearchFile = FileSearchResponse["files"][number];
export type FileSearchMatch = FileSearchFile["matches"][number];

/** Content search request; `include`/`exclude` are root-relative globs (at most ten each). */
export interface FileSearchParams {
  pattern: string;
  regex: boolean;
  caseSensitive: boolean;
  wholeWord: boolean;
  include: string[];
  exclude: string[];
}

/**
 * Which root a read names. A session's root is its bound worktree, else its project checkout
 * (resolved by the daemon); a workspace root names the checkout or one of its worktrees.
 */
export type FilesRootRef =
  | { kind: "session"; workspaceId: string; sessionId: string }
  | { kind: "workspace"; workspaceId: string; worktreeId?: string };

/** A failed files read with the daemon's deterministic error code (`file_not_found`, …). */
export class FilesApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code: string | null
  ) {
    super(message);
    this.name = "FilesApiError";
  }
}

export function filesErrorCode(error: unknown): string | null {
  return error instanceof FilesApiError ? error.code : null;
}

function throwFilesError(response: Response, error: unknown, fallback: string): never {
  throw new FilesApiError(
    defaultApiErrorMessage(fallback, response, error),
    response.status,
    apiErrorCode(error) ?? null
  );
}

/** Lists one directory level of the root; `path` is `""` for the root itself. */
export async function listFileEntries(
  root: FilesRootRef,
  path: string,
  options: { showIgnored: boolean; fresh?: boolean },
  signal?: AbortSignal
): Promise<FileEntriesResponse> {
  const fallback = "Couldn't list files";
  const query = { ...(path ? { path } : {}), show_ignored: options.showIgnored };
  // A refresh skips the daemon's short-lived status snapshot; expansions reuse it.
  const headers = options.fresh ? { "Cache-Control": "no-cache" } : undefined;
  const result =
    root.kind === "session"
      ? await apiClient.GET("/api/workspaces/{workspace_id}/sessions/{session_id}/files/entries", {
          params: {
            path: { workspace_id: root.workspaceId, session_id: root.sessionId },
            // A session owned by another profile still resolves, as the transcript read does.
            query: { ...query, all_profiles: true },
          },
          headers,
          signal,
        })
      : await apiClient.GET("/api/workspaces/{workspace_id}/files/entries", {
          params: {
            path: { workspace_id: root.workspaceId },
            query: { ...query, ...(root.worktreeId ? { worktree_id: root.worktreeId } : {}) },
          },
          headers,
          signal,
        });
  if (apiRequestFailed(result.response, result.error)) {
    throwFilesError(result.response, result.error, fallback);
  }
  return requireResponseData(result.data, result.response, fallback);
}

/**
 * Reads one file. `reveal` is sent only after the operator chose **Show contents** on a
 * sensitive file; it is never persisted (Business Rule 11).
 */
export async function readFileContent(
  root: FilesRootRef,
  path: string,
  options: { reveal: boolean },
  signal?: AbortSignal
): Promise<FileContentResponse> {
  const fallback = "Couldn't read this file";
  const query = { path, ...(options.reveal ? { reveal: true } : {}) };
  const result =
    root.kind === "session"
      ? await apiClient.GET("/api/workspaces/{workspace_id}/sessions/{session_id}/files/content", {
          params: {
            path: { workspace_id: root.workspaceId, session_id: root.sessionId },
            query: { ...query, all_profiles: true },
          },
          signal,
        })
      : await apiClient.GET("/api/workspaces/{workspace_id}/files/content", {
          params: {
            path: { workspace_id: root.workspaceId },
            query: { ...query, ...(root.worktreeId ? { worktree_id: root.worktreeId } : {}) },
          },
          signal,
        });
  if (apiRequestFailed(result.response, result.error)) {
    throwFilesError(result.response, result.error, fallback);
  }
  return requireResponseData(result.data, result.response, fallback);
}

/** Ranks the root's paths by fuzzy name match on the daemon (ADR-005); `q` is never empty. */
export async function findFiles(
  root: FilesRootRef,
  q: string,
  signal?: AbortSignal
): Promise<FileFindResponse> {
  const fallback = "Couldn't search files";
  const result =
    root.kind === "session"
      ? await apiClient.GET("/api/workspaces/{workspace_id}/sessions/{session_id}/files/find", {
          params: {
            path: { workspace_id: root.workspaceId, session_id: root.sessionId },
            query: { q, all_profiles: true },
          },
          signal,
        })
      : await apiClient.GET("/api/workspaces/{workspace_id}/files/find", {
          params: {
            path: { workspace_id: root.workspaceId },
            query: { q, ...(root.worktreeId ? { worktree_id: root.worktreeId } : {}) },
          },
          signal,
        });
  if (apiRequestFailed(result.response, result.error)) {
    throwFilesError(result.response, result.error, fallback);
  }
  return requireResponseData(result.data, result.response, fallback);
}

/** Searches file contents under the root; globs travel as repeated `include`/`exclude`. */
export async function searchFileContents(
  root: FilesRootRef,
  params: FileSearchParams,
  signal?: AbortSignal
): Promise<FileSearchResponse> {
  const fallback = "Couldn't search file contents";
  const query = {
    q: params.pattern,
    ...(params.regex ? { regex: true } : {}),
    ...(params.caseSensitive ? { case_sensitive: true } : {}),
    ...(params.wholeWord ? { whole_word: true } : {}),
    ...(params.include.length > 0 ? { include: params.include } : {}),
    ...(params.exclude.length > 0 ? { exclude: params.exclude } : {}),
  };
  const result =
    root.kind === "session"
      ? await apiClient.GET("/api/workspaces/{workspace_id}/sessions/{session_id}/files/search", {
          params: {
            path: { workspace_id: root.workspaceId, session_id: root.sessionId },
            query: { ...query, all_profiles: true },
          },
          signal,
        })
      : await apiClient.GET("/api/workspaces/{workspace_id}/files/search", {
          params: {
            path: { workspace_id: root.workspaceId },
            query: { ...query, ...(root.worktreeId ? { worktree_id: root.worktreeId } : {}) },
          },
          signal,
        });
  if (apiRequestFailed(result.response, result.error)) {
    throwFilesError(result.response, result.error, fallback);
  }
  return requireResponseData(result.data, result.response, fallback);
}
