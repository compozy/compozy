import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { notifyUser } from "@/lib/user-feedback";

import {
  isAgentInstanceRouteMissing,
  runAgentInstanceCheck,
  setAgentInstanceChecks,
} from "../adapters/agent-instance-api";
import { checkInReason } from "../lib/agent-check-in-activity";
import { agentKeys } from "../lib/query-keys";
import {
  agentInstanceDayCheckInsOptions,
  agentInstanceOccurrenceOptions,
} from "../lib/query-options";
import type { AgentInstanceCheckDigestItem } from "../types";
import { applyAgentInstanceUpdate } from "./use-agent-instances";

/** Today's check-ins; `null` means no route answers yet, which isn't the same as none. */
export function useAgentInstanceDayCheckIns(id: string | null) {
  const { data, error } = useQuery({
    ...agentInstanceDayCheckInsOptions(id ?? ""),
    enabled: id !== null,
  });
  if (isAgentInstanceRouteMissing(error)) return null;
  return data?.check_ins ?? [];
}

export function useSetAgentInstanceChecks() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, enabled }: { id: string; enabled: boolean }) =>
      setAgentInstanceChecks(id, enabled),
    onSuccess: instance => applyAgentInstanceUpdate(queryClient, instance),
    onError: error =>
      notifyUser({
        message: isAgentInstanceRouteMissing(error)
          ? "Check-ins can't be changed here yet."
          : error.message,
        tone: "error",
      }),
  });
}

export type TryCheck =
  | { phase: "idle" }
  | { phase: "running" }
  | { phase: "result"; digest: readonly AgentInstanceCheckDigestItem[] }
  | { phase: "stopped"; sentence: string }
  | { phase: "sent"; sentence: string }
  | { phase: "unavailable" };

const IDLE: TryCheck = { phase: "idle" };
const DONE_TOLD = "Done. It told you in its main chat.";
const DONE_QUIET = "Done. Nothing worth telling you.";

function failedCheck(error: unknown): TryCheck {
  if (isAgentInstanceRouteMissing(error)) return { phase: "unavailable" };
  return {
    phase: "stopped",
    sentence: error instanceof Error ? error.message : "It couldn't run this check.",
  };
}

/** Try it now, then Run it for real: a dry run first, then a real one polled until it finishes. */
export function useTryCheck(id: string | null) {
  const queryClient = useQueryClient();
  const [held, setHeld] = useState<{ id: string | null; state: TryCheck; occurrence?: string }>({
    id,
    state: IDLE,
  });
  const current = held.id === id ? held : { id, state: IDLE };
  const put = (state: TryCheck, occurrence?: string) => setHeld({ id, state, occurrence });

  const occurrence = useQuery({
    ...agentInstanceOccurrenceOptions(id ?? "", current.occurrence ?? ""),
    enabled: Boolean(id && current.occurrence),
  });
  const finished = current.occurrence && occurrence.data?.state === "finished";

  const tryIt = async () => {
    if (!id) return;
    put({ phase: "running" });
    try {
      const { gate, digest } = await runAgentInstanceCheck(id, true);
      put(
        gate && gate !== "would_run"
          ? { phase: "stopped", sentence: checkInReason(gate) }
          : { phase: "result", digest: digest ?? [] }
      );
    } catch (error) {
      put(failedCheck(error));
    }
  };

  const runForReal = async () => {
    if (!id) return;
    put({ phase: "running" });
    try {
      const { gate, occurrence_id: started } = await runAgentInstanceCheck(id, false);
      if (started) put({ phase: "running" }, started);
      else put({ phase: "stopped", sentence: checkInReason(gate) });
    } catch (error) {
      put(failedCheck(error));
    }
    void queryClient.invalidateQueries({ queryKey: agentKeys.instanceCheckIns(id) });
  };

  let state = current.state;
  if (finished) {
    state = {
      phase: "sent",
      sentence: occurrence.data?.result === "quiet" ? DONE_QUIET : DONE_TOLD,
    };
  } else if (current.occurrence && occurrence.error) {
    state = failedCheck(occurrence.error);
  }
  return { state, tryIt, runForReal, reset: () => put(IDLE) };
}
