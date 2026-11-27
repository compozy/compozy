import { useAgentWindowScope } from "./use-agent-window-scope";
import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";

import { isRemoteComputerId, type ComputerId } from "@/lib/computer-scope";
import { notifyUser } from "@/lib/user-feedback";
import { useProfilePerProfileScope, useProfileReadScope, useProfiles } from "@/systems/profiles";

import {
  archiveAgentInstance,
  createAgentInstance,
  ensureAgentInstanceMain,
  pauseAgentInstance,
  markAgentInstanceSeen,
  resumeAgentInstance,
  type AgentInstancesRead,
} from "../adapters/agent-instance-api";
import {
  agentInstancesOptions,
  alwaysOnAgentsOptions,
  agentTeamOptions,
} from "../lib/query-options";
import { resumeInOrder } from "../lib/agent-instance-resume";
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
export function applyAgentInstanceUpdate(
  queryClient: QueryClient,
  next: AgentInstanceWirePayload,
  computerId?: ComputerId
) {
  let known = false;
  let slotMoved = false;
  queryClient.setQueriesData<AgentInstancesRead>(
    { queryKey: agentKeys.forComputer(computerId).instanceLists() },
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
  if (!known || slotMoved) void invalidateAgentInstances(queryClient, computerId);
}

/**
 * An instance changed: a pick, an auto pick or a finished check-in. Its reports and check-ins are
 * reread where something shows them, so a transcript report shows its live pick state.
 */
export function invalidateAgentInstanceReads(
  queryClient: QueryClient,
  id: string,
  computerId?: ComputerId
) {
  void queryClient.invalidateQueries({
    queryKey: agentKeys.forComputer(computerId).instanceReports(id),
  });
  void queryClient.invalidateQueries({
    queryKey: agentKeys.forComputer(computerId).instanceCheckIns(id),
  });
}

export function invalidateAgentInstances(queryClient: QueryClient, computerId?: ComputerId) {
  return queryClient.invalidateQueries({
    queryKey: agentKeys.forComputer(computerId).instanceLists(),
  });
}

function useAgentInstancesRead(computerId?: ComputerId) {
  const remote = isRemoteComputerId(computerId);
  const { key, names: localNames } = useProfilePerProfileScope();
  const remoteProfiles = useProfiles(remote, remote ? computerId : undefined);
  const names = remote ? (remoteProfiles.data ?? []).map(profile => profile.name) : localNames;
  const viewKey = remote ? "remote-all" : key;
  return useQuery({
    ...agentInstancesOptions(viewKey, names, computerId),
    enabled: names.length > 0,
  }).data;
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

/** Every always-on instance of the active view (or owning remote computer); no route behind it yet reads as none. */
export function useAgentInstances(computerId?: ComputerId): readonly AgentInstancePayload[] {
  return useAgentInstancesRead(computerId)?.instances ?? NO_INSTANCES;
}

/**
 * The slots of the profile a new instance would land in, as the daemon reports them. `profile`
 * names another one, e.g. the profile a duplicate's copy lands in.
 */
export function useAgentInstanceCapacity(profile?: string) {
  const { destination } = useProfilePerProfileScope();
  return agentInstanceCapacity(useAgentInstancesRead()?.capacity[profile ?? destination]);
}

/** The always-on agents a scope can start; `""` is Global. Reads only while a picker is open. */
export function useAlwaysOnAgents(workspaceId: string, enabled: boolean) {
  const { destination } = useProfileReadScope();
  return useQuery(alwaysOnAgentsOptions(workspaceId, destination, enabled));
}

/** `failure` null leaves the error to a caller that says so in place. */
function useInstanceMutation<TInput, TResult>(
  run: (input: TInput) => Promise<TResult>,
  failure: string | null,
  computerId?: ComputerId
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: run,
    onError: error => {
      if (failure === null) return;
      notifyUser({ message: error instanceof Error ? error.message : failure, tone: "error" });
    },
    onSettled: () => invalidateAgentInstances(queryClient, computerId),
  });
}

export function useCreateAgentInstance() {
  const queryClient = useQueryClient();
  const { destination } = useProfilePerProfileScope();
  return useMutation({
    /** `profile` starts it in a profile other than the acting one, e.g. a duplicate's source's. */
    mutationFn: ({ profile, ...params }: CreateAgentInstanceParams & { profile?: string }) =>
      createAgentInstance(params, profile ?? destination),
    onSettled: () => invalidateAgentInstances(queryClient),
  });
}

export function useAgentInstanceActions(computerId?: ComputerId) {
  const { destination } = useProfilePerProfileScope();
  const pause = useInstanceMutation(
    (instance: AgentInstancePayload) =>
      computerId ? pauseAgentInstance(instance, computerId) : pauseAgentInstance(instance),
    "Failed to pause this agent.",
    computerId
  );
  const queryClient = useQueryClient();
  const resume = useInstanceMutation(
    async (instance: AgentInstancePayload) => {
      const resumed = await (computerId
        ? resumeAgentInstance(instance, computerId)
        : resumeAgentInstance(instance));
      applyAgentInstanceUpdate(queryClient, resumed, computerId);
      return resumed;
    },
    "Failed to resume this agent.",
    computerId
  );
  const archive = useInstanceMutation(
    (instance: AgentInstancePayload) =>
      computerId ? archiveAgentInstance(instance, computerId) : archiveAgentInstance(instance),
    "Failed to archive this agent.",
    computerId
  );
  const add = useInstanceMutation(
    (params: CreateAgentInstanceParams) => createAgentInstance(params, destination),
    "Failed to start this agent."
  );
  const ensureMain = useInstanceMutation(
    (instance: AgentInstancePayload) =>
      computerId
        ? ensureAgentInstanceMain(instance, computerId)
        : ensureAgentInstanceMain(instance),
    "Failed to open this agent.",
    computerId
  );
  return {
    pause: pause.mutate,
    resume: resume.mutate,
    archive: archive.mutate,
    add: add.mutate,
    ensureMain: ensureMain.mutateAsync,
  };
}

/**
 * Resume one or several instances, in order, with the pending and failed states a caller shows
 * in place. Every resume is attempted; a refusal fails with `AgentResumeError`, naming each
 * agent that stayed paused. `inline` drops the toast for a caller that shows the failure itself.
 */
export function useResumeAgentInstances({
  inline = false,
  computerId,
}: { inline?: boolean; computerId?: ComputerId } = {}) {
  const queryClient = useQueryClient();
  return useInstanceMutation(
    (instances: readonly AgentInstancePayload[]) =>
      // Each resumed agent moves in every open view at once, before the lists reread.
      resumeInOrder(instances, async instance =>
        applyAgentInstanceUpdate(
          queryClient,
          await (computerId
            ? resumeAgentInstance(instance, computerId)
            : resumeAgentInstance(instance)),
          computerId
        )
      ),
    inline ? null : "Failed to resume this agent.",
    computerId
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

export function useMarkAgentInstanceSeen(computerId?: ComputerId) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (instance: AgentInstancePayload) =>
      computerId ? markAgentInstanceSeen(instance, computerId) : markAgentInstanceSeen(instance),
    onError: error => notifyUser({ message: error.message, tone: "error" }),
    onSuccess: async (_, instance) => {
      invalidateAgentInstanceReads(queryClient, instance.id, computerId);
      await Promise.all([
        invalidateAgentInstances(queryClient, computerId),
        queryClient.invalidateQueries({
          queryKey: agentKeys.forComputer(computerId).instanceAway(instance.id),
        }),
      ]);
    },
  });
}
