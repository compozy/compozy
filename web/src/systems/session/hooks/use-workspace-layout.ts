import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import type { ProfileScopeParams } from "@/systems/profiles";

import {
  createWorkspaceFolder,
  deleteWorkspaceFolder,
  moveWorkspace,
  renameWorkspaceFolder,
  reorderWorkspaceFolders,
  reorderWorkspaces,
} from "../adapters/workspace-layout-api";
import { sessionKeys } from "../lib/query-keys";
import { workspaceLayoutOptions } from "../lib/session-catalog-options";
import { shiftId } from "../lib/session-folders";
import type { SessionListGroupSpec } from "../lib/session-list-groups";
import type { SessionListGroupBy } from "../lib/session-list-preferences";
import {
  buildProjectGroupTree,
  projectGroupsForView,
  projectGroupTreeGroups,
  withShownOrder,
  type ProjectGroupTree,
} from "../lib/workspace-folders";
import {
  applyWorkspaceFolderOrder,
  applyWorkspaceFolderRemoval,
  applyWorkspaceMove,
  applyWorkspaceOrder,
} from "../lib/workspace-layout-edits";
import type { WorkspaceFolderPayload, WorkspaceLayoutResponse } from "../types";

/** One profile's project folders and project order, plus the operator's layout verbs. */
export interface WorkspaceLayout {
  folders: readonly WorkspaceFolderPayload[];
  placements: WorkspaceLayoutResponse["placements"];
  createFolder: (name: string) => Promise<WorkspaceFolderPayload>;
  renameFolder: (folderId: string, name: string) => Promise<void>;
  deleteFolder: (folderId: string) => Promise<void>;
  /** Keyboard alternative to dragging a folder header. */
  moveFolder: (folderId: string, delta: -1 | 1) => void;
  reorderFolders: (folderIds: readonly string[]) => void;
  /** Moves a project (id or `@user`) to the end of a folder, or of the root when `folderId` is null. */
  moveWorkspace: (workspaceId: string, folderId: string | null) => void;
  /** Pins a container's visible project order; `folderId` null is the root. */
  reorderWorkspaces: (folderId: string | null, workspaceIds: readonly string[]) => void;
}

/** The layout once loaded, or the failed first read and its retry. */
export interface WorkspaceLayoutState {
  layout?: WorkspaceLayout;
  loadError?: { retry: () => void };
}

interface LayoutWrite {
  failure: string;
  request: () => Promise<unknown>;
  optimistic?: (layout: WorkspaceLayoutResponse) => WorkspaceLayoutResponse;
}

/**
 * Project folders of one acting profile (issue #1044). Writes patch the served
 * layout optimistically, restore it when the daemon refuses, and reread it
 * either way; one mutation scope per profile keeps rapid drags in order.
 */
export function useWorkspaceLayout(options: {
  profile: string;
  enabled: boolean;
}): WorkspaceLayoutState {
  const queryClient = useQueryClient();
  const profile = options.profile.trim();
  const query = useQuery({ ...workspaceLayoutOptions(profile), enabled: options.enabled });
  const key = sessionKeys.workspaceLayout(profile);
  const requestProfile = profile || undefined;

  const mutation = useMutation({
    scope: { id: `workspace-layout:${profile}` },
    mutationFn: (run: LayoutWrite) => run.request(),
    onMutate: async (run: LayoutWrite) => {
      await queryClient.cancelQueries({ queryKey: key });
      const previous = queryClient.getQueryData<WorkspaceLayoutResponse>(key);
      if (previous && run.optimistic) queryClient.setQueryData(key, run.optimistic(previous));
      return { previous };
    },
    onError: (error, run, context) => {
      queryClient.setQueryData(key, context?.previous);
      toast.error(run.failure, { description: error instanceof Error ? error.message : undefined });
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: key }),
  });
  const write = (run: LayoutWrite) => mutation.mutateAsync(run).then(() => undefined);
  const fireAndForget = (run: LayoutWrite) => void write(run).catch(() => undefined);

  if (!options.enabled) return {};
  if (query.data === undefined) {
    return query.isError ? { loadError: { retry: () => void query.refetch() } } : {};
  }
  const { folders, placements } = query.data;
  const folderOrderWrite = (folderIds: readonly string[]): LayoutWrite => ({
    failure: "Couldn't reorder the folders.",
    request: () => reorderWorkspaceFolders(requestProfile, folderIds),
    optimistic: current => applyWorkspaceFolderOrder(current, folderIds),
  });

  return {
    layout: {
      folders,
      placements,
      createFolder: async name => {
        const folder = await createWorkspaceFolder(requestProfile, name);
        queryClient.setQueryData<WorkspaceLayoutResponse>(key, current =>
          current ? { ...current, folders: [folder, ...current.folders] } : current
        );
        await queryClient.invalidateQueries({ queryKey: key });
        return folder;
      },
      renameFolder: (folderId, name) =>
        write({
          failure: "Couldn't rename the folder.",
          request: () => renameWorkspaceFolder(requestProfile, folderId, name),
          optimistic: current => ({
            ...current,
            folders: current.folders.map(folder =>
              folder.id === folderId ? { ...folder, name } : folder
            ),
          }),
        }),
      deleteFolder: folderId =>
        write({
          failure: "Couldn't delete the folder.",
          request: () => deleteWorkspaceFolder(requestProfile, folderId),
          optimistic: current => applyWorkspaceFolderRemoval(current, folderId),
        }),
      moveFolder: (folderId, delta) => {
        const ordered = [...folders].sort((left, right) => left.position - right.position);
        const next = shiftId(
          ordered.map(folder => folder.id),
          folderId,
          delta
        );
        if (next) fireAndForget(folderOrderWrite(next));
      },
      reorderFolders: folderIds => fireAndForget(folderOrderWrite(folderIds)),
      moveWorkspace: (workspaceId, folderId) =>
        fireAndForget({
          failure: "Couldn't move the project.",
          request: () => moveWorkspace(requestProfile, workspaceId, folderId),
          optimistic: current => applyWorkspaceMove(current, workspaceId, folderId),
        }),
      reorderWorkspaces: (folderId, workspaceIds) =>
        fireAndForget({
          failure: "Couldn't reorder the projects.",
          request: () => reorderWorkspaces(requestProfile, folderId, workspaceIds),
          optimistic: current => applyWorkspaceOrder(current, folderId, workspaceIds),
        }),
    },
  };
}

/**
 * Project groups arranged by the profile's project folders where they apply:
 * the groups in on-screen order, the folders model, and a failed layout read.
 * Until the layout loads, groups keep the order they were served in. Projects
 * with no sessions in view, and folders holding only those, show only while the
 * operator shows empty projects (#1085).
 */
export function useProjectFolderGroups(options: {
  groups: SessionListGroupSpec[];
  groupBy: SessionListGroupBy;
  breadth: { allProjects: boolean; allProfiles: boolean };
  lens: ProfileScopeParams;
  workspaces: ReadonlyArray<{ id: string; name: string }>;
  showEmptyProjects: boolean;
  enabled: boolean;
}): {
  groups: SessionListGroupSpec[];
  projectFolders?: { layout: WorkspaceLayout; tree: ProjectGroupTree };
  loadError?: { retry: () => void };
} {
  const { groupBy, breadth, lens, showEmptyProjects: showEmpty } = options;
  const projectGrouping = groupBy === "project" && breadth.allProjects;
  // Project folders belong to one profile and arrange the project groups of All projects.
  const applies = projectGrouping && "profile" in lens;
  const { layout, loadError } = useWorkspaceLayout({
    profile: "profile" in lens ? lens.profile : "",
    enabled: options.enabled && applies,
  });
  const visible = (placements?: WorkspaceLayout["placements"]) =>
    projectGrouping
      ? projectGroupsForView(options.groups, {
          showEmpty,
          workspaces: options.workspaces,
          lens,
          placements,
        })
      : options.groups;
  if (!layout) return { groups: visible(), loadError };
  const tree = buildProjectGroupTree(visible(layout.placements), layout, {
    hideEmptyFolders: !showEmpty,
  });
  return {
    groups: projectGroupTreeGroups(tree),
    projectFolders: {
      layout: showEmpty
        ? layout
        : withShownOrder(layout, tree, new Set(options.workspaces.map(row => row.id))),
      tree,
    },
    loadError,
  };
}
