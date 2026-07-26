import { useEffect, useState } from "react";

import { useDeleteWorkspace, useResolveWorkspace, useWorkspaces } from "@/systems/workspace";
import type { WorkspacePayload } from "@/systems/workspace";

import { useDirectoryBrowser } from "./use-directory-browser";
import {
  useOnboardingDraftStore,
  type OnboardingWorkspaceDraft,
} from "../stores/use-onboarding-draft-store";
import type { FSEntry } from "../types";

export interface OnboardingWorkspacesApi {
  currentPath: string;
  parent: string | null;
  home: string | null;
  entries: FSEntry[];
  isBrowsing: boolean;
  browseError: string | null;
  workspaces: OnboardingWorkspaceDraft[];
  isResolving: boolean;
  resolveError: string | null;
  catalogError: string | null;
  navigateTo: (path: string) => void;
  goToParent: () => void;
  goHome: () => void;
  addWorkspace: (path: string) => Promise<void>;
  removeWorkspace: (path: string) => Promise<void>;
  isAdded: (path: string) => boolean;
  isRemoving: boolean;
  isCatalogLoading: boolean;
  reloadCatalog: () => Promise<void>;
}

function basename(path: string): string {
  const trimmed = path.replace(/\/+$/, "");
  const segment = trimmed.split("/").pop();
  return segment && segment.length > 0 ? segment : trimmed;
}

function draftFromWorkspace(
  workspace: Pick<WorkspacePayload, "id" | "name" | "root_dir">,
  fallbackPath: string
): OnboardingWorkspaceDraft {
  const path = workspace.root_dir.trim() || fallbackPath;
  return {
    path,
    name: workspace.name || basename(path),
    workspaceId: workspace.id,
  };
}

function registeredWorkspaceIdForDraft(
  draft: OnboardingWorkspaceDraft | undefined,
  registeredWorkspaces: WorkspacePayload[]
): string {
  if (!draft) {
    return "";
  }
  const registeredWorkspace =
    registeredWorkspaces.find(workspace => workspace.id === draft.workspaceId) ??
    registeredWorkspaces.find(workspace => workspace.root_dir === draft.path);
  return registeredWorkspace?.id ?? "";
}

export function useOnboardingWorkspaces(): OnboardingWorkspacesApi {
  const workspaces = useOnboardingDraftStore(state => state.workspaces);
  const addToDraft = useOnboardingDraftStore(state => state.addWorkspace);
  const removeFromDraft = useOnboardingDraftStore(state => state.removeWorkspace);
  const resolveWorkspace = useResolveWorkspace();
  const deleteWorkspace = useDeleteWorkspace();
  const registeredWorkspaces = useWorkspaces();
  const workspaceCatalog = registeredWorkspaces.data;
  const [currentPath, setCurrentPath] = useState<string>("");
  const [resolveError, setResolveError] = useState<string | null>(null);

  const browse = useDirectoryBrowser({ path: currentPath || undefined, dirsOnly: true });
  const data = browse.data;

  useEffect(() => {
    if (workspaces.length > 0) {
      return;
    }
    for (const workspace of registeredWorkspaces.data ?? []) {
      const path = workspace.root_dir.trim();
      if (path.length === 0) {
        continue;
      }
      addToDraft(draftFromWorkspace(workspace, path));
    }
  }, [addToDraft, registeredWorkspaces.data, workspaces.length]);

  const navigateTo = (path: string) => {
    setCurrentPath(path);
  };

  const goToParent = () => {
    if (data?.parent) {
      setCurrentPath(data.parent);
    }
  };

  const goHome = () => {
    if (data?.home) {
      setCurrentPath(data.home);
    }
  };

  const addWorkspace = async (path: string) => {
    const trimmed = path.trim();
    if (trimmed.length === 0 || workspaces.some(item => item.path === trimmed)) return;
    setResolveError(null);
    try {
      const workspace = await resolveWorkspace.mutateAsync({ path: trimmed });
      addToDraft(draftFromWorkspace(workspace, trimmed));
    } catch (error) {
      setResolveError(
        error instanceof Error ? error.message : "Failed to register that folder as a workspace."
      );
    }
  };

  const removeWorkspace = async (path: string) => {
    if (workspaceCatalog === undefined) {
      return;
    }
    const draft = workspaces.find(item => item.path === path);
    const workspaceId = registeredWorkspaceIdForDraft(draft, workspaceCatalog);
    if (workspaceId.length === 0) {
      removeFromDraft(path);
      return;
    }

    setResolveError(null);
    try {
      await deleteWorkspace.mutateAsync(workspaceId);
    } catch (error) {
      setResolveError(error instanceof Error ? error.message : "Failed to remove workspace.");
      return;
    }
    removeFromDraft(path);
  };

  const isAdded = (path: string) => workspaces.some(item => item.path === path);
  const catalogError = registeredWorkspaces.error
    ? registeredWorkspaces.error instanceof Error
      ? registeredWorkspaces.error.message
      : "Failed to load registered workspaces."
    : null;

  return {
    currentPath: data?.path ?? currentPath,
    parent: data?.parent ?? null,
    home: data?.home ?? null,
    entries: data?.entries ?? [],
    isBrowsing: browse.isLoading || browse.isFetching,
    browseError: browse.error
      ? browse.error instanceof Error
        ? browse.error.message
        : "Failed to browse directory."
      : null,
    workspaces,
    isResolving: resolveWorkspace.isPending,
    isRemoving: deleteWorkspace.isPending,
    isCatalogLoading: registeredWorkspaces.isLoading,
    catalogError,
    resolveError,
    navigateTo,
    goToParent,
    goHome,
    addWorkspace,
    removeWorkspace,
    isAdded,
    reloadCatalog: async () => {
      await registeredWorkspaces.refetch();
    },
  };
}
