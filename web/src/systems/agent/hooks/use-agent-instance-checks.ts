import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createStore } from "@xstate/store";
import { useSelector } from "@xstate/store-react";
import { useStoreBinding } from "@/hooks/use-store-binding";

import { notifyUser } from "@/lib/user-feedback";

import {
  isAgentInstanceRouteMissing,
  runAgentInstanceCheck,
  setAgentInstanceChecks,
  type AgentInstanceRef,
} from "../adapters/agent-instance-api";
import { checkInReason } from "../lib/agent-check-in-activity";
import { agentKeys } from "../lib/query-keys";
import {
  agentInstanceDayCheckInsOptions,
  agentInstanceOccurrenceOptions,
} from "../lib/query-options";
import type {
  AgentInstanceCheckDigestItem,
  AgentInstanceOccurrence,
  AgentInstanceSourceOutcome,
} from "../types";
import { applyAgentInstanceUpdate } from "./use-agent-instances";

const NO_INSTANCE: AgentInstanceRef = { id: "", profile_name: "" };

/** Today's heartbeats; `null` means no route answers yet, which isn't the same as none. */
export function useAgentInstanceDayCheckIns(ref: AgentInstanceRef | null) {
  const { data, error } = useQuery({
    ...agentInstanceDayCheckInsOptions(ref ?? NO_INSTANCE),
    enabled: ref !== null,
  });
  if (isAgentInstanceRouteMissing(error)) return null;
  return data?.check_ins ?? [];
}

export function useSetAgentInstanceChecks() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ instance, enabled }: { instance: AgentInstanceRef; enabled: boolean }) =>
      setAgentInstanceChecks(instance, enabled),
    onSuccess: instance => applyAgentInstanceUpdate(queryClient, instance),
    onError: error =>
      notifyUser({
        message: isAgentInstanceRouteMissing(error)
          ? "Heartbeat can't be changed here yet."
          : error.message,
        tone: "error",
      }),
  });
}

export type TryCheck =
  | { phase: "idle" }
  | { phase: "running" }
  | {
      phase: "result";
      digest: readonly AgentInstanceCheckDigestItem[];
      /** How each declared source's read went; empty when it declares none. */
      sources: readonly AgentInstanceSourceOutcome[];
    }
  | { phase: "stopped"; sentence: string }
  | { phase: "sent"; sentence: string }
  | { phase: "unavailable" };

const IDLE: TryCheck = { phase: "idle" };
const DONE_TOLD = "Done. It told you in its main chat.";
const DONE_QUIET = "Done. Nothing worth telling you.";
const DONE_FAILED = "It couldn't finish. It told you in its main chat.";

/** What the finished run actually did: only some outcomes reached the main chat. */
function finishedSentence({ result, reason }: AgentInstanceOccurrence) {
  if (result === "quiet") return DONE_QUIET;
  if (result === "reported" || result === "sent") return DONE_TOLD;
  if (result === "failed") return DONE_FAILED;
  return checkInReason(reason);
}

function failedCheck(error: unknown): TryCheck {
  if (isAgentInstanceRouteMissing(error)) return { phase: "unavailable" };
  return {
    phase: "stopped",
    sentence: error instanceof Error ? error.message : "It couldn't run this check.",
  };
}

function createTryCheckStore() {
  return createStore({
    context: { state: IDLE as TryCheck, request: 0, occurrence: undefined as string | undefined },
    on: {
      started: context => ({
        state: { phase: "running" } as TryCheck,
        request: context.request + 1,
        occurrence: undefined,
      }),
      finished: (context, event: { request: number; state: TryCheck; occurrence?: string }) =>
        event.request === context.request
          ? { ...context, state: event.state, occurrence: event.occurrence }
          : context,
      reset: context => ({ state: IDLE, request: context.request + 1, occurrence: undefined }),
    },
  });
}

/** Try it now, then Run it for real: a dry run first, then a real one polled until it finishes. */
export function useTryCheck(ref: AgentInstanceRef | null) {
  const queryClient = useQueryClient();
  const id = ref?.id ?? null;
  const key = `${ref?.profile_name ?? ""}:${id ?? ""}`;
  const { store } = useStoreBinding(key, createTryCheckStore);
  const current = useSelector(store, snapshot => snapshot.context);
  const begin = () => {
    store.trigger.started();
    const request = store.getSnapshot().context.request;
    return (state: TryCheck, occurrence?: string) =>
      store.trigger.finished({ request, state, occurrence });
  };

  const occurrence = useQuery({
    ...agentInstanceOccurrenceOptions(ref ?? NO_INSTANCE, current.occurrence ?? ""),
    enabled: Boolean(id && current.occurrence),
  });
  const finished = current.occurrence && occurrence.data?.state === "finished";

  const tryIt = async () => {
    if (!ref) return;
    const put = begin();
    try {
      const { gate, digest, sources } = await runAgentInstanceCheck(ref, true);
      put(
        gate && gate !== "would_run"
          ? { phase: "stopped", sentence: checkInReason(gate) }
          : { phase: "result", digest: digest ?? [], sources: sources ?? [] }
      );
    } catch (error) {
      put(failedCheck(error));
    }
  };

  const runForReal = async () => {
    if (!ref) return;
    const put = begin();
    try {
      const { gate, occurrence_id: started } = await runAgentInstanceCheck(ref, false);
      if (started) put({ phase: "running" }, started);
      else put({ phase: "stopped", sentence: checkInReason(gate) });
    } catch (error) {
      put(failedCheck(error));
    }
    void queryClient.invalidateQueries({ queryKey: agentKeys.instanceCheckIns(ref.id) });
  };

  let state = current.state;
  if (finished && occurrence.data) {
    state = { phase: "sent", sentence: finishedSentence(occurrence.data) };
  } else if (current.occurrence && occurrence.error) {
    state = failedCheck(occurrence.error);
  }
  return { state, tryIt, runForReal, reset: () => store.trigger.reset() };
}
