import { useState, type FormEvent } from "react";
import { toast } from "sonner";

import { useCreateWorkspace, useWorkspaces } from "./use-workspaces";
import { useDirectoryPathField } from "@/systems/onboarding";

type SubmissionMode = "create" | null;

interface UseWorkspaceSetupContentOptions {
  enabled?: boolean;
  onSuccessClose?: () => void;
  onWorkspaceResolved: (workspaceId: string) => void;
}

export interface WorkspaceSetupDraft {
  rootDir: string;
  name: string;
  addDirs: string[];
  defaultAgent: string;
}

function emptyDraft(): WorkspaceSetupDraft {
  return { rootDir: "", name: "", addDirs: [], defaultAgent: "" };
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
  enabled = true,
  onWorkspaceResolved,
  onSuccessClose,
}: UseWorkspaceSetupContentOptions) {
  const createWorkspace = useCreateWorkspace();
  const [draft, setDraft] = useState<WorkspaceSetupDraft>(emptyDraft);
  const [submissionMode, setSubmissionMode] = useState<SubmissionMode>(null);
  const [createError, setCreateError] = useState<string | null>(null);

  const workspaces = useWorkspaces({ enabled });

  /**
   * Typing or picking a root only updates the draft. Registration happens once,
   * on the dialog's primary action — choosing a folder must never create a workspace.
   */
  const selectRoot = (rootDir: string) => {
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

  const rootField = useDirectoryPathField({
    value: draft.rootDir,
    onValueChange: selectRoot,
    isProject: path => (workspaces.data ?? []).some(workspace => workspace.root_dir === path),
    enabled,
  });

  const canSubmit = rootField.status === "valid" && submissionMode === null;

  const handleCreateSubmit = async (event?: FormEvent<HTMLFormElement>) => {
    event?.preventDefault();
    const rootDir = rootField.path;
    if (rootDir === null) {
      setCreateError(rootField.status === "empty" ? "Choose the project folder." : null);
      return;
    }

    setSubmissionMode("create");
    setCreateError(null);
    try {
      const name = draft.name.trim();
      const defaultAgent = draft.defaultAgent.trim();
      const workspace = await createWorkspace.mutateAsync({
        root_dir: rootDir,
        ...(name !== "" ? { name } : {}),
        ...(draft.addDirs.length > 0 ? { add_dirs: draft.addDirs } : {}),
        ...(defaultAgent !== "" ? { default_agent: defaultAgent } : {}),
      });
      onWorkspaceResolved(workspace.id);
      resetDraft();
      toast.success(`Project ready: ${workspace.name}`);
      onSuccessClose?.();
    } catch (error) {
      // The draft survives a failed write so nothing typed is lost.
      setCreateError(getErrorMessage(error, "Couldn't add the project"));
    }
    setSubmissionMode(null);
  };

  return {
    rootField,
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
    submissionMode,
  };
}

export type WorkspaceSetupContent = ReturnType<typeof useWorkspaceSetupContent>;
