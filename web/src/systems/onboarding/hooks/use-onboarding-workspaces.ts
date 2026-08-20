import { useEffect, useRef, useState } from "react";
import { useSelector } from "@xstate/store-react";

import { useDirectoryBrowser } from "./use-directory-browser";
import {
  onboardingDraftStore,
  type OnboardingWorkspaceDraft,
} from "../stores/use-onboarding-draft-store";
import type { FSEntry } from "../types";
import {
  isOperatorHomeWorkspace,
  partitionProjectWorkspaces,
  useCreateWorkspace,
  useDeleteWorkspace,
  useWorkspaces,
  type WorkspacePayload,
} from "@/systems/workspace";

export interface OnboardingWorkspacesApi {
  currentPath: string;
  parent: string | null;
  home: string | null;
  roots: string[];
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
  const trimmed = path.replace(/[\\/]+$/, "");
  const segment = trimmed.split(/[\\/]/).pop();
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
  const workspaces = useSelector(onboardingDraftStore, state => state.context.workspaces);
  const createWorkspace = useCreateWorkspace();
  const deleteWorkspace = useDeleteWorkspace();
  const registeredWorkspaces = useWorkspaces();
  const workspaceCatalog = registeredWorkspaces.data;
  const [currentPath, setCurrentPath] = useState<string>("");
  const [resolveError, setResolveError] = useState<string | null>(null);
  const catalogSeeded = useRef(false);

  const browse = useDirectoryBrowser({ path: currentPath || undefined, dirsOnly: true });
  const data = browse.data;
  const userHomeDir = data?.home ?? undefined;

  useEffect(() => {
    if (
      registeredWorkspaces.data === undefined ||
      userHomeDir === undefined ||
      catalogSeeded.current
    ) {
      return;
    }
    const operatorHomeDraft = workspaces.find(workspace =>
      isOperatorHomeWorkspace(
        { id: workspace.workspaceId ?? "operator-home", root_dir: workspace.path },
        userHomeDir
      )
    );
    if (operatorHomeDraft) {
      onboardingDraftStore.trigger.workspaceDraftRemoved({ path: operatorHomeDraft.path });
      return;
    }
    catalogSeeded.current = true;
    if (workspaces.length > 0) return;
    const { projectWorkspaces } = partitionProjectWorkspaces(
      registeredWorkspaces.data,
      userHomeDir
    );
    for (const workspace of projectWorkspaces) {
      const path = workspace.root_dir.trim();
      if (path.length === 0) {
        continue;
      }
      onboardingDraftStore.trigger.workspaceDraftAdded({
        workspace: draftFromWorkspace(workspace, path),
      });
    }
  }, [registeredWorkspaces.data, userHomeDir, workspaces]);

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
    if (
      trimmed.length === 0 ||
      isOperatorHomeWorkspace({ id: "operator-home", root_dir: trimmed }, userHomeDir) ||
      workspaces.some(item => item.path === trimmed)
    ) {
      return;
    }
    setResolveError(null);
    try {
      const workspace = await createWorkspace.mutateAsync({ root_dir: trimmed });
      onboardingDraftStore.trigger.workspaceDraftAdded({
        workspace: draftFromWorkspace(workspace, trimmed),
      });
    } catch (error) {
      setResolveError(
        error instanceof Error ? error.message : "Failed to register that folder as a workspace."
      );
    }
  };

  const removeWorkspace = async (path: string) => {
    if (workspaceCatalog === undefined || userHomeDir === undefined) {
      return;
    }
    const { projectWorkspaces } = partitionProjectWorkspaces(workspaceCatalog, userHomeDir);
    const draft = workspaces.find(item => item.path === path);
    const workspaceId = registeredWorkspaceIdForDraft(draft, projectWorkspaces);
    if (workspaceId.length === 0) {
      onboardingDraftStore.trigger.workspaceDraftRemoved({ path });
      return;
    }

    setResolveError(null);
    try {
      await deleteWorkspace.mutateAsync(workspaceId);
    } catch (error) {
      setResolveError(error instanceof Error ? error.message : "Failed to remove workspace.");
      return;
    }
    onboardingDraftStore.trigger.workspaceDraftRemoved({ path });
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
    roots: data?.roots ?? [],
    entries: data?.entries ?? [],
    isBrowsing: browse.isLoading || browse.isFetching,
    browseError: browse.error
      ? browse.error instanceof Error
        ? browse.error.message
        : "Failed to browse directory."
      : null,
    workspaces,
    isResolving: createWorkspace.isPending,
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
