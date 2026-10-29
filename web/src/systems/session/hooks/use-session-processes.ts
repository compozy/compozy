import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { SessionNotFoundError } from "../adapters/session-api-errors";
import {
  terminateSessionProcesses,
  type SessionProcessPayload,
  type SessionProcessTarget,
  type TerminateSessionProcessesResponse,
} from "../adapters/session-processes-api";
import { sessionKeys } from "../lib/query-keys";
import { processIdentity, sortSessionProcesses } from "../lib/session-processes";
import { sessionProcessesOptions } from "../lib/session-processes-query-options";
import { invalidateSessionProcessesQueries } from "../lib/session-query-invalidation";

/** `all` stops every process the session started; otherwise the listed identities. */
export type SessionProcessStopRequest =
  | { all: true }
  | { targets: readonly SessionProcessTarget[] };

export interface SessionProcessesModel {
  /** Background and Terminal rows first, then foreground tool calls. */
  processes: readonly SessionProcessPayload[];
  /** False when the daemon cannot scan processes on this system. */
  supported: boolean;
  loading: boolean;
  failed: boolean;
  /** The daemon no longer knows the session, so a retry cannot succeed. */
  missing: boolean;
  retry: () => void;
  /** Resolves with the per-target results; rejects when the request itself fails. */
  stop: (request: SessionProcessStopRequest) => Promise<TerminateSessionProcessesResponse>;
  /** Identities whose stop is in flight; `"all"` while Stop all runs. */
  stopping: ReadonlySet<string> | "all" | null;
}

/** The composer's process list, read only while `enabled` (the popover is open). */
export function useSessionProcesses(
  workspaceId: string,
  sessionId: string,
  profile: string,
  enabled: boolean
): SessionProcessesModel {
  const queryClient = useQueryClient();
  const options = sessionProcessesOptions(workspaceId, sessionId, profile);
  const queryEnabled = options.enabled === true && enabled;
  const query = useQuery({ ...options, enabled: queryEnabled });
  const mutation = useMutation({
    mutationFn: (request: SessionProcessStopRequest) =>
      terminateSessionProcesses(
        workspaceId,
        sessionId,
        profile,
        "all" in request
          ? { all: true }
          : {
              targets: request.targets.map(({ pid, started_at }) => ({ pid, started_at })),
            }
      ),
    onSettled: () =>
      Promise.all([
        invalidateSessionProcessesQueries(queryClient, workspaceId, sessionId),
        queryClient.invalidateQueries({
          queryKey: sessionKeys.detail(workspaceId, sessionId),
          exact: true,
        }),
      ]),
  });
  const pending = mutation.isPending ? mutation.variables : undefined;
  return {
    processes: sortSessionProcesses(query.data?.items ?? []),
    supported: query.data?.supported ?? true,
    loading: query.isPending && queryEnabled,
    failed: query.isError,
    missing: query.error instanceof SessionNotFoundError,
    retry: () => void query.refetch(),
    stop: request => mutation.mutateAsync(request),
    stopping:
      pending === undefined
        ? null
        : "all" in pending
          ? "all"
          : new Set(pending.targets.map(processIdentity)),
  };
}
