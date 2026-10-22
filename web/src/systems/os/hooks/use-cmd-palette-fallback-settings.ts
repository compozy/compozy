import { useQuery } from "@tanstack/react-query";

import { settingsCmdPaletteOptions } from "@/systems/settings";
/** Fallback agent setting: the focused project's, or the user layer outside a project. */
export function useCmdPaletteFallbackSettings(input: {
  activeWorkspaceId: string | null;
  open: boolean;
  settled: boolean;
}): boolean {
  const filter =
    input.activeWorkspaceId !== null
      ? { scope: "workspace" as const, workspace_id: input.activeWorkspaceId }
      : { scope: "user" as const };
  const query = useQuery({
    ...settingsCmdPaletteOptions(filter),
    enabled: input.open && input.settled,
  });
  return query.data?.fallback_agent_enabled ?? false;
}
