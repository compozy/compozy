import { useQuery } from "@tanstack/react-query";

import {
  profileDetailOptions,
  profileOperationsOptions,
  profilesListOptions,
} from "../lib/query-options";

export function useProfiles(enabled = true) {
  return useQuery(profilesListOptions(enabled));
}

export function useProfile(name: string, enabled = true) {
  return useQuery(profileDetailOptions(name, enabled));
}

export function useProfileOperations(enabled = true) {
  return useQuery(profileOperationsOptions(enabled));
}
