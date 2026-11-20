import { GLOBAL_SCOPE, type AgentInstancePayload } from "@/systems/agent";

/** One scope's instances: a workspace id, or `@user` for Global. */
export function instancesInScope(instances: readonly AgentInstancePayload[], scope: string) {
  const workspaceId = scope === GLOBAL_SCOPE ? "" : scope;
  return instances.filter(instance => instance.workspace_id === workspaceId);
}

/**
 * One project group's instances. Global instances already sit in the Global row above every
 * group, so the User group, whose scope is also `@user`, carries none of them.
 */
export function projectGroupInstances(
  instances: readonly AgentInstancePayload[],
  workspaceId: string
) {
  return workspaceId === GLOBAL_SCOPE ? [] : instancesInScope(instances, workspaceId);
}
