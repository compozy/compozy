import { useQuery } from "@tanstack/react-query";

import type { ComputerId } from "@/lib/computer-scope";

import {
  profileDetailOptions,
  profileOperationsOptions,
  profilesListOptions,
} from "../lib/query-options";

export function useProfiles(enabled = true, computerId?: ComputerId) {
  return useQuery(profilesListOptions(enabled, computerId));
}

export function useProfile(name: string, enabled = true) {
  return useQuery(profileDetailOptions(name, enabled));
}

export function useProfileOperations(enabled = true) {
  return useQuery(profileOperationsOptions(enabled));
}
