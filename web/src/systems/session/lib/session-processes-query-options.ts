import { queryOptions } from "@tanstack/react-query";

import {
  fetchLeftRunningProcesses,
  fetchSessionProcesses,
} from "../adapters/session-processes-api";
import { sessionKeys } from "./query-keys";

/**
 * The session's running processes. A change to the session's `process_summary`
 * upserts it on the catalog stream, which invalidates this read.
 */
export function sessionProcessesOptions(workspaceId: string, sessionId: string, profile: string) {
  return queryOptions({
    queryKey: sessionKeys.processes(workspaceId, sessionId),
    queryFn: ({ signal }) => fetchSessionProcesses(workspaceId, sessionId, profile, signal),
    enabled: workspaceId !== "" && sessionId !== "",
    staleTime: 5_000,
    refetchOnWindowFocus: false,
    retry: false,
  });
}

/** How often an open "Left running" view rescans; deleted sessions have no catalog row to wake it. */
export const PROCESSES_LEFT_RUNNING_REFRESH_MS = 10_000;

/** Processes whose session is stopped, archived, deleted, or unknown, across every project. */
export function processesLeftRunningOptions() {
  return queryOptions({
    queryKey: sessionKeys.processesLeftRunning(),
    queryFn: ({ signal }) => fetchLeftRunningProcesses(signal),
    staleTime: 5_000,
    refetchInterval: PROCESSES_LEFT_RUNNING_REFRESH_MS,
    refetchOnWindowFocus: false,
    retry: false,
  });
}
