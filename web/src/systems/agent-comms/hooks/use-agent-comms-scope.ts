/**
 * The one place a calls surface learns which workspace and profile it is reading.
 *
 * Every hook in this system takes its scope from here rather than reaching for
 * the workspace store and the profile lens separately. That is what keeps the
 * cache key and the request parameters in lockstep: they are derived from one
 * value, so a surface cannot key on one profile while requesting another.
 *
 * The workspace id is empty until it resolves; query options treat that as
 * "not ready" rather than as a real scope, so nothing fetches unscoped.
 */
import { useProfileReadScope } from "@/systems/profiles";
import { useActiveWorkspace } from "@/systems/workspace";

import type { AgentCommsScope } from "../lib/agent-comms-scope";

export function useAgentCommsScope(): AgentCommsScope {
  const { runtimeWorkspaceId } = useActiveWorkspace();
  const profile = useProfileReadScope();
  return {
    workspaceId: runtimeWorkspaceId ?? "",
    profileKey: profile.key,
    params: profile.params,
    actingProfile: profile.destination,
  };
}
