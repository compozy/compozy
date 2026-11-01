import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";

import { notifyUser } from "@/lib/user-feedback";
import { useProfileReadScope } from "@/systems/profiles";

import {
  archiveAgentInstance,
  createAgentInstance,
  ensureAgentInstanceMain,
  pauseAgentInstance,
  resumeAgentInstance,
} from "../adapters/agent-instance-api";
import { normalizeAgentInstance } from "../lib/agent-instance-wire";
import { agentInstancesOptions, staysOnAgentsOptions } from "../lib/query-options";
import { agentKeys } from "../lib/query-keys";
import type { AgentInstancePayload, CreateAgentInstanceParams } from "../types";

const NO_INSTANCES: readonly AgentInstancePayload[] = [];

/** Merges one instance into the list; an unknown id rereads, since order is the server's. */
export function applyAgentInstanceUpdate(queryClient: QueryClient, payload: AgentInstancePayload) {
  const next = normalizeAgentInstance(payload);
  let known = false;
  queryClient.setQueryData<AgentInstancePayload[]>(agentKeys.instances(), current => {
    if (!current?.some(instance => instance.id === next.id)) return current;
    known = true;
    return current.map(instance => (instance.id === next.id ? next : instance));
  });
  if (!known) void invalidateAgentInstances(queryClient);
}

export function invalidateAgentInstances(queryClient: QueryClient) {
  return queryClient.invalidateQueries({ queryKey: agentKeys.instances() });
}

/** Every stays-on instance; no route behind it yet reads as none, so no row shows. */
export function useAgentInstances(): readonly AgentInstancePayload[] {
  const { data } = useQuery(agentInstancesOptions());
  return data ?? NO_INSTANCES;
}

/** The stays-on agents a scope can start; `""` is Global. Reads only while a picker is open. */
export function useStaysOnAgents(workspaceId: string, enabled: boolean) {
  const { destination } = useProfileReadScope();
  return useQuery(staysOnAgentsOptions(workspaceId, destination, enabled));
}

function useInstanceMutation<TInput, TResult>(
  run: (input: TInput) => Promise<TResult>,
  failure: string
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: run,
    onError: error =>
      notifyUser({ message: error instanceof Error ? error.message : failure, tone: "error" }),
    onSettled: () => invalidateAgentInstances(queryClient),
  });
}

export function useCreateAgentInstance() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: CreateAgentInstanceParams) => createAgentInstance(params),
    onSettled: () => invalidateAgentInstances(queryClient),
  });
}

export function useAgentInstanceActions() {
  const pause = useInstanceMutation(pauseAgentInstance, "Failed to pause this agent.");
  const resume = useInstanceMutation(resumeAgentInstance, "Failed to resume this agent.");
  const archive = useInstanceMutation(archiveAgentInstance, "Failed to archive this agent.");
  const add = useInstanceMutation(createAgentInstance, "Failed to start this agent.");
  const ensureMain = useInstanceMutation(ensureAgentInstanceMain, "Failed to open this agent.");
  return {
    pause: pause.mutate,
    resume: resume.mutate,
    archive: archive.mutate,
    add: add.mutate,
    ensureMain: ensureMain.mutateAsync,
  };
}
