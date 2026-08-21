import { useArchivePlan, useDeletePlan, useRenamePlan } from "./use-profile-plans";
import { useProfileLifecycle, type ProfileLifecycleState } from "./use-profile-lifecycle";
import {
  useArchiveProfile,
  useCreateProfile,
  useDeleteProfile,
  useRenameProfile,
  useUnarchiveProfile,
  useUpdateProfileIdentity,
} from "./use-profile-mutations";
import type {
  ArchiveProfilePlan,
  DeleteProfilePlan,
  ProfilePayload,
  RenameProfilePlan,
} from "../types";

type Mutation<TVariables, TData> = {
  mutate: (variables: TVariables, options?: { onSuccess?: (data: TData) => void }) => void;
  isPending: boolean;
  error: unknown;
};

export interface ProfileLifecycleDialogsModel {
  lifecycle: ProfileLifecycleState;
  target: string;
  profile: ProfilePayload | undefined;
  workItems: number;
  create: Mutation<Parameters<ReturnType<typeof useCreateProfile>["mutate"]>[0], ProfilePayload>;
  rename: ReturnType<typeof useRenameProfile>;
  update: ReturnType<typeof useUpdateProfileIdentity>;
  archive: ReturnType<typeof useArchiveProfile>;
  unarchive: ReturnType<typeof useUnarchiveProfile>;
  remove: ReturnType<typeof useDeleteProfile>;
  renamePlan: {
    data: RenameProfilePlan | undefined;
    isFetching: boolean;
    error: unknown;
    refetch: () => void;
  };
  archivePlan: { data: ArchiveProfilePlan | undefined; error: unknown; refetch: () => void };
  deletePlan: { data: DeleteProfilePlan | undefined; error: unknown; refetch: () => void };
}

/**
 * Everything the dialog host needs, assembled in one place.
 *
 * The host stays a switch over the open intent; the plan reads and the five
 * mutations live here so no single component carries the whole lifecycle.
 */
export function useProfileLifecycleDialogs(
  profiles: readonly ProfilePayload[]
): ProfileLifecycleDialogsModel {
  const lifecycle = useProfileLifecycle();
  const flow = lifecycle.intent?.flow;
  const target = lifecycle.intent?.profile ?? "";
  const profile = profiles.find(candidate => candidate.name === target);
  const workItems = profile?.work_items ?? 0;

  const renamePlan = useRenamePlan(target, lifecycle.renameName, flow === "rename");
  const archivePlan = useArchivePlan(target, flow === "archive");
  const deletePlan = useDeletePlan(target, flow === "delete" && workItems === 0);

  return {
    lifecycle,
    target,
    profile,
    workItems,
    create: useCreateProfile(),
    rename: useRenameProfile(),
    update: useUpdateProfileIdentity(),
    archive: useArchiveProfile(),
    unarchive: useUnarchiveProfile(),
    remove: useDeleteProfile(),
    renamePlan: {
      data: renamePlan.data,
      isFetching: renamePlan.isFetching,
      error: renamePlan.error,
      refetch: () => void renamePlan.refetch(),
    },
    archivePlan: {
      data: archivePlan.data,
      error: archivePlan.error,
      refetch: () => void archivePlan.refetch(),
    },
    deletePlan: {
      data: deletePlan.data,
      error: deletePlan.error,
      refetch: () => void deletePlan.refetch(),
    },
  };
}
