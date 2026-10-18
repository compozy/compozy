import { useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";

import { notifyUser } from "@/lib/user-feedback";

import {
  archiveProfile,
  createProfile,
  deleteProfile,
  renameProfile,
  unarchiveProfile,
  updateProfileIdentity,
} from "../adapters/profiles-api";
import { profileKeys } from "../lib/query-keys";
import { setProfileView, sweepProfileView } from "../stores/profile-view-store";
import type { CreateProfileParams, UpdateProfileParams } from "../types";

/**
 * Every lifecycle change moves the list, the row, and the remembered choice —
 * archive drops selections back to `default`, delete sweeps them — so all three
 * projections reconcile rather than each mutation guessing its own new state.
 */
function reconcile(queryClient: QueryClient, name?: string): Promise<unknown> {
  const invalidations = [
    queryClient.invalidateQueries({ queryKey: profileKeys.lists() }),
    queryClient.invalidateQueries({ queryKey: profileKeys.selections() }),
  ];
  if (name !== undefined && name !== "") {
    invalidations.push(queryClient.invalidateQueries({ queryKey: profileKeys.detail(name) }));
  }
  return Promise.all(invalidations);
}

// Lifecycle dialogs own failure feedback through each mutation's error state.
export function useCreateProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (params: CreateProfileParams) => createProfile(params),
    onSuccess: (profile, input) => {
      notifyUser({ message: `Created ${profile.name}.`, tone: "success" });
      const activation = input.activate;
      if (activation?.scope === "global") {
        setProfileView({ scope: "global" }, { kind: "profile", profile: profile.name });
      } else if (activation?.scope === "workspace" && activation.workspace_id) {
        setProfileView(
          { scope: "workspace", workspaceId: activation.workspace_id },
          { kind: "profile", profile: profile.name }
        );
      }
      return reconcile(queryClient, profile.name);
    },
  });
}

export function useUpdateProfileIdentity() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ name, patch }: { name: string; patch: UpdateProfileParams }) =>
      updateProfileIdentity(name, patch),
    onSuccess: profile => reconcile(queryClient, profile.name),
  });
}

export function useRenameProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: {
      name: string;
      newName: string;
      planRevision: string;
      repos?: string[];
    }) =>
      renameProfile(input.name, {
        new_name: input.newName,
        plan_revision: input.planRevision,
        ...(input.repos ? { repos: input.repos } : {}),
      }),
    onSuccess: (_result, input) => {
      notifyUser({ message: `Renamed ${input.name} to ${input.newName}.`, tone: "success" });
      // The old name is gone; any client still looking through it must fall back.
      sweepProfileView(input.name);
      return reconcile(queryClient, input.newName);
    },
  });
}

export function useArchiveProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: { name: string; planRevision: string }) =>
      archiveProfile(input.name, input.planRevision),
    onSuccess: (_result, input) => {
      sweepProfileView(input.name);
      return reconcile(queryClient, input.name);
    },
  });
}

export function useUnarchiveProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => unarchiveProfile(name),
    onSuccess: (_result, name) => reconcile(queryClient, name),
  });
}

export function useDeleteProfile() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (input: { name: string; planRevision: string }) =>
      deleteProfile(input.name, input.planRevision),
    onSuccess: (_result, input) => {
      notifyUser({ message: `Deleted ${input.name}. The name is free.`, tone: "success" });
      sweepProfileView(input.name);
      queryClient.removeQueries({ queryKey: profileKeys.detail(input.name) });
      return reconcile(queryClient);
    },
  });
}
