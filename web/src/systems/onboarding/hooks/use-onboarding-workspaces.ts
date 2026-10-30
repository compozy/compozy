import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { useSelector } from "@xstate/store-react";

import { useDirectoryPathField, type DirectoryPathField } from "./use-directory-path-field";
import { folderName } from "../lib/directory-path";
import { directoryBrowseOptions } from "../lib/query-options";
import {
  onboardingDraftStore,
  type OnboardingWorkspaceDraft,
} from "../stores/use-onboarding-draft-store";
import {
  isOperatorHomeWorkspace,
  useCreateWorkspace,
  useDeleteWorkspace,
  useWorkspaces,
  type WorkspacePayload,
} from "@/systems/workspace";

export interface OnboardingWorkspacesApi {
  /** The folder field; adding its validated path registers the project. */
  pathField: DirectoryPathField;
  /** Registers the field's validated folder and clears the field on success. */
  addFromField: () => Promise<void>;
  workspaces: OnboardingWorkspaceDraft[];
  isResolving: boolean;
  resolveError: string | null;
  catalogError: string | null;
  addWorkspace: (path: string) => Promise<void>;
  removeWorkspace: (path: string) => Promise<void>;
  isAdded: (path: string) => boolean;
  isRemoving: boolean;
  isCatalogLoading: boolean;
  reloadCatalog: () => Promise<void>;
}

function draftFromWorkspace(
  workspace: Pick<WorkspacePayload, "id" | "name" | "root_dir">,
  fallbackPath: string
): OnboardingWorkspaceDraft {
  const path = workspace.root_dir.trim() || fallbackPath;
  return {
    path,
    name: workspace.name || folderName(path),
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
  const [resolveError, setResolveError] = useState<string | null>(null);
  const catalogSeeded = useRef(false);

  const [fieldValue, setFieldValue] = useState("");
  const homeListing = useQuery(directoryBrowseOptions({ dirsOnly: true }, true));
  const userHomeDir = homeListing.data?.home ?? undefined;
  const pathField = useDirectoryPathField({
    value: fieldValue,
    onValueChange: value => {
      setResolveError(null);
      setFieldValue(value);
    },
    isProject: path =>
      workspaces.some(item => item.path === path) ||
      (registeredWorkspaces.data ?? []).some(workspace => workspace.root_dir === path),
  });

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
    for (const workspace of registeredWorkspaces.data) {
      if (isOperatorHomeWorkspace(workspace, userHomeDir)) continue;
      const path = workspace.root_dir.trim();
      if (path.length === 0) {
        continue;
      }
      onboardingDraftStore.trigger.workspaceDraftAdded({
        workspace: draftFromWorkspace(workspace, path),
      });
    }
  }, [registeredWorkspaces.data, userHomeDir, workspaces]);

  const addWorkspace = async (path: string): Promise<boolean> => {
    const trimmed = path.trim();
    if (
      trimmed.length === 0 ||
      isOperatorHomeWorkspace({ id: "operator-home", root_dir: trimmed }, userHomeDir) ||
      workspaces.some(item => item.path === trimmed)
    ) {
      return false;
    }
    setResolveError(null);
    try {
      const workspace = await createWorkspace.mutateAsync({ root_dir: trimmed });
      onboardingDraftStore.trigger.workspaceDraftAdded({
        workspace: draftFromWorkspace(workspace, trimmed),
      });
      return true;
    } catch (error) {
      setResolveError(
        error instanceof Error ? error.message : "Failed to register that folder as a workspace."
      );
      return false;
    }
  };

  const addFromField = async () => {
    if (pathField.path === null) return;
    if (await addWorkspace(pathField.path)) setFieldValue("");
  };

  const removeWorkspace = async (path: string) => {
    if (workspaceCatalog === undefined || userHomeDir === undefined) {
      return;
    }
    const draft = workspaces.find(item => item.path === path);
    const workspaceId = registeredWorkspaceIdForDraft(draft, workspaceCatalog);
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
    pathField,
    addFromField,
    workspaces,
    isResolving: createWorkspace.isPending,
    isRemoving: deleteWorkspace.isPending,
    isCatalogLoading: registeredWorkspaces.isLoading,
    catalogError,
    resolveError,
    addWorkspace: async path => {
      await addWorkspace(path);
    },
    removeWorkspace,
    isAdded,
    reloadCatalog: async () => {
      await registeredWorkspaces.refetch();
    },
  };
}
