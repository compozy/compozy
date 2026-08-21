import { useQuery } from "@tanstack/react-query";

import { settingsCmdPaletteOptions } from "@/systems/settings";
import type { WorkspaceScopeMode } from "@/systems/workspace";

export function useCmdPaletteFallbackSettings(input: {
  activeWorkspaceId: string | null;
  open: boolean;
  scope: WorkspaceScopeMode;
  settled: boolean;
}): boolean {
  const filter =
    input.scope === "workspace" && input.activeWorkspaceId !== null
      ? { scope: "workspace" as const, workspace_id: input.activeWorkspaceId }
      : { scope: "global" as const };
  const query = useQuery({
    ...settingsCmdPaletteOptions(filter),
    enabled: input.open && input.settled,
  });
  return query.data?.fallback_agent_enabled ?? false;
}
