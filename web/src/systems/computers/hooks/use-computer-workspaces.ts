import { useQuery } from "@tanstack/react-query";

import type { ComputerId } from "@/lib/computer-scope";
import { computerWorkspacesOptions } from "@/systems/workspace";

/**
 * Queries projects/workspaces on a specific computer with key `["computer", id, "workspaces"]` (UT-081).
 */
export function useComputerWorkspaces(computerId: ComputerId, enabled = true) {
  return useQuery(computerWorkspacesOptions(computerId, enabled));
}
