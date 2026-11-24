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
export function useProfileSwitcher(): ProfileSwitcherModel {
  const profiles = useProfiles();
  const view = useActiveProfileView();
  const switchProfile = useSwitchProfile();

  const all = profiles.data ?? [];
  const activeName = view.kind === "profile" ? view.profile : PERMANENT_PROFILE;

  return {
    rows: toProfileRows(activeProfiles(all), activeName),
    activeName,
    aggregate: view.kind === "aggregate",
    quiet: isQuiet(all),
    archivedCount: archivedProfiles(all).length,
    selectProfile: name => {
      switchProfile.mutate({ kind: "profile", profile: name });
    },
    selectAggregate: () => switchProfile.mutate({ kind: "aggregate" }),
    create: () => {
      openProfileDialog({ flow: "create" });
    },
    manageable: !profiles.isLoading && !profiles.isError,
    isLoading: profiles.isLoading,
    error: profiles.error instanceof Error ? profiles.error : null,
    retry: () => void profiles.refetch(),
  };
}
