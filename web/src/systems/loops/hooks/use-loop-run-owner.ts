import { useQueryClient } from "@tanstack/react-query";

import { useProfileReadScope, type ProfileMutationScopeParams } from "@/systems/profiles";
import { loopRunDetailOptions } from "../lib/query-options";

/** Resolve an existing run's owner within the current read scope before a mutation. */
export function useLoopRunOwner() {
  const queryClient = useQueryClient();
  const { params } = useProfileReadScope();
  return async (workspaceId: string, runId: string): Promise<ProfileMutationScopeParams> => {
    const detail = await queryClient.fetchQuery({
      ...loopRunDetailOptions(workspaceId, runId, true, params),
      staleTime: 0,
    });
    if (!detail.run.profile_name) throw new Error("The run's Profile is unavailable");
    return { profile: detail.run.profile_name };
  };
}
