import {
  apiClient,
  apiErrorCode,
  apiRequestFailed,
  defaultApiErrorMessage,
  requireResponseData,
} from "@/lib/api-client";
import type { OperationResponse } from "@/lib/api-contract";

export type SessionChangesSummary = OperationResponse<"getSessionChanges", 200>;
export type SessionChangeFile = SessionChangesSummary["files"][number];
export type SessionChangePatches = OperationResponse<"getSessionChangePatches", 200>;
export type SessionChangePatch = SessionChangePatches["patches"][number];
export type SessionChangeContents = OperationResponse<"getSessionChangeContents", 200>;
export type SessionChangeBases = OperationResponse<"listSessionChangeBases", 200>;
export type SessionChangeBaseRef = SessionChangeBases["refs"][number];

export type SessionChangesScope = "branch" | "uncommitted";

/** One session's Changes read: the scope and base the operator chose, or the daemon defaults. */
export interface SessionChangesRequest {
  workspaceId: string;
  sessionId: string;
  /** Omitted: the daemon picks Branch when HEAD is ahead of the merge base (US-008.AC-3). */
  scope?: SessionChangesScope;
  /** Omitted: the worktree base, else the remote default branch, else `main`/`master`. */
  base?: string;
}

/** A detail read fenced by the summary it belongs to (Safety Invariant 6). */
export interface SessionChangesDetailRequest {
  workspaceId: string;
  sessionId: string;
  scope: SessionChangesScope;
  base?: string;
  revision: string;
}

export interface SessionChangeTarget {
  path: string;
  oldPath?: string;
}

/** A failed Changes read with the daemon's code (`changes_stale`, `base_not_found`, …). */
export class SessionChangesApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code: string | null
  ) {
    super(message);
    this.name = "SessionChangesApiError";
  }
}

export function sessionChangesErrorCode(error: unknown): string | null {
  return error instanceof SessionChangesApiError ? error.code : null;
}

function throwChangesError(response: Response, error: unknown, fallback: string): never {
  throw new SessionChangesApiError(
    defaultApiErrorMessage(fallback, response, error),
    response.status,
    apiErrorCode(error) ?? null
  );
}

function sessionPath(request: { workspaceId: string; sessionId: string }) {
  return { workspace_id: request.workspaceId, session_id: request.sessionId };
}

function scopeQuery(scope: SessionChangesScope | undefined, base: string | undefined) {
  return {
    ...(scope ? { scope } : {}),
    // The daemon ignores a base for Uncommitted; never send one there.
    ...(base && scope !== "uncommitted" ? { base } : {}),
    // A session owned by another profile still resolves, as the transcript read does.
    all_profiles: true,
  };
}

export async function getSessionChanges(
  request: SessionChangesRequest,
  signal?: AbortSignal
): Promise<SessionChangesSummary> {
  const fallback = "Couldn't read changes";
  const result = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/changes",
    {
      params: { path: sessionPath(request), query: scopeQuery(request.scope, request.base) },
      signal,
    }
  );
  if (apiRequestFailed(result.response, result.error)) {
    throwChangesError(result.response, result.error, fallback);
  }
  return requireResponseData(result.data, result.response, fallback);
}

/**
 * One batch of per-file patches (at most 20 paths, server cap). `old_path` aligns with `path`
 * positionally and is omitted for a batch without renames.
 */
export async function getSessionChangePatches(
  request: SessionChangesDetailRequest,
  targets: readonly SessionChangeTarget[],
  options: { reveal?: boolean } = {},
  signal?: AbortSignal
): Promise<SessionChangePatches> {
  const fallback = "Couldn't load this diff";
  const renames = targets.some(target => target.oldPath);
  const result = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/changes/patches",
    {
      params: {
        path: sessionPath(request),
        query: {
          ...scopeQuery(request.scope, request.base),
          revision: request.revision,
          path: targets.map(target => target.path),
          ...(renames ? { old_path: targets.map(target => target.oldPath ?? "") } : {}),
          ...(options.reveal ? { reveal: true } : {}),
        },
      },
      signal,
    }
  );
  if (apiRequestFailed(result.response, result.error)) {
    throwChangesError(result.response, result.error, fallback);
  }
  return requireResponseData(result.data, result.response, fallback);
}

/** Both sides of one changed file, for expanding unchanged context. */
export async function getSessionChangeContents(
  request: SessionChangesDetailRequest,
  target: SessionChangeTarget,
  options: { reveal?: boolean } = {},
  signal?: AbortSignal
): Promise<SessionChangeContents> {
  const fallback = "Couldn't load this file";
  const result = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/changes/contents",
    {
      params: {
        path: sessionPath(request),
        query: {
          ...scopeQuery(request.scope, request.base),
          revision: request.revision,
          path: target.path,
          ...(target.oldPath ? { old_path: target.oldPath } : {}),
          ...(options.reveal ? { reveal: true } : {}),
        },
      },
      signal,
    }
  );
  if (apiRequestFailed(result.response, result.error)) {
    throwChangesError(result.response, result.error, fallback);
  }
  return requireResponseData(result.data, result.response, fallback);
}

/** Base refs for the selector: the default first, then local, then remote-tracking. */
export async function listSessionChangeBases(
  request: { workspaceId: string; sessionId: string; query?: string },
  signal?: AbortSignal
): Promise<SessionChangeBases> {
  const fallback = "Couldn't list branches";
  const result = await apiClient.GET(
    "/api/workspaces/{workspace_id}/sessions/{session_id}/changes/bases",
    {
      params: {
        path: sessionPath(request),
        query: { ...(request.query ? { q: request.query } : {}), all_profiles: true },
      },
      signal,
    }
  );
  if (apiRequestFailed(result.response, result.error)) {
    throwChangesError(result.response, result.error, fallback);
  }
  return requireResponseData(result.data, result.response, fallback);
}
