import { useQuery } from "@tanstack/react-query";

import {
  profileDetailOptions,
  profileOperationsOptions,
  profileSelectionMapOptions,
  profilesListOptions,
} from "../lib/query-options";

export function useProfiles(enabled = true) {
  return useQuery(profilesListOptions(enabled));
}

export function useProfile(name: string, enabled = true) {
  return useQuery(profileDetailOptions(name, enabled));
}

/** The project → profile map behind the Settings disclosure. */
export function useProfileSelectionMap(enabled = true) {
  return useQuery(profileSelectionMapOptions(enabled));
}

export function useProfileOperations(enabled = true) {
  return useQuery(profileOperationsOptions(enabled));
}
