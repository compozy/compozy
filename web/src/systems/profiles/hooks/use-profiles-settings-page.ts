import { useGatewayAccessTier } from "@/systems/gateway";
import { useWorkspaces } from "@/systems/workspace";

import { activeProfiles, archivedProfiles } from "../lib/profile-rows";
import { openProfileDialog } from "../stores/profile-dialog-store";
import type { ProfileDialogIntent, ProfilePayload, ProfileSelectionPayload } from "../types";
import { useActiveProfileView } from "./use-profile-selection";
import { useProfileLens } from "./use-profile-lens";
import { useProfileSelectionMap, useProfiles } from "./use-profiles";

export interface ProfilesSettingsPageModel {
  active: ProfilePayload[];
  archived: ProfilePayload[];
  selections: ProfileSelectionPayload[];
  currentName: string;
  /** Remote surfaces read the list; management is absent, not disabled. */
  manageable: boolean;
  isLoading: boolean;
  errorMessage: string | null;
  refetch: () => void;
  open: (intent: ProfileDialogIntent) => void;
  projectName: (workspaceId: string) => string;
}

/**
 * Settings → Profiles.
 *
 * Its default read is three answers: which profiles exist, how to make one, and
 * where each is active. The archived list and the selection map sit behind
 * disclosure because neither is a daily question.
 */
export function useProfilesSettingsPage(): ProfilesSettingsPageModel {
  const lens = useProfileLens();
  const profiles = useProfiles();
  const selections = useProfileSelectionMap();
  const workspaces = useWorkspaces();
  const view = useActiveProfileView(lens);
  const tier = useGatewayAccessTier();

  const all = profiles.data ?? [];
  const names = new Map((workspaces.data ?? []).map(workspace => [workspace.id, workspace.name]));

  return {
    active: activeProfiles(all),
    archived: archivedProfiles(all),
    selections: selections.data ?? [],
    currentName: view.kind === "profile" ? view.profile : "",
    manageable: tier === "local",
    isLoading: profiles.isLoading || selections.isLoading || workspaces.isLoading,
    errorMessage:
      [profiles.error, selections.error, workspaces.error].find(
        (error): error is Error => error instanceof Error
      )?.message ?? null,
    refetch: () => {
      void profiles.refetch();
      void selections.refetch();
      void workspaces.refetch();
    },
    open: openProfileDialog,
    projectName: workspaceId => names.get(workspaceId) ?? workspaceId,
  };
}
