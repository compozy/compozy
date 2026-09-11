import { useProfiles } from "./use-profiles";
import { useActiveProfileView, useSwitchProfile } from "./use-profile-selection";
import {
  activeProfiles,
  archivedProfiles,
  isQuiet,
  PERMANENT_PROFILE,
  toProfileRows,
  type ProfileRow,
} from "../lib/profile-rows";
import { openProfileDialog } from "../stores/profile-dialog-store";
import type { ProfileLens } from "../types";
import { useGatewayCapabilities } from "@/systems/gateway";

export interface ProfileSwitcherModel {
  rows: ProfileRow[];
  activeName: string;
  aggregate: boolean;
  /**
   * Switches to the aggregate view. That is the same selection write as
   * switching profiles (`PUT /api/profiles/selection`), which remote tiers
   * refuse with `profile_remote_management_forbidden` — so the handler — and
   * the menu entry it drives — goes absent on those tiers (BR-1).
   */
  selectAggregate: (() => void) | undefined;
  quiet: boolean;
  archivedCount: number;
  selectProfile: (name: string) => void;
  create: () => void;
  manageable: boolean;
  isLoading: boolean;
  error: Error | null;
  retry: () => void;
}

/**
 * The switcher's view model.
 *
 * Only active profiles are offered: an archived one is not a place you can go,
 * and its work stays reachable through the aggregate instead.
 */
export function useProfileSwitcher(lens: ProfileLens): ProfileSwitcherModel {
  const profiles = useProfiles();
  const view = useActiveProfileView(lens);
  const switchProfile = useSwitchProfile(lens);

  const { profileEnablementWrites } = useGatewayCapabilities();
  const all = profiles.data ?? [];
  const activeName = view.kind === "profile" ? view.profile : PERMANENT_PROFILE;

  return {
    rows: toProfileRows(activeProfiles(all), activeName),
    activeName,
    aggregate: view.kind === "aggregate",
    quiet: isQuiet(all),
    archivedCount: archivedProfiles(all).length,
    selectProfile: name => {
      if (profileEnablementWrites) switchProfile.mutate({ kind: "profile", profile: name });
    },
    selectAggregate: profileEnablementWrites
      ? () => switchProfile.mutate({ kind: "aggregate" })
      : undefined,
    create: () => {
      if (profileEnablementWrites) openProfileDialog({ flow: "create" });
    },
    // Profile management is a local-only write surface: on remote tiers the
    // affordances go absent (manageable=false hides the create/edit entries).
    manageable: profileEnablementWrites && !profiles.isLoading && !profiles.isError,
    isLoading: profiles.isLoading,
    error: profiles.error instanceof Error ? profiles.error : null,
    retry: () => void profiles.refetch(),
  };
}
