import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";

import { daemonReadiness, daemonRecoveryFinished } from "../lib/daemon-readiness";
import { runtimeIdentityOptions, statusOptions } from "../lib/query-options";
import type { RuntimeIdentityPayload, StatusPayload } from "../types";
import { recoveryRefetchInterval } from "./use-daemon-recovery";

const UNREACHABLE_REFETCH_INTERVAL_MS = 2_000;
const REACHABLE_REFETCH_INTERVAL_MS = 10_000;

/** One runtime process: a restart keeps the origin and changes this. */
function selectProcess(identity: RuntimeIdentityPayload): string {
  return `${identity.daemon.pid}:${identity.daemon.started_at}`;
}

/** The lifecycle phase the runtime reports, or null for a runtime that reports none. */
function selectPhase(status: StatusPayload): string | null {
  return daemonReadiness(status.daemon)?.phase ?? null;
}

/** An unreachable runtime is asked more often, so its replacement is noticed quickly. */
function refetchInterval(query: { state: { status: string } }): number {
  const unreachable = query.state.status === "error";
  return unreachable ? UNREACHABLE_REFETCH_INTERVAL_MS : REACHABLE_REFETCH_INTERVAL_MS;
}

/**
 * Refreshes the page's server state when a different runtime process answers on the same origin
 * (a restart, or an update that keeps the web bundle). Every cached read belonged to the previous
 * process, so all of them are marked stale and the mounted ones refetch; streams reconnect on
 * their own. The page is not reloaded, so open windows, drafts and scroll positions stay.
 *
 * A restarted runtime answers reads while it still reconciles its session catalog, so a read taken
 * in that window can be incomplete. The cached reads are marked stale once more when the runtime
 * moves from starting or serving to ready.
 */
export function useRuntimeReconnectRefresh(): void {
  const queryClient = useQueryClient();
  const identity = useQuery({
    ...runtimeIdentityOptions(),
    select: selectProcess,
    refetchInterval,
  });
  const current = identity.data ?? null;
  const bound = useRef<string | null>(null);

  useEffect(() => {
    if (current === null) return;
    const previous = bound.current;
    bound.current = current;
    if (previous !== null && previous !== current) void queryClient.invalidateQueries();
  }, [current, queryClient]);

  const status = useQuery({
    ...statusOptions(),
    select: selectPhase,
    refetchInterval: recoveryRefetchInterval,
  });
  const phase = status.data ?? null;
  const boundPhase = useRef<string | null>(null);

  useEffect(() => {
    if (phase === null) return;
    const previousPhase = boundPhase.current;
    boundPhase.current = phase;
    if (daemonRecoveryFinished(previousPhase, phase)) void queryClient.invalidateQueries();
  }, [phase, queryClient]);
}
