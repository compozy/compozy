import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { notifyUser } from "@/lib/user-feedback";
import { cmdPaletteKeys, resetCmdPalettePersonalization } from "@/systems/os";
import { useActiveWorkspace } from "@/systems/workspace";

import { updateSettingsCmdPalette } from "../adapters/settings-sections-api";
import { settingsKeys } from "../lib/query-keys";
import { settingsCmdPaletteOptions } from "../lib/query-options";
import type {
  SettingsCmdPaletteFilter,
  SettingsCmdPaletteSection,
  SettingsUpdateCmdPaletteRequest,
} from "../types";
import { useSettingsPage } from "./use-settings-page";

/** A write carries the scope it was made in, so a late answer cannot move. */
interface PaletteSettingsWrite {
  update: SettingsUpdateCmdPaletteRequest;
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
  scopeLabel: string;
  setFallbackAgentEnabled: (enabled: boolean) => void;
  setPersonalization: (enabled: boolean) => void;
  resetPersonalization: () => Promise<void>;
  isResetting: boolean;
  resetError: string | null;
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
    mutationFn: (variables: PaletteSettingsWrite) =>
      updateSettingsCmdPalette(variables.update, variables.filter),
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
  const pendingUpdate = mutation.variables?.update;
  const section =
    pendingHere && query.data !== undefined
      ? {
          ...query.data,
          ...(typeof pendingUpdate?.fallback_agent_enabled === "boolean"
            ? { fallback_agent_enabled: pendingUpdate.fallback_agent_enabled }
            : {}),
          ...(typeof pendingUpdate?.personalization === "boolean"
            ? { personalization: pendingUpdate.personalization }
            : {}),
        }
      : (query.data ?? null);
  const resetMutation = useMutation({
    mutationFn: async () => {
      if (workspace.runtimeWorkspaceId === null) {
        throw new Error("The active workspace is not ready.");
      }
      await resetCmdPalettePersonalization(workspace.runtimeWorkspaceId);
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: cmdPaletteKeys.all });
      notifyUser({ message: "Palette personalization reset.", tone: "success" });
    },
  });
  const scopeLabel =
    workspace.scope === "workspace"
      ? (workspace.activeWorkspace?.name ?? workspace.activeWorkspaceId ?? "workspace")
      : "Global";

  return {
    section,
    isLoading: !settled || query.isLoading,
    isSaving: mutation.isPending,
    error: query.error,
    saveError: mutation.error instanceof Error ? mutation.error.message : null,
    restart: page.restart,
    scopeLabel,
    setFallbackAgentEnabled: enabled => {
      if (mutation.isPending) return;
      mutation.mutate({ update: { fallback_agent_enabled: enabled }, filter });
    },
    setPersonalization: enabled => {
      if (mutation.isPending) return;
      mutation.mutate({ update: { personalization: enabled }, filter });
    },
    resetPersonalization: async () => {
      if (resetMutation.isPending) return;
      await resetMutation.mutateAsync();
    },
    isResetting: resetMutation.isPending,
    resetError: resetMutation.error instanceof Error ? resetMutation.error.message : null,
    handleRetry: () => void query.refetch(),
  };
}
