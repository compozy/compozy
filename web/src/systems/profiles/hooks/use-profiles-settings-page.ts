import { useGatewayAccessTier } from "@/systems/gateway";
import { useWorkspaces } from "@/systems/workspace";

import { activeProfiles, archivedProfiles, PERMANENT_PROFILE } from "../lib/profile-rows";
import { openProfileDialog } from "../stores/profile-dialog-store";
import type { ProfileLifecycleFlow, ProfilePayload, ProfileSelectionPayload } from "../types";
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
  open: (flow: ProfileLifecycleFlow, profile?: string) => void;
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
    currentName: view.kind === "profile" ? view.profile : PERMANENT_PROFILE,
    manageable: tier === "local",
    isLoading: profiles.isLoading,
    errorMessage: profiles.error instanceof Error ? profiles.error.message : null,
    refetch: () => void profiles.refetch(),
    open: (flow, profile) => openProfileDialog({ flow, ...(profile ? { profile } : {}) }),
    projectName: workspaceId => names.get(workspaceId) ?? workspaceId,
  };
}
