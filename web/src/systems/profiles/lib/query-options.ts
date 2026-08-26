import { queryOptions } from "@tanstack/react-query";

import {
  fetchArchivePlan,
  fetchDeletePlan,
  fetchProfile,
  fetchProfileOperations,
  fetchProfileSelection,
  fetchProfileSelections,
  fetchProfiles,
  fetchRenamePlan,
} from "../adapters/profiles-api";
import type { ProfileLens } from "../types";
import { loadProfileIconCatalog } from "./profile-identity";
import { profileKeys } from "./query-keys";

export function profilesListOptions(enabled = true) {
  return queryOptions({
    queryKey: profileKeys.list(),
    queryFn: ({ signal }) => fetchProfiles(signal),
    enabled,
    staleTime: 30_000,
  });
}

export function profileDetailOptions(name: string, enabled = true) {
  const normalizedName = name.trim();
  return queryOptions({
    queryKey: profileKeys.detail(normalizedName),
    queryFn: ({ signal }) => fetchProfile(normalizedName, signal),
    enabled: enabled && normalizedName !== "",
    staleTime: 30_000,
  });
}

/**
 * The remembered choice for one lens.
 *
 * This is the projection the `profile.selection_changed` stream invalidates. It
 * is deliberately *not* the active view: refreshing it changes what a client
 * would default to on next entry, never what it is currently showing.
 */
export function profileSelectionOptions(lens: ProfileLens, enabled = true) {
  return queryOptions({
    queryKey: profileKeys.selection(lens),
    queryFn: ({ signal }) => fetchProfileSelection(lens, signal),
    enabled,
    staleTime: 30_000,
  });
}

/** The whole project → profile map, for the Settings disclosure. */
export function profileSelectionMapOptions(enabled = true) {
  return queryOptions({
    queryKey: profileKeys.selectionMap(),
    queryFn: ({ signal }) => fetchProfileSelections(signal),
    enabled,
    staleTime: 30_000,
  });
}

// Plans are read fresh every time a dialog opens: a stale revision is refused by
// the daemon (`profile_plan_stale`), so caching one would only buy a round trip
// and then fail the mutation.
export function renamePlanOptions(name: string, newName: string, enabled = true) {
  const normalizedName = name.trim();
  const normalizedNewName = newName.trim();
  return queryOptions({
    queryKey: profileKeys.renamePlan(normalizedName, normalizedNewName),
    queryFn: ({ signal }) => fetchRenamePlan(normalizedName, normalizedNewName, signal),
    enabled: enabled && normalizedName !== "" && normalizedNewName !== "",
    staleTime: 0,
    gcTime: 0,
  });
}

export function archivePlanOptions(name: string, enabled = true) {
  const normalizedName = name.trim();
  return queryOptions({
    queryKey: profileKeys.archivePlan(normalizedName),
    queryFn: ({ signal }) => fetchArchivePlan(normalizedName, signal),
    enabled: enabled && normalizedName !== "",
    staleTime: 0,
    gcTime: 0,
  });
}

export function deletePlanOptions(name: string, enabled = true) {
  const normalizedName = name.trim();
  return queryOptions({
    queryKey: profileKeys.deletePlan(normalizedName),
    queryFn: ({ signal }) => fetchDeletePlan(normalizedName, signal),
    enabled: enabled && normalizedName !== "",
    staleTime: 0,
    gcTime: 0,
  });
}

export function profileOperationsOptions(enabled = true) {
  return queryOptions({
    queryKey: profileKeys.operations(),
    queryFn: ({ signal }) => fetchProfileOperations(signal),
    enabled,
    staleTime: 10_000,
  });
}

// The catalog ships with the build, so it can never go stale within a session.
export function profileIconCatalogOptions() {
  return queryOptions({
    queryKey: profileKeys.iconCatalog(),
    queryFn: loadProfileIconCatalog,
    staleTime: Infinity,
    gcTime: Infinity,
  });
}
