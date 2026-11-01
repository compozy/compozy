import { createStoreLogic } from "@xstate/store";
import { useSelector, useStore } from "@xstate/store-react";
import { toast } from "@compozy/ui";

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

interface GroupFolderDialogState {
  open: boolean;
  project: string | null;
  profile: string | null;
}

const groupFolderDialogLogic = createStoreLogic({
  context: (): GroupFolderDialogState => ({ open: false, project: null, profile: null }),
  on: {
    opened: () => ({ open: true, project: null, profile: null }),
    closed: context => ({ ...context, open: false }),
    projectPicked: (context, event: { value: string }) => ({ ...context, project: event.value }),
    profilePicked: (context, event: { value: string }) => ({ ...context, profile: event.value }),
  },
});

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
  const store = useStore(groupFolderDialogLogic);
  const picks = useSelector(store, snapshot => snapshot.context);
  const { open } = picks;
  const profileScope = useProfileReadScope();
  const active = useActiveWorkspace();
  const profiles = useProfiles(open && target.profile === null);
  const projectOptions =
    target.workspaceId === null ? sessionProjectOptions(active.projectWorkspaces) : [];
  const profileOptions = target.profile === null ? sessionProfileOptions(profiles.data ?? []) : [];
  const workspaceId = groupFolderWorkspace(target.workspaceId, picks.project, projectOptions);
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
    openDialog: () => store.trigger.opened(),
    close: () => store.trigger.closed(),
    project:
      target.workspaceId === null
        ? ({
            options: projectOptions,
            value: workspaceId ?? "",
            onChange: value => store.trigger.projectPicked({ value }),
          } satisfies SessionGroupFolderAxis)
        : undefined,
    profile:
      target.profile === null
        ? ({
            options: profileOptions,
            value: profile,
            onChange: value => store.trigger.profilePicked({ value }),
          } satisfies SessionGroupFolderAxis)
        : undefined,
    /** Creates the folder in the settled pair; refuses until that pair's folders loaded. */
    create: (name: string) =>
      createGroupFolder(folders, name, [projectLabel, target.profile === null ? profile : null]),
  };
}

async function createGroupFolder(
  folders: ReturnType<typeof useSessionFolders>,
  name: string,
  destination: (string | null | undefined)[]
) {
  const layout = folders.layout;
  if (!layout) {
    throw new Error(
      folders.loadError ? "Couldn't load folders. Try again." : "Folders are still loading."
    );
  }
  const folder = await layout.createFolder(name);
  const where = destination.filter(Boolean);
  toast.success(
    where.length > 0
      ? `Folder “${folder.name}” created in ${where.join(" · ")}`
      : `Folder “${folder.name}” created`
  );
  return folder;
}

function groupFolderWorkspace(
  target: string | null,
  picked: string | null,
  options: SessionTargetOption[]
): string | null {
  if (target !== null) return target;
  if (picked !== null) return picked;
  return options.length > 0
    ? prefillSessionProject(options, { lastUsed: readLastSessionProject() })
    : null;
}
