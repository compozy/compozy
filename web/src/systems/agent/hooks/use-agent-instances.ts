import { useAgentWindowScope } from "./use-agent-window-scope";
import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";

import { notifyUser } from "@/lib/user-feedback";
import { useProfilePerProfileScope, useProfileReadScope } from "@/systems/profiles";

import {
  archiveAgentInstance,
  createAgentInstance,
  ensureAgentInstanceMain,
  pauseAgentInstance,
  resumeAgentInstance,
  type AgentInstancesRead,
} from "../adapters/agent-instance-api";
import {
  agentInstancesOptions,
  staysOnAgentsOptions,
  agentTeamOptions,
} from "../lib/query-options";
import { agentKeys } from "../lib/query-keys";
import { agentInstanceCapacity, agentInstanceHoldsSlot } from "../lib/agent-instance";
import type {
  AgentPayload,
  AgentInstancePayload,
  AgentInstanceWirePayload,
  CreateAgentInstanceParams,
} from "../types";

const NO_INSTANCES: readonly AgentInstancePayload[] = [];

/**
 * Merges one instance into every view that holds it; an unknown id rereads, since order is the
 * server's. A row that took or gave back a slot rereads too, because the count is the server's.
 */
export function applyAgentInstanceUpdate(queryClient: QueryClient, next: AgentInstanceWirePayload) {
  let known = false;
  let slotMoved = false;
  queryClient.setQueriesData<AgentInstancesRead>(
    { queryKey: agentKeys.instanceLists() },
    current => {
      const held = current?.instances.find(instance => instance.id === next.id);
      if (!current || !held) return current;
      known = true;
      slotMoved ||= agentInstanceHoldsSlot(held) !== agentInstanceHoldsSlot(next);
      const updated = { ...next, profile_name: held.profile_name };
      return {
        ...current,
        instances: current.instances.map(row => (row.id === next.id ? updated : row)),
      };
    }
  );
  if (!known || slotMoved) void invalidateAgentInstances(queryClient);
}

export function invalidateAgentInstances(queryClient: QueryClient) {
  return queryClient.invalidateQueries({ queryKey: agentKeys.instanceLists() });
}

function useAgentInstancesRead() {
  const { key, names } = useProfilePerProfileScope();
  return useQuery(agentInstancesOptions(key, names)).data;
}

/**
 * The instances with whether their read has settled: answered, failed, or never enabled. Surfaces
 * that act on an instance's profile wait for this rather than treating "not loaded" as "none".
 */
export function useAgentInstancesState(): {
  instances: readonly AgentInstancePayload[];
  settled: boolean;
} {
  const { key, names } = useProfilePerProfileScope();
  const query = useQuery(agentInstancesOptions(key, names));
  return {
    instances: query.data?.instances ?? NO_INSTANCES,
    settled: !query.isPending || names.length === 0,
  };
}

/** Every stays-on instance of the active view; no route behind it yet reads as none, so no row shows. */
export function useAgentInstances(): readonly AgentInstancePayload[] {
  return useAgentInstancesRead()?.instances ?? NO_INSTANCES;
}

/** The slots of the profile a new instance would land in, as the daemon reports them. */
export function useAgentInstanceCapacity() {
  const { destination } = useProfilePerProfileScope();
  return agentInstanceCapacity(useAgentInstancesRead()?.capacity[destination]);
}

/** The stays-on agents a scope can start; `""` is Global. Reads only while a picker is open. */
export function useStaysOnAgents(workspaceId: string, enabled: boolean) {
  const { destination } = useProfileReadScope();
  return useQuery(staysOnAgentsOptions(workspaceId, destination, enabled));
}

/** `failure` null leaves the error to a caller that says so in place. */
function useInstanceMutation<TInput, TResult>(
  run: (input: TInput) => Promise<TResult>,
  failure: string | null
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: run,
    onError: error => {
      if (failure === null) return;
      notifyUser({ message: error instanceof Error ? error.message : failure, tone: "error" });
    },
    onSettled: () => invalidateAgentInstances(queryClient),
  });
}

export function useCreateAgentInstance() {
  const queryClient = useQueryClient();
  const { destination } = useProfilePerProfileScope();
  return useMutation({
    mutationFn: (params: CreateAgentInstanceParams) => createAgentInstance(params, destination),
    onSettled: () => invalidateAgentInstances(queryClient),
  });
}

export function useAgentInstanceActions() {
  const { destination } = useProfilePerProfileScope();
  const pause = useInstanceMutation(pauseAgentInstance, "Failed to pause this agent.");
  const resume = useInstanceMutation(resumeAgentInstance, "Failed to resume this agent.");
  const archive = useInstanceMutation(archiveAgentInstance, "Failed to archive this agent.");
  const add = useInstanceMutation(
    (params: CreateAgentInstanceParams) => createAgentInstance(params, destination),
    "Failed to start this agent."
  );
  const ensureMain = useInstanceMutation(ensureAgentInstanceMain, "Failed to open this agent.");
  return {
    pause: pause.mutate,
    resume: resume.mutate,
    archive: archive.mutate,
    add: add.mutate,
    ensureMain: ensureMain.mutateAsync,
  };
}

/**
 * Resume one or several instances, with the pending and failed states a report shows in place.
 * Every resume is attempted; it fails when any one of them did. `inline` drops the error toast
 * for a caller that shows the failure itself.
 */
export function useResumeAgentInstances({ inline = false }: { inline?: boolean } = {}) {
  return useInstanceMutation(
    async (instances: readonly AgentInstancePayload[]) => {
      const settled = await Promise.allSettled(instances.map(resumeAgentInstance));
      const failed = settled.find(result => result.status === "rejected");
      if (failed) throw failed.reason;
    },
    inline ? null : "Failed to resume this agent."
  );
}

/** Instances must belong to the same profile and resolved definition as the open agent. */
export function useAgentDefinitionInstances(agent: AgentPayload) {
  const { destination } = useProfileReadScope();
  const { workspace } = useAgentWindowScope();
  const { data: team } = useQuery(agentTeamOptions("", destination));
  const scopes = team?.find(
    item => item.name === agent.name && item.definition_digest === agent.definition_digest
  )?.scopes ?? [workspace || "@user"];
  const ownedScopes = new Set(scopes);
  return useAgentInstances().filter(
    instance =>
      instance.profile_name === destination &&
      instance.agent_name === agent.name &&
      ownedScopes.has(instance.workspace_id || "@user")
  );
}

export function useAgentInstanceInScope(name: string, workspace: string | null) {
  const { destination } = useProfileReadScope();
  return useAgentInstances().find(
    instance =>
      instance.agent_name === name &&
      instance.profile_name === destination &&
      instance.workspace_id === (workspace === "@user" ? "" : (workspace ?? ""))
  );
}
