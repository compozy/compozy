import { useState, type FormEvent } from "react";
import { toast } from "sonner";

import { useCreateWorkspace } from "./use-workspaces";
import { useDirectoryBrowser } from "@/systems/onboarding";

type SubmissionMode = "create" | null;

interface UseWorkspaceSetupContentOptions {
  onSuccessClose?: () => void;
  onWorkspaceResolved: (workspaceId: string) => void;
}

export interface WorkspaceSetupDraft {
  rootDir: string;
  name: string;
  addDirs: string[];
  defaultAgent: string;
  sandboxRef: string;
}

function emptyDraft(): WorkspaceSetupDraft {
  return { rootDir: "", name: "", addDirs: [], defaultAgent: "", sandboxRef: "" };
}

function getErrorMessage(error: unknown, fallback: string): string {
  if (error instanceof Error && error.message.trim() !== "") {
    return error.message;
  }

  return fallback;
}

/** Folder name of an absolute path, used to autofill the display name. */
function workspaceNameFromPath(path: string): string {
  const trimmed = path.trim().replace(/[\\/]+$/, "");
  const segment = trimmed.split(/[\\/]/).pop();
  return segment && segment.length > 0 ? segment : trimmed;
}

export function useWorkspaceSetupContent({
  onWorkspaceResolved,
  onSuccessClose,
}: UseWorkspaceSetupContentOptions) {
  const createWorkspace = useCreateWorkspace();
  const [browsePath, setBrowsePath] = useState("");
  const [draft, setDraft] = useState<WorkspaceSetupDraft>(emptyDraft);
  const [submissionMode, setSubmissionMode] = useState<SubmissionMode>(null);
  const [createError, setCreateError] = useState<string | null>(null);

  const browse = useDirectoryBrowser({ path: browsePath || undefined, dirsOnly: true });
  const browseData = browse.data;

  /**
   * Picking a root only updates the draft. Registration happens once, on the
   * dialog's primary action — browsing must never create a workspace.
   */
  const selectRoot = (path: string) => {
    const rootDir = path.trim();
    if (rootDir === "") return;
    setCreateError(null);
    setDraft(current => ({
      ...current,
      rootDir,
      // The display name follows the folder until the operator types their own.
      name:
        current.name === "" || current.name === workspaceNameFromPath(current.rootDir)
          ? workspaceNameFromPath(rootDir)
          : current.name,
    }));
  };

  const setName = (name: string) => setDraft(current => ({ ...current, name }));
  const setDefaultAgent = (defaultAgent: string) =>
    setDraft(current => ({ ...current, defaultAgent }));
  const setSandboxRef = (sandboxRef: string) => setDraft(current => ({ ...current, sandboxRef }));

  const addDir = (dir: string) => {
    const trimmed = dir.trim();
    if (trimmed === "") return;
    setDraft(current =>
      current.addDirs.includes(trimmed)
        ? current
        : { ...current, addDirs: [...current.addDirs, trimmed] }
    );
  };

  const removeDir = (dir: string) => {
    setDraft(current => ({ ...current, addDirs: current.addDirs.filter(item => item !== dir) }));
  };

  const resetDraft = () => {
    setDraft(emptyDraft());
    setCreateError(null);
  };

  const canSubmit = draft.rootDir.trim() !== "" && submissionMode === null;

  const handleCreateSubmit = async (event?: FormEvent<HTMLFormElement>) => {
    event?.preventDefault();
    const rootDir = draft.rootDir.trim();
    if (rootDir === "") {
      setCreateError("Pick the workspace root in the browser above.");
      return;
    }

    setSubmissionMode("create");
    setCreateError(null);
    try {
      const name = draft.name.trim();
      const defaultAgent = draft.defaultAgent.trim();
      const sandboxRef = draft.sandboxRef.trim();
      const workspace = await createWorkspace.mutateAsync({
        root_dir: rootDir,
        ...(name !== "" ? { name } : {}),
        ...(draft.addDirs.length > 0 ? { add_dirs: draft.addDirs } : {}),
        ...(defaultAgent !== "" ? { default_agent: defaultAgent } : {}),
        ...(sandboxRef !== "" ? { sandbox_ref: sandboxRef } : {}),
      });
      onWorkspaceResolved(workspace.id);
      resetDraft();
      toast.success(`Workspace ready: ${workspace.name}`);
      onSuccessClose?.();
    } catch (error) {
      // The draft survives a failed write so nothing typed is lost.
      setCreateError(getErrorMessage(error, "Failed to add workspace"));
    }
    setSubmissionMode(null);
  };

  return {
    browse: {
      currentPath: browseData?.path ?? browsePath,
      parentPath: browseData?.parent ?? null,
      homePath: browseData?.home ?? null,
      roots: browseData?.roots ?? [],
      entries: browseData?.entries ?? [],
      isBrowsing: browse.isLoading || browse.isFetching,
      browseError: browse.error
        ? getErrorMessage(browse.error, "Failed to browse directory.")
        : null,
      navigateTo: setBrowsePath,
      goToParent: () => {
        if (browseData?.parent) setBrowsePath(browseData.parent);
      },
      goHome: () => {
        if (browseData?.home) setBrowsePath(browseData.home);
      },
    },
    canSubmit,
    createError,
    draft,
    handleCreateSubmit,
    addDir,
    removeDir,
    resetDraft,
    selectRoot,
    setDefaultAgent,
    setName,
    setSandboxRef,
    submissionMode,
  };
}

export type WorkspaceSetupContent = ReturnType<typeof useWorkspaceSetupContent>;
