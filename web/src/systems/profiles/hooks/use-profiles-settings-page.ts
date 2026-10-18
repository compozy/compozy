import { useGatewayAccessTier } from "@/systems/gateway";

import { activeProfiles, archivedProfiles } from "../lib/profile-rows";
import { openProfileDialog } from "../stores/profile-dialog-store";
import type { ProfileDialogIntent, ProfilePayload } from "../types";
import { useActiveProfileView } from "./use-profile-selection";
import { useProfiles } from "./use-profiles";

export interface ProfilesSettingsPageModel {
  active: ProfilePayload[];
  archived: ProfilePayload[];
  currentName: string;
  /** Remote surfaces read the list; management is absent, not disabled. */
  manageable: boolean;
  isLoading: boolean;
  errorMessage: string | null;
  refetch: () => void;
  open: (intent: ProfileDialogIntent) => void;
}

/**
 * Settings → Profiles.
 *
 * Its default read is two answers: which profiles exist and how to make one.
 * The archived list sits behind disclosure because it is not a daily question.
 */
export function useProfilesSettingsPage(): ProfilesSettingsPageModel {
  const profiles = useProfiles();
  const view = useActiveProfileView();
  const tier = useGatewayAccessTier();

  const all = profiles.data ?? [];

  return {
    active: activeProfiles(all),
    archived: archivedProfiles(all),
    currentName: view.kind === "profile" ? view.profile : "",
    manageable: tier === "local",
    isLoading: profiles.isLoading,
    errorMessage: profiles.error instanceof Error ? profiles.error.message : null,
    refetch: () => void profiles.refetch(),
    open: openProfileDialog,
  };
}
