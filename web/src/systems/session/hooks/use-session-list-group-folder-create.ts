import { useState } from "react";
import { toast } from "sonner";

import { useProfileReadScope, useProfiles } from "@/systems/profiles";
import { useActiveWorkspace } from "@/systems/workspace";

import {
  prefillSessionProject,
  readLastSessionProject,
  sessionProfileOptions,
  sessionProjectOptions,
  type SessionTargetOption,
} from "../lib/session-create-target";
import { useSessionFolders } from "./use-session-folders";

/** One axis the operator picks because the group does not settle it. */
export interface SessionGroupFolderAxis {
  options: SessionTargetOption[];
  value: string;
  onChange: (value: string) => void;
}

/**
 * New folder from a list group's `+`. A folder belongs to one (workspace,
 * profile) pair; the group settles what it can — its project, its profile —
 * and the dialog asks for the rest: the profile of a project group under All
 * profiles (prefilled with the creation destination), or the project of a
 * profile group under All projects (prefilled like session creation).
 */
export function useSessionListGroupFolderCreate(target: {
  /** A workspace id or `@user`; `null` when the group spans projects. */
  workspaceId: string | null;
  /** `null` when the group spans profiles. */
  profile: string | null;
}) {
  const [open, setOpen] = useState(false);
  const [picks, setPicks] = useState<{ project: string | null; profile: string | null }>({
    project: null,
    profile: null,
  });
  const profileScope = useProfileReadScope();
  const active = useActiveWorkspace();
  const profiles = useProfiles(open && target.profile === null);
  const projectOptions =
    target.workspaceId === null ? sessionProjectOptions(active.projectWorkspaces) : [];
  const profileOptions = target.profile === null ? sessionProfileOptions(profiles.data ?? []) : [];
  const workspaceId =
    target.workspaceId ??
    picks.project ??
    (projectOptions.length > 0
      ? prefillSessionProject(projectOptions, { lastUsed: readLastSessionProject() })
      : null);
  const profile = target.profile ?? picks.profile ?? profileScope.destination;
  const folders = useSessionFolders({
    workspaceId,
    profile,
    enabled: open && workspaceId !== null && profile !== "",
  });
  const projectLabel =
    projectOptions.find(option => option.value === workspaceId)?.label ??
    active.projectWorkspaces.find(workspace => workspace.id === workspaceId)?.name;

  return {
    open,
    openDialog: () => {
      setPicks({ project: null, profile: null });
      setOpen(true);
    },
    close: () => setOpen(false),
    project:
      target.workspaceId === null
        ? ({
            options: projectOptions,
            value: workspaceId ?? "",
            onChange: value => setPicks(current => ({ ...current, project: value })),
          } satisfies SessionGroupFolderAxis)
        : undefined,
    profile:
      target.profile === null
        ? ({
            options: profileOptions,
            value: profile,
            onChange: value => setPicks(current => ({ ...current, profile: value })),
          } satisfies SessionGroupFolderAxis)
        : undefined,
    /** Creates the folder in the settled pair; refuses until that pair's folders loaded. */
    create: async (name: string) => {
      const layout = folders.layout;
      if (!layout) {
        throw new Error(
          folders.loadError ? "Couldn't load folders. Try again." : "Folders are still loading."
        );
      }
      const folder = await layout.createFolder(name);
      const where = [projectLabel, target.profile === null ? profile : null].filter(Boolean);
      toast.success(
        where.length > 0
          ? `Folder “${folder.name}” created in ${where.join(" · ")}`
          : `Folder “${folder.name}” created`
      );
      return folder;
    },
  };
}
