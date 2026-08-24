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
import { useProfileReadScope } from "@/systems/profiles";

/** A write carries the scope it was made in, so a late answer cannot move. */
interface PaletteSettingsWrite {
  update: SettingsUpdateCmdPaletteRequest;
  filter: SettingsCmdPaletteFilter;
}

interface PalettePersonalizationReset {
  profile: string;
  workspaceId: string;
}

function isSameScope(left: SettingsCmdPaletteFilter, right: SettingsCmdPaletteFilter): boolean {
  return (
    left.scope === right.scope &&
    (left.workspace_id ?? "") === (right.workspace_id ?? "") &&
    (left.profile ?? "") === (right.profile ?? "")
  );
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
  canResetPersonalization: boolean;
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
 * Reads and writes address one scope. A named profile wins because palette
 * preferences follow that persona; the permanent default profile falls back
 * to the active workspace, then to the user-wide config.
 */
export function useSettingsPalettePage(): SettingsPalettePageModel {
  // Personalization is per profile; a reset acts as one rather than across all.
  const { destination } = useProfileReadScope();
  const workspace = useActiveWorkspace();
  const queryClient = useQueryClient();
  const page = useSettingsPage({ currentSlug: "palette" });
  // Until workspace truth settles, the resolver reports the *requested* scope
  // with no id — indistinguishable from user scope. Asking then would write a
  // user read into the cache for an operator who is actually in a workspace.
  // `pending` is its own signal: the catalog can be loaded while `$HOME` is
  // still unknown, and the resolver settles nothing until both have landed.
  const settled = workspace.hasHydrated && !workspace.isLoading && !workspace.pending;
  const filter: SettingsCmdPaletteFilter =
    destination !== "default"
      ? { scope: "profile", profile: destination }
      : workspace.scope === "workspace" && workspace.activeWorkspaceId !== null
        ? { scope: "workspace", workspace_id: workspace.activeWorkspaceId }
        : { scope: "user" };
  const needsWorkspace = destination === "default";
  const query = useQuery({
    ...settingsCmdPaletteOptions(filter),
    enabled: !needsWorkspace || settled,
  });
  const pageError =
    needsWorkspace && workspace.error instanceof Error ? workspace.error : (query.error ?? null);
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
    mutationFn: ({ workspaceId, profile }: PalettePersonalizationReset) =>
      resetCmdPalettePersonalization(workspaceId, profile),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: cmdPaletteKeys.all });
      notifyUser({ message: "Palette personalization reset.", tone: "success" });
    },
  });
  const resetVariables = resetMutation.variables;
  const resetHere =
    resetVariables !== undefined &&
    resetVariables.profile === destination &&
    resetVariables.workspaceId === workspace.runtimeWorkspaceId;
  const canResetPersonalization = workspace.runtimeWorkspaceId !== null;
  const scopeLabel =
    filter.scope === "profile"
      ? `Profile ${destination}`
      : filter.scope === "workspace"
        ? (workspace.activeWorkspace?.name ?? workspace.activeWorkspaceId ?? "workspace")
        : "User";

  return {
    section,
    isLoading: pageError === null && ((needsWorkspace && !settled) || query.isLoading),
    isSaving: mutation.isPending,
    error: pageError,
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
      const workspaceId = workspace.runtimeWorkspaceId;
      if (workspaceId === null || (resetMutation.isPending && resetHere)) return;
      await resetMutation.mutateAsync({ workspaceId, profile: destination });
    },
    canResetPersonalization,
    isResetting: resetMutation.isPending && resetHere,
    resetError:
      resetHere && resetMutation.error instanceof Error ? resetMutation.error.message : null,
    handleRetry: () => {
      void workspace.refetch();
      void query.refetch();
    },
  };
}
