import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { cmdPaletteKeys } from "@/systems/os";
import { useActiveWorkspace } from "@/systems/workspace";

import { updateSettingsCmdPalette } from "../adapters/settings-sections-api";
import { settingsKeys } from "../lib/query-keys";
import { settingsCmdPaletteOptions } from "../lib/query-options";
import type { SettingsCmdPaletteFilter, SettingsCmdPaletteSection } from "../types";
import { useSettingsPage } from "./use-settings-page";

/** A write carries the scope it was made in, so a late answer cannot move. */
interface PalettePersonalizationWrite {
  personalization: boolean;
  filter: SettingsCmdPaletteFilter;
}

function isSameScope(left: SettingsCmdPaletteFilter, right: SettingsCmdPaletteFilter): boolean {
  return left.scope === right.scope && (left.workspace_id ?? "") === (right.workspace_id ?? "");
}

export interface SettingsPalettePageModel {
  section: SettingsCmdPaletteSection | null;
  isLoading: boolean;
  isSaving: boolean;
  error: Error | null;
  saveError: string | null;
  restart: ReturnType<typeof useSettingsPage>["restart"];
  setPersonalization: (enabled: boolean) => void;
  handleRetry: () => void;
}

/**
 * `[cmd_palette]` is a Live section, so the control is the commit — there is no
 * save bar to hold a value the CLI could change underneath it.
 *
 * Turning personalization off stops recording and neutralizes personal ranking;
 * what was already learned is kept until it is reset, so the ranking the palette
 * shows next has to be re-read rather than assumed.
 *
 * Reads and writes address one scope. `resolveActiveWorkspace` has already
 * decided which one — it downgrades workspace to global when nothing is
 * remembered, and only then is `activeWorkspaceId` null — so this reads that
 * resolution instead of applying a second policy of its own.
 */
export function useSettingsPalettePage(): SettingsPalettePageModel {
  const workspace = useActiveWorkspace();
  const queryClient = useQueryClient();
  const page = useSettingsPage({ currentSlug: "palette" });
  // Until workspace truth settles, the resolver reports the *requested* scope
  // with no id — indistinguishable from global. Asking then would write a real
  // global read into the cache for an operator who is actually in a workspace.
  // `pending` is its own signal: the catalog can be loaded while `$HOME` is
  // still unknown, and the resolver settles nothing until both have landed.
  const settled = workspace.hasHydrated && !workspace.isLoading && !workspace.pending;
  const filter: SettingsCmdPaletteFilter =
    workspace.scope === "workspace" && workspace.activeWorkspaceId !== null
      ? { scope: "workspace", workspace_id: workspace.activeWorkspaceId }
      : { scope: "global" };
  const query = useQuery({ ...settingsCmdPaletteOptions(filter), enabled: settled });
  const mutation = useMutation({
    mutationFn: (variables: PalettePersonalizationWrite) =>
      updateSettingsCmdPalette({ personalization: variables.personalization }, variables.filter),
    // The scope travels with the write, not with the render. A write issued in
    // one workspace can land after the operator moved to another, and the
    // callback would otherwise file that answer under wherever they are now.
    onSuccess: async (section, variables) => {
      queryClient.setQueryData(settingsKeys.cmdPaletteSection(variables.filter), section);
      await queryClient.invalidateQueries({ queryKey: cmdPaletteKeys.all });
    },
  });
  // Show the pending value only where it was made. After a scope switch the
  // in-flight write belongs to the scope the operator left.
  const pendingHere =
    mutation.isPending &&
    mutation.variables !== undefined &&
    isSameScope(mutation.variables.filter, filter);
  const section =
    pendingHere && query.data !== undefined
      ? { ...query.data, personalization: mutation.variables?.personalization ?? false }
      : (query.data ?? null);

  return {
    section,
    isLoading: !settled || query.isLoading,
    isSaving: mutation.isPending,
    error: query.error,
    saveError: mutation.error instanceof Error ? mutation.error.message : null,
    restart: page.restart,
    setPersonalization: enabled => {
      if (mutation.isPending) return;
      mutation.mutate({ personalization: enabled, filter });
    },
    handleRetry: () => void query.refetch(),
  };
}
