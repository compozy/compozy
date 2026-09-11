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
  quiet: boolean;
  archivedCount: number;
  selectProfile: (name: string) => void;
  selectAggregate: () => void;
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
    selectAggregate: () => switchProfile.mutate({ kind: "aggregate" }),
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
